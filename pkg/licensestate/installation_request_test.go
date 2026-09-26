package licensestate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/installationtoken"
	"github.com/stretchr/testify/require"
)

func TestPortalRequestUsesTokenAndSignsExactRequest(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	token, err := installationtoken.New("installation-one", 2)
	require.NoError(t, err)
	encoded, err := token.Encode()
	require.NoError(t, err)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got != encoded {
			t.Error("request used the wrong credential")
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		timestamp, err := strconv.ParseInt(r.Header.Get("X-Replicated-Installation-Timestamp"), 10, 64)
		require.NoError(t, err)
		proof := installationtoken.Proof{Timestamp: timestamp, Nonce: r.Header.Get("X-Replicated-Installation-Nonce"), Signature: r.Header.Get("X-Replicated-Installation-Signature")}
		require.NoError(t, proof.Verify(public, encoded, r.Method, r.URL.EscapedPath(), body, time.Now()))
		require.Error(t, proof.Verify(public, encoded, r.Method, "/different-operation", body, time.Now()))
		require.Error(t, proof.Verify(public, encoded, r.Method, r.URL.EscapedPath(), []byte("{}"), time.Now()))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	state := &State{InstallationID: token.InstallationID, CredentialGeneration: 2, PrivateKey: private,
		ActiveLicense: []byte("apiVersion: kots.io/v1beta1\nkind: License\nmetadata:\n  name: test\nspec:\n  licenseID: " + encoded + "\n")}
	status, err := NewPortalClient(server.URL).post(context.Background(), "/rotation-offer", map[string]string{"installationId": state.InstallationID}, &struct{}{}, state)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)
	require.True(t, called)
	called = false
	state.PrivateKey = nil
	_, err = NewPortalClient(server.URL).post(context.Background(), "/rotation-offer", map[string]string{"installationId": state.InstallationID}, &struct{}{}, state)
	require.Error(t, err)
	require.False(t, called, "missing key must fail before contacting Portal")
}
