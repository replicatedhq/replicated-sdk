package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/metrics"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type SendCustomApplicationMetricsRequest struct {
	Data ApplicationMetricsData `json:"data"`
}

type ApplicationMetricsData map[string]interface{}

func SendCustomApplicationMetrics(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if util.IsAirgap() {
		JSON(w, http.StatusForbidden, "This request cannot be satisfied in airgap mode")
		return
	}

	request := SendCustomApplicationMetricsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Error(errors.Wrap(err, "decode request"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := validateCustomMetricsData(request.Data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = metrics.SendApplicationMetricsData(store.GetStore(), clientset, license, request.Data)
	if err != nil {
		logger.Error(errors.Wrap(err, "set application data"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	JSON(w, http.StatusOK, "")
}

func validateCustomMetricsData(data ApplicationMetricsData) error {
	if len(data) == 0 {
		return errors.New("no data provided")
	}

	for key, val := range data {
		valType := reflect.TypeOf(val)
		if valType == nil {
			return errors.Errorf("%s value is nil, only scalar values are allowed", key)
		}

		switch valType.Kind() {
		case reflect.Slice:
			return errors.Errorf("%s value is an array, only scalar values are allowed", key)
		case reflect.Array:
			return errors.Errorf("%s value is an array, only scalar values are allowed", key)
		case reflect.Map:
			return errors.Errorf("%s value is a map, only scalar values are allowed", key)
		}
	}

	return nil
}
