package apiserver

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/mux"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
)

type APIServerParams struct {
	Context               context.Context
	License               *kotsv1beta1.License
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
	Namespace             string
}

func Start(params APIServerParams) {
	log.Println("Replicated SDK version:", buildversion.Version())

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

	r.HandleFunc("/healthz", handlers.Healthz)

	// license
	r.HandleFunc("/api/v1/license/info", handlers.GetLicenseInfo).Methods("GET")
	r.HandleFunc("/api/v1/license/fields", handlers.GetLicenseFields).Methods("GET")
	r.HandleFunc("/api/v1/license/fields/{fieldName}", handlers.GetLicenseField).Methods("GET")

	// app
	r.HandleFunc("/api/v1/app/info", handlers.GetCurrentAppInfo).Methods("GET")
	r.HandleFunc("/api/v1/app/updates", handlers.GetAppUpdates).Methods("GET")
	r.HandleFunc("/api/v1/app/history", handlers.GetAppHistory).Methods("GET")

	// integration
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.PostIntegrationMockData)).Methods("POST")
	r.HandleFunc("/api/v1/integration/mock-data", handlers.EnforceMockAccess(handlers.GetIntegrationMockData)).Methods("GET")
	r.HandleFunc("/api/v1/integration/status", handlers.EnforceMockAccess(handlers.GetIntegrationStatus)).Methods("GET")

	// Custom metrics
	r.HandleFunc("/api/v1/app/custom-metrics", handlers.SendCustomApplicationMetrics).Methods("POST")

	srv := &http.Server{
		Handler: r,
		Addr:    ":3000",
	}

	log.Printf("Starting Replicated SDK API on port %d...\n", 3000)

	log.Fatal(srv.ListenAndServe())
}
