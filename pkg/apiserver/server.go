package apiserver

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/startupstate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// bootstrapRetryInterval is how long the bootstrap retry loop waits
	// between attempts when bootstrapCritical returns a transient error.
	bootstrapRetryInterval = 10 * time.Second

	// bootstrapCriticalDeadline bounds how long Start() will wait for
	// bootstrapCritical before marking the pod Ready in the default
	// resilient mode. Critical continues retrying in the background after
	// this deadline; the deadline only governs when /healthz flips to 200.
	bootstrapCriticalDeadline = 30 * time.Second
)

type APIServerParams struct {
	Context               context.Context
	LicenseBytes          []byte
	IntegrationLicenseID  string
	LicenseFields         sdklicensetypes.LicenseFields
	AppName               string
	ChannelID             string
	ChannelName           string
	ChannelSequence       int64
	ReleaseSequence       int64
	ReleaseCreatedAt      string
	ReleaseNotes          string
	VersionLabel          string
	ReplicatedAppEndpoint string
	ReleaseImages         []string
	StatusInformers       []appstatetypes.StatusInformerString
	ReplicatedID          string
	AppID                 string
	Namespace             string
	TlsCertSecretName     string
	ReportAllImages       bool
	ReadOnlyMode          bool

	// RequireUpstreamOnStartup, when true, restores the pre-Phase-1
	// behavior in which the pod does not become Ready until the full
	// bootstrap (critical + background) succeeds, including a successful
	// upstream license refresh and update fetch. Default false.
	RequireUpstreamOnStartup bool
}

func Start(params APIServerParams) {
	log.Println("Replicated version:", buildversion.Version())

	state := startupstate.New()
	handlers.SetStartupState(state)

	srv, err := buildServer(params)
	if err != nil {
		logger.Error(err)
		return
	}

	go func() {
		if err := runBootstrap(params, state); err != nil {
			log.Fatalf("%v", err)
		}
	}()

	if params.TlsCertSecretName != "" {
		log.Printf("Starting Replicated API on port %d with TLS...\n", 3000)
		log.Fatal(srv.ListenAndServeTLS("", ""))
	} else {
		log.Printf("Starting Replicated API on port %d...\n", 3000)
		log.Fatal(srv.ListenAndServe())
	}
}

// buildServer constructs the HTTP server and registers all routes. It also
// loads TLS material (from a kubernetes Secret) when configured.
//
// This is split out so the listener can be started in the foreground while
// bootstrap runs concurrently, instead of bootstrap blocking listener
// startup as it did pre-Phase-1.
func buildServer(params APIServerParams) (*http.Server, error) {
	r := mux.NewRouter()
	r.Use(handlers.CorsMiddleware)

	// TODO: make all routes authenticated
	authRouter := r.NewRoute().Subrouter()
	authRouter.Use(handlers.RequireValidLicenseIDMiddleware)

	cacheHandler := handlers.CacheMiddleware(handlers.NewCache(), handlers.CacheMiddlewareDefaultTTL)
	cachedRouter := r.NewRoute().Subrouter()
	cachedRouter.Use(cacheHandler)

	r.HandleFunc("/healthz", handlers.Healthz)

	// license
	r.HandleFunc("/api/v1/license/info", handlers.GetLicenseInfo).Methods("GET")
	r.HandleFunc("/api/v1/license/fields", handlers.GetLicenseFields).Methods("GET")
	r.HandleFunc("/api/v1/license/fields/{fieldName}", handlers.GetLicenseField).Methods("GET")

	// app
	r.HandleFunc("/api/v1/app/info", handlers.GetCurrentAppInfo).Methods("GET")
	r.HandleFunc("/api/v1/app/status", handlers.GetCurrentAppStatus).Methods("GET")
	r.HandleFunc("/api/v1/app/updates", handlers.GetAppUpdates).Methods("GET")
	r.HandleFunc("/api/v1/app/history", handlers.GetAppHistory).Methods("GET")
	cachedRouter.HandleFunc("/api/v1/app/custom-metrics", handlers.SendCustomAppMetrics).Methods("POST", "PATCH")
	cachedRouter.HandleFunc("/api/v1/app/custom-metrics/{key}", handlers.DeleteCustomAppMetricsKey).Methods("DELETE")
	cachedRouter.HandleFunc("/api/v1/app/instance-tags", handlers.SendAppInstanceTags).Methods("POST")

	// support bundle
	r.HandleFunc("/api/v1/supportbundle", handlers.UploadSupportBundle).Methods("POST")
	r.HandleFunc("/api/v1/supportbundle/metadata", handlers.PostSupportBundleMetadata).Methods("POST")
	r.HandleFunc("/api/v1/supportbundle/metadata", handlers.PatchSupportBundleMetadata).Methods("PATCH")

	// integration
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.PostIntegrationMockData)).Methods("POST")
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.GetIntegrationMockData)).Methods("GET")
	r.HandleFunc("/api/v1/integration/status", handlers.EnforceMockAccess(handlers.GetIntegrationStatus)).Methods("GET")

	srv := &http.Server{
		Handler: r,
		Addr:    ":3000",
	}

	if params.TlsCertSecretName != "" {
		clientset, err := k8sutil.GetClientset()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get clientset")
		}
		tlsConfig, err := loadTLSConfig(clientset, params.Namespace, params.TlsCertSecretName)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load TLS config")
		}
		srv.TLSConfig = tlsConfig
	}

	return srv, nil
}

