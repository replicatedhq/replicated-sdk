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

	// bootstrapBackgroundMaxRetries caps how many times the background
	// phase is retried before the orchestrator gives up. bootstrapBackground
	// joins its step errors via stderrors.Join, which strips any
	// backoff.Permanent wrapping from individual steps, so without an
	// explicit cap the retry loop would spin forever on a persistent
	// failure (log-spamming every retryInterval) instead of giving up.
	// 30 attempts × 10s ≈ 5 minutes of recovery window; after that the
	// heartbeat cron tick (every 4 hours) is the long-term refresh path.
	bootstrapBackgroundMaxRetries = 30
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

	// DevOffline, when true, instructs bootstrapCritical to validate
	// the loaded license is a dev license and then flip the runtime
	// airgap flag so all !util.IsAirgap() gates skip their upstream
	// calls. Production licenses are rejected at bootstrap with a
	// permanent error so this opt-in cannot silently disable upstream
	// behavior in production. Default false.
	DevOffline bool
}

func Start(params APIServerParams) {
	log.Println("Replicated version:", buildversion.Version())

	state := startupstate.New()
	handlers.SetStartupState(state)

	srv, err := buildServer(params)
	if err != nil {
		// Pre-Phase-1 this same condition (most commonly: TLS secret
		// missing or malformed) terminated the process with exit code
		// 1 via log.Fatalf. Returning early instead would cause the
		// cobra RunE to return nil and the binary to exit 0, masking
		// a misconfigured deployment as a "successful" run. Keep the
		// fatal exit so the kubelet sees a CrashLoopBackOff signal
		// and operators get an obvious failure mode.
		log.Fatalf("failed to build server: %v", err)
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
	critical         func(APIServerParams) error
	background       func(APIServerParams) error
	deadline         time.Duration
	retryInterval    time.Duration
	bgMaxRetries     uint64
}

func defaultBootstrapDeps() bootstrapDeps {
	return bootstrapDeps{
		critical:      bootstrapCritical,
		background:    bootstrapBackground,
		deadline:      bootstrapCriticalDeadline,
		retryInterval: bootstrapRetryInterval,
		bgMaxRetries:  bootstrapBackgroundMaxRetries,
	}
}

// runBootstrap drives the bootstrap state machine. bootstrapCritical is
// retried with backoff; the pod is marked Ready as soon as critical
// succeeds OR the bootstrapCriticalDeadline elapses, whichever comes
// first. After the deadline, critical continues to retry; on eventual
// success the bootstrapBackground phase runs. bootstrapBackground errors
// are logged but never block readiness.
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
			// We have already signaled Ready, so the kubelet has
			// (or is about to) added this pod to its Service
			// endpoints. Flipping back to Failed → log.Fatalf would
			// produce a Ready→crash→restart→Ready→crash loop where
			// every cycle briefly exposes an under-initialized
			// store to traffic. The contract is "serve degraded
			// rather than flap" — log loudly and keep the pod
			// alive. Background work below is skipped because it
			// depends on store state that critical never populated;
			// the heartbeat won't run, but existing endpoints will
			// continue to answer with whatever defaults the
			// in-memory store has.
			logger.Errorf(
				"sdk_critical_failed_after_readiness: bootstrapCritical failed after the readiness deadline; pod stays Ready to avoid a Ready→crash flap, but the in-memory store may be under-initialized and downstream handlers may return degraded data: %v",
				criticalErr,
			)
			return nil
		}
		// Symmetric counterpart to sdk_ready_after_critical_timeout —
		// gives operators a closing log line when the deferred critical
		// path eventually succeeds, instead of leaving them to infer it
		// from the absence of further retry warnings.
		logger.Infof("sdk_critical_succeeded_after_readiness: bootstrapCritical completed after the readiness deadline; advancing to background phase")
	}

	// Retry bootstrapBackground for a bounded number of attempts. Without
	// this loop, a transient failure on the first attempt (e.g.
	// heartbeat.Start hitting a momentary cron init issue, or upstream
	// being briefly unreachable while the pod was already marked Ready
	// via the timeout path) would leave the heartbeat cron job and any
	// subsequent license refreshes disabled for the entire pod lifetime.
	//
	// The cap matters because bootstrapBackground returns
	// stderrors.Join(errs...), which strips any backoff.Permanent
	// wrapping from inner steps; without WithMaxRetries, a persistent
	// background failure would log-spam every retryInterval forever and
	// the give-up branch below would be dead code. After the cap, the
	// heartbeat cron tick (every 4 hours) takes over as the long-term
	// refresh path. Errors are still logged at Warn level on each
	// attempt and never block the pod from staying Ready.
	bgBackoff := backoff.WithMaxRetries(backoff.NewConstantBackOff(deps.retryInterval), deps.bgMaxRetries)
	if err := backoff.RetryNotify(
		func() error { return deps.background(params) },
		bgBackoff,
		func(err error, d time.Duration) {
			logger.Warnf("bootstrap background phase failed, retrying in %s (handlers continue serving from in-memory store): %v", d, err)
		},
	); err != nil {
		logger.Warnf("bootstrap background phase gave up (handlers continue serving from in-memory store; heartbeat cron will retry on next tick): %v", err)
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
