package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUploadSupportBundle_Airgap(t *testing.T) {

	t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "true")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/app/supportbundle", nil)

	UploadSupportBundle(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