// bootstrapDeps wires the orchestrator to its collaborators. The default
// production deps are constructed by defaultBootstrapDeps; tests substitute
// fakes by calling runBootstrapWithDeps directly.
type bootstrapDeps struct {
	critical      func(APIServerParams) error
	background    func(APIServerParams) error
	deadline      time.Duration
	retryInterval time.Duration
}

func defaultBootstrapDeps() bootstrapDeps {
	return bootstrapDeps{
		critical:      bootstrapCritical,
		background:    bootstrapBackground,
		deadline:      bootstrapCriticalDeadline,
		retryInterval: bootstrapRetryInterval,
	}
}

// runBootstrap drives the bootstrap state machine. The behavior depends on
// params.RequireUpstreamOnStartup:
//
//   - false (default): bootstrapCritical is retried with backoff; the pod
//     is marked Ready as soon as critical succeeds OR the
//     bootstrapCriticalDeadline elapses, whichever comes first. After the
//     deadline, critical continues to retry; on eventual success the
//     bootstrapBackground phase runs. bootstrapBackground errors are logged
//     but never block readiness.
//
//   - true: the pre-Phase-1 contract is preserved. bootstrapCritical and
//     bootstrapBackground are run in sequence with retry-with-backoff and
//     the pod is not marked Ready until both succeed.
//
// A non-nil return value indicates the process should exit fatally; the
// readiness state has already been transitioned to Failed when this happens
// so a final scrape of /healthz reflects what occurred. Callers (typically
// Start) map a non-nil error to log.Fatalf.
//
// runBootstrap blocks until the bootstrap pipeline has fully resolved (or
// has resolved enough to know it must exit). It is intended to be invoked
// from a goroutine while the HTTP listener runs in the foreground.
func runBootstrap(params APIServerParams, state *startupstate.Tracker) error {
	return runBootstrapWithDeps(params, state, defaultBootstrapDeps())
}

func runBootstrapWithDeps(params APIServerParams, state *startupstate.Tracker, deps bootstrapDeps) error {
	if params.RequireUpstreamOnStartup {
		return runBootstrapStrict(params, state, deps)
	}
	return runBootstrapResilient(params, state, deps)
}

func runBootstrapStrict(params APIServerParams, state *startupstate.Tracker, deps bootstrapDeps) error {
	err := backoff.RetryNotify(
		func() error {
			if err := deps.critical(params); err != nil {
				return err
			}
			return deps.background(params)
		},
		backoff.NewConstantBackOff(deps.retryInterval),
		func(err error, d time.Duration) {
			log.Printf("failed to bootstrap (requireUpstreamOnStartup=true), retrying in %s: %v", d, err)
		},
	)
	if err != nil {
		state.MarkFailed()
		return errors.Wrap(err, "failed to bootstrap")
	}
	state.MarkReady()
	return nil
}

func runBootstrapResilient(params APIServerParams, state *startupstate.Tracker, deps bootstrapDeps) error {
	criticalDone := make(chan error, 1)
	go func() {
		criticalDone <- backoff.RetryNotify(
			func() error { return deps.critical(params) },
			backoff.NewConstantBackOff(deps.retryInterval),
			func(err error, d time.Duration) {
				log.Printf("failed to bootstrap critical, retrying in %s: %v", d, err)
			},
		)
	}()

	timer := time.NewTimer(deps.deadline)
	defer timer.Stop()

	select {
	case criticalErr := <-criticalDone:
		if criticalErr != nil {
			state.MarkFailed()
			return errors.Wrap(criticalErr, "failed to bootstrap critical")
		}
		state.MarkReady()

	case <-timer.C:
		// Critical hasn't completed in the readiness window. Mark Ready
		// anyway — handlers will serve whatever the in-memory store has
		// (likely empty until critical completes) but the pod won't be
		// stuck in CrashLoopBackOff. Block here until critical does
		// resolve so we can decide whether to advance to background.
		logger.Warnf(
			"sdk_ready_after_critical_timeout: bootstrapCritical did not complete within %s; marking pod Ready and continuing critical bootstrap in background",
			deps.deadline,
		)
		state.MarkReady()
		if criticalErr := <-criticalDone; criticalErr != nil {
			state.MarkFailed()
			return errors.Wrap(criticalErr, "failed to bootstrap critical (after readiness)")
		}
		// Symmetric counterpart to sdk_ready_after_critical_timeout —
		// gives operators a closing log line when the deferred critical
		// path eventually succeeds, instead of leaving them to infer it
		// from the absence of further retry warnings.
		logger.Infof("sdk_critical_succeeded_after_readiness: bootstrapCritical completed after the readiness deadline; advancing to background phase")
	}

	if err := deps.background(params); err != nil {
		logger.Warnf("bootstrap background phase completed with errors (handlers continue serving from in-memory store): %v", err)
	}
	return nil
}

// loadTLSConfig loads TLS certificate and key from a Kubernetes secret
func loadTLSConfig(clientset kubernetes.Interface, namespace, secretName string) (*tls.Config, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get TLS secret %s", secretName)
	}

	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, errors.Errorf("tls.crt not found in secret %s", secretName)
	}

	keyData, ok := secret.Data["tls.key"]
	if !ok {
		return nil, errors.Errorf("tls.key not found in secret %s", secretName)
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse TLS certificate and key")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
