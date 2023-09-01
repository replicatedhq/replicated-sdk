package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
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
	if r.Header.Get("Authorization") != license.Spec.LicenseID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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

	err := metrics.SendApplicationMetricsData(store.GetStore(), license, request.Data)
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
