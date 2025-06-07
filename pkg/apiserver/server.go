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
	StatusInformers       []appstatetypes.StatusInformerString
	ReplicatedID          string
	AppID                 string
	Namespace             string
	TlsCertSecretName     string
}

func Start(params APIServerParams) {
	log.Println("Replicated version:", buildversion.Version())

	backoffDuration := 10 * time.Second
	bootstrapFn := func() error {
		return bootstrap(params)
	}
	err := backoff.RetryNotify(bootstrapFn, backoff.NewConstantBackOff(backoffDuration), func(err error, d time.Duration) {
		log.Printf("failed to bootstrap, retrying in %s: %v", d, err)
	})
	if err != nil {
		log.Fatalf("failed to bootstrap: %v", err)
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
			logger.Error(errors.Wrap(err, "failed to get clientset"))
			return
		}

		tlsConfig, err := loadTLSConfig(clientset, params.Namespace, params.TlsCertSecretName)
		if err != nil {
			log.Fatalf("failed to load TLS config: %v", err)
		}
		srv.TLSConfig = tlsConfig

		log.Printf("Starting Replicated API on port %d with TLS...\n", 3000)
		log.Fatal(srv.ListenAndServeTLS("", ""))
	} else {
		log.Printf("Starting Replicated API on port %d...\n", 3000)
		log.Fatal(srv.ListenAndServe())
	}
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
