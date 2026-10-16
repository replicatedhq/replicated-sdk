package installationtoken

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRotationPreservesInstallationIdentity(t *testing.T) {
	first, err := New("installation_123", 1)
	if err != nil {
		t.Fatal(err)
	}
	next, err := New(first.InstallationID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Secret == next.Secret {
		t.Fatal("rotation reused a secret")
	}
	for _, original := range []*Token{first, next} {
		encoded, err := original.Encode()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if *parsed != *original {
			t.Fatal("token did not round trip")
		}
		if strings.Contains(fmt.Sprintf("%v %+v %#v", original, original, original), original.Secret) {
			t.Fatal("formatting exposed the secret")
		}
	}
}

func TestRejectMalformedAndLegacyCredentials(t *testing.T) {
	token, _ := New("installation_123", 1)
	good, _ := token.Encode()
	data, _ := base64.RawURLEncoding.DecodeString(good)
	for name, raw := range map[string]string{
		"service account":   `{"i":"account","s":"secret"}`,
		"version":           strings.Replace(string(data), `"v":1`, `"v":2`, 1),
		"kind":              strings.Replace(string(data), `"installation"`, `"service_account"`, 1),
		"generation":        strings.Replace(string(data), `"g":1`, `"g":0`, 1),
		"duplicate":         strings.Replace(string(data), `"g":1`, `"g":2,"g":1`, 1),
		"extra field":       strings.Replace(string(data), `"g":1`, `"g":1,"admin":true`, 1),
		"trailing document": string(data) + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
	for _, invalid := range []string{"", "legacy-license-id", good + "=", strings.Repeat("x", 1025)} {
		if _, err := Parse(invalid); err == nil {
			t.Fatal("accepted invalid encoding")
		}
	}
}

func TestProofBindsCredentialOperationBodyAndTime(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	token, _ := New("installation_123", 1)
	encoded, _ := token.Encode()
	now := time.Unix(1700000000, 0)
	body := []byte(`{"rotationId":"rotation1"}`)
	proof, err := Sign(private, encoded, "POST", "/sdk/offer", body, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := proof.Verify(public, encoded, "POST", "/sdk/offer", body, now); err != nil {
		t.Fatal(err)
	}
	other, _ := New("installation_123", 2)
	otherEncoded, _ := other.Encode()
	for _, c := range []struct {
		name, token, method, path string
		body                      []byte
		now                       time.Time
	}{
		{"generation", otherEncoded, "POST", "/sdk/offer", body, now},
		{"method", encoded, "GET", "/sdk/offer", body, now},
		{"path", encoded, "POST", "/sdk/ack", body, now},
		{"body", encoded, "POST", "/sdk/offer", []byte(`{"rotationId":"rotation2"}`), now},
		{"expired", encoded, "POST", "/sdk/offer", body, now.Add(ProofLifetime + time.Second)},
		{"future", encoded, "POST", "/sdk/offer", body, now.Add(-time.Minute)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if proof.Verify(public, c.token, c.method, c.path, c.body, c.now) == nil {
				t.Fatal("accepted altered proof")
			}
		})
	}
	otherPublic, _, _ := ed25519.GenerateKey(rand.Reader)
	if proof.Verify(otherPublic, encoded, "POST", "/sdk/offer", body, now) == nil {
		t.Fatal("accepted another installation key")
	}
}
