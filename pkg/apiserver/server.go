package apiserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/kots-sdk/pkg/appstate"
	appstatetypes "github.com/replicatedhq/kots-sdk/pkg/appstate/types"
	"github.com/replicatedhq/kots-sdk/pkg/handlers"
	"github.com/replicatedhq/kots-sdk/pkg/k8sutil"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/util"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
)

type APIServerParams struct {
	License                *kotsv1beta1.License
	ChannelID              string
	ChannelName            string
	ChannelSequence        int64
	ReleaseSequence        int64
	VersionLabel           string
	InformersLabelSelector string
}

func Start(params APIServerParams) {
	storeOptions := store.InitStoreOptions{
		License:         params.License,
		ChannelID:       params.ChannelID,
		ChannelName:     params.ChannelName,
		ChannelSequence: params.ChannelSequence,
		ReleaseSequence: params.ReleaseSequence,
		VersionLabel:    params.VersionLabel,
	}
	if err := store.Init(storeOptions); err != nil {
		log.Fatalf("Failed to init store: %v", err)
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		log.Fatalf("Failed to get clientset: %v", err)
	}

	targetNamespace := util.PodNamespace
	if k8sutil.IsKotsSDKClusterScoped(context.Background(), clientset, util.PodNamespace) {
		targetNamespace = "" // watch all namespaces
	}
	appStateOperator := appstate.InitOperator(clientset, targetNamespace)
	appStateOperator.Start()

	appStateOperator.ApplyAppInformers(appstatetypes.AppInformersArgs{
		AppSlug:       store.GetStore().GetAppSlug(),
		Sequence:      store.GetStore().GetReleaseSequence(),
		LabelSelector: params.InformersLabelSelector,
	})

	r := mux.NewRouter()
	r.Use(handlers.CorsMiddleware)

	r.HandleFunc("/healthz", handlers.Healthz)

	// license
	r.HandleFunc("/api/v1/licensefields", handlers.GetLicenseFields).Methods("GET")
	r.HandleFunc("/api/v1/licensefield/{fieldName}", handlers.GetLicenseField).Methods("GET")

	// app
	r.HandleFunc("/api/v1/app/info", handlers.GetCurrentAppInfo).Methods("GET")
	r.HandleFunc("/api/v1/app/updatecheck", handlers.CheckForUpdates).Methods("GET")

	srv := &http.Server{
		Handler: r,
		Addr:    ":3000",
	}

	fmt.Printf("Starting KOTS SDK API on port %d...\n", 3000)

	log.Fatal(srv.ListenAndServe())
}
