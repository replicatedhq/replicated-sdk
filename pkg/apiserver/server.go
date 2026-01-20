package apiserver

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/mux"
	pkgerrors "github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
}

func Start(params APIServerParams) error {
	logger.Infof("Replicated version: %s", buildversion.Version())

	// If the caller provided a non-cancellable context (or nil), panic so that these errors are caught in testing.
	ctx := params.Context
	if ctx == nil {
		panic("context is nil")
	} else if ctx.Done() == nil {
		panic("context is not cancellable")
	}
	// Use a derived context so we can cancel all long-running background components if the HTTP server exits early.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	params.Context = ctx

	backoffDuration := 10 * time.Second
	bootstrapFn := func() error {
		return bootstrap(params)
	}
	bo := backoff.WithContext(backoff.NewConstantBackOff(backoffDuration), ctx)
	err := backoff.RetryNotify(bootstrapFn, bo, func(err error, d time.Duration) {
		logger.Errorf("failed to bootstrap, retrying in %s: %v", d, err)
	})
	if err != nil {
		return pkgerrors.Wrap(err, "failed to bootstrap")
	}

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

	// integration
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.PostIntegrationMockData)).Methods("POST")
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.GetIntegrationMockData)).Methods("GET")
	r.HandleFunc("/api/v1/integration/status", handlers.EnforceMockAccess(handlers.GetIntegrationStatus)).Methods("GET")

	srv := &http.Server{
		Handler: r,
		Addr:    ":3000",
	}

	// Configure TLS if certificate name is provided
	if params.TlsCertSecretName != "" {
		clientset, err := k8sutil.GetClientset()
		if err != nil {
			logger.Error(pkgerrors.Wrap(err, "failed to get clientset"))
			return pkgerrors.Wrap(err, "failed to get clientset")
		}

		tlsConfig, err := loadTLSConfig(clientset, params.Namespace, params.TlsCertSecretName)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to load TLS config")
		}
		srv.TLSConfig = tlsConfig

		logger.Infof("Starting Replicated API on port %d with TLS...", 3000)
		err = serveWithShutdown(ctx, srv, func() error { return srv.ListenAndServeTLS("", "") })
		cancel()
		waitForLeaderElectionStop(10 * time.Second)
		return err
	} else {
		logger.Infof("Starting Replicated API on port %d...", 3000)
		err = serveWithShutdown(ctx, srv, srv.ListenAndServe)
		cancel()
		waitForLeaderElectionStop(10 * time.Second)
		return err
	}
}

func serveWithShutdown(ctx context.Context, srv *http.Server, serveFn func() error) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveFn()
	}()

	select {
	case <-ctx.Done():
		// Best-effort graceful shutdown. This is what triggers leader-election ReleaseOnCancel to run too.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)

		// Prefer the server's return value, but don't wait longer than shutdown timeout.
		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-shutdownCtx.Done():
			return nil
		}
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// loadTLSConfig loads TLS certificate and key from a Kubernetes secret
func loadTLSConfig(clientset kubernetes.Interface, namespace, secretName string) (*tls.Config, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to get TLS secret %s", secretName)
	}

	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, pkgerrors.Errorf("tls.crt not found in secret %s", secretName)
	}

	keyData, ok := secret.Data["tls.key"]
	if !ok {
		return nil, pkgerrors.Errorf("tls.key not found in secret %s", secretName)
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to parse TLS certificate and key")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
