package apiserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/appstate"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

type APIServerParams struct {
	License                *kotsv1beta1.License
	LicenseFields          sdklicensetypes.LicenseFields
	AppName                string
	ChannelID              string
	ChannelName            string
	ChannelSequence        int64
	ReleaseSequence        int64
	ReleaseCreatedAt       string
	ReleaseNotes           string
	VersionLabel           string
	ReplicatedAppEndpoint  string
	InformersLabelSelector string
	Namespace              string
}

func Start(params APIServerParams) {
	log.Println("Replicated version:", buildversion.Version())

	storeOptions := store.InitInMemoryStoreOptions{
		License:               params.License,
		LicenseFields:         params.LicenseFields,
		AppName:               params.AppName,
		ChannelID:             params.ChannelID,
		ChannelName:           params.ChannelName,
		ChannelSequence:       params.ChannelSequence,
		ReleaseSequence:       params.ReleaseSequence,
		ReleaseCreatedAt:      params.ReleaseCreatedAt,
		ReleaseNotes:          params.ReleaseNotes,
		VersionLabel:          params.VersionLabel,
		ReplicatedAppEndpoint: params.ReplicatedAppEndpoint,
		Namespace:             params.Namespace,
	}
	if err := store.InitInMemory(storeOptions); err != nil {
		log.Fatalf("Failed to init store: %v", err)
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		log.Fatalf("Failed to get clientset: %v", err)
	}

	targetNamespace := params.Namespace
	if k8sutil.IsReplicatedClusterScoped(context.Background(), clientset, params.Namespace) {
		targetNamespace = "" // watch all namespaces
	}
	appStateOperator := appstate.InitOperator(clientset, targetNamespace)
	appStateOperator.Start()

	appStateOperator.ApplyAppInformers(appstatetypes.AppInformersArgs{
		AppSlug:       store.GetStore().GetAppSlug(),
		Sequence:      store.GetStore().GetReleaseSequence(),
		LabelSelector: params.InformersLabelSelector,
	})

	if err := heartbeat.Start(); err != nil {
		log.Println("Failed to start heartbeat:", err)
	}

	if store.GetStore().IsDevLicense() {
		logger.Info("Detected dev license, initializing mock")
		mock.InitMock(clientset, store.GetStore().GetNamespace())
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

	// mock-data
	r.HandleFunc("/api/v1/mock-data", handlers.EnforceMockAccess(handlers.PostMockData)).Methods("POST")
	r.HandleFunc("/api/v1/mock-data", handlers.EnforceMockAccess(handlers.GetMockData)).Methods("GET")

	srv := &http.Server{
		Handler: r,
		Addr:    ":3000",
	}

	fmt.Printf("Starting Replicated API on port %d...\n", 3000)

	log.Fatal(srv.ListenAndServe())
}
