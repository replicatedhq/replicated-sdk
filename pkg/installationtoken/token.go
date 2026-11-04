// Package installationtoken defines the credential carried in an SDK-managed
// KOTS license's licenseID. Parsing a token does not authenticate it.
package installationtoken

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

const Kind = "installation"

var ErrInvalid = errors.New("invalid installation token")

// Token identifies one installation across all credential generations.
// Secret must never appear in logs, metrics, or public status responses.
type Token struct {
	Version        int    `json:"v"`
	Kind           string `json:"k"`
	InstallationID string `json:"i"`
	Secret         string `json:"s"`
	Generation     int64  `json:"g"`
}

func (t Token) String() string   { return "[installation token redacted]" }
func (t Token) GoString() string { return t.String() }

func New(installationID string, generation int64) (*Token, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	t := &Token{Version: 1, Kind: Kind, InstallationID: installationID,
		Secret: base64.RawURLEncoding.EncodeToString(secret), Generation: generation}
	if !t.valid() {
		return nil, ErrInvalid
	}
	return t, nil
}

func (t Token) Encode() (string, error) {
	if !t.valid() {
		return "", ErrInvalid
	}
	data, err := json.Marshal(t)
	if err != nil {
		return "", ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func Parse(encoded string) (*Token, error) {
	if len(encoded) == 0 || len(encoded) > 1024 {
		return nil, ErrInvalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(data) != encoded {
		return nil, ErrInvalid
	}
	// Require canonical JSON, including a single occurrence of each field.
	// This prevents different consumers from interpreting the same token
	// differently, and provides one wire format for request signatures.
	var t Token
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&t); err != nil || !t.valid() {
		return nil, ErrInvalid
	}
	if err := d.Decode(new(interface{})); err != io.EOF {
		return nil, ErrInvalid
	}
	canonical, err := t.Encode()
	if err != nil || canonical != encoded {
		return nil, ErrInvalid
	}
	return &t, nil
}

func (t Token) valid() bool {
	if t.Version != 1 || t.Kind != Kind || t.Generation <= 0 || len(t.InstallationID) == 0 || len(t.InstallationID) > 128 {
		return false
	}
	for _, c := range t.InstallationID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(t.Secret)
	return err == nil && len(secret) == 32 && base64.RawURLEncoding.EncodeToString(secret) == t.Secret
}
