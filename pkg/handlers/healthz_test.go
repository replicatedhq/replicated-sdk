package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/replicated-sdk/pkg/startupstate"
	"github.com/stretchr/testify/require"
)

func TestHealthz_Starting_Returns503(t *testing.T) {
	tr := startupstate.New()
	SetStartupState(tr)
	t.Cleanup(func() { SetStartupState(nil) })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	Healthz(w, r)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body HealthzResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "starting", body.Status)
}

func TestHealthz_Ready_Returns200(t *testing.T) {
	tr := startupstate.New()
	tr.MarkReady()
	SetStartupState(tr)
	t.Cleanup(func() { SetStartupState(nil) })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	Healthz(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var body HealthzResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "ready", body.Status)
}
