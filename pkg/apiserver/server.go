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
	"github.com/replicatedhq/replicated-sdk/pkg/handlers"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type APIServerParams struct {
	License                *kotsv1beta1.License
	LicenseFields          sdklicensetypes.LicenseFields
	AppName                string
	ChannelID              string
	ChannelName            string
	ChannelSequence        int64
	ReleaseSequence        int64
	ReleaseIsRequired      bool
	ReleaseCreatedAt       string
	ReleaseNotes           string
	VersionLabel           string
	InformersLabelSelector string
	Namespace              string
}

func Start(params APIServerParams) {
	storeOptions := store.InitStoreOptions{
		License:           params.License,
		LicenseFields:     params.LicenseFields,
		AppName:           params.AppName,
		ChannelID:         params.ChannelID,
		ChannelName:       params.ChannelName,
		ChannelSequence:   params.ChannelSequence,
		ReleaseSequence:   params.ReleaseSequence,
		ReleaseIsRequired: params.ReleaseIsRequired,
		ReleaseCreatedAt:  params.ReleaseCreatedAt,
		ReleaseNotes:      params.ReleaseNotes,
		VersionLabel:      params.VersionLabel,
		Namespace:         params.Namespace,
	}
	if err := store.Init(storeOptions); err != nil {
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

	r := mux.NewRouter()
	r.Use(handlers.CorsMiddleware)

	r.HandleFunc("/healthz", handlers.Healthz)

	if util.IsDevModeEnabled() {
		logger.Info("dev mode enabled")
		err := handlers.RegisterDevModeRoutes(r)
		if err != nil {
			log.Fatalf("Failed to register dev mode routes: %v", err)
		}
	} else {
		handlers.RegisterProductionRoutes(r)
	}

	port := 3000
	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%d", port),
	}

	fmt.Printf("Starting Replicated API on port %d...\n", port)

	log.Fatal(srv.ListenAndServe())
}
