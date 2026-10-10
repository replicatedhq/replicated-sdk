package installationtoken

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const ProofLifetime = 5 * time.Minute

var ErrProof = errors.New("invalid installation request proof")

// Proof travels in headers so the signature can cover the exact HTTP body.
// The server must also authenticate the token and atomically consume Nonce
// before performing the operation. Verify alone does not prevent replay.
type Proof struct {
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

func Sign(privateKey ed25519.PrivateKey, encodedToken, method, path string, body []byte, now time.Time) (*Proof, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, ErrProof
	}
	if _, err := Parse(encodedToken); err != nil {
		return nil, ErrProof
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	p := &Proof{Timestamp: now.Unix(), Nonce: base64.RawURLEncoding.EncodeToString(nonce)}
	p.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, p.message(encodedToken, method, path, body)))
	return p, nil
}

func (p Proof) Verify(publicKey ed25519.PublicKey, encodedToken, method, path string, body []byte, now time.Time) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return ErrProof
	}
	if _, err := Parse(encodedToken); err != nil {
		return ErrProof
	}
	timestamp := time.Unix(p.Timestamp, 0)
	if timestamp.Before(now.Add(-ProofLifetime)) || timestamp.After(now.Add(30*time.Second)) {
		return ErrProof
	}
	nonce, err := base64.RawURLEncoding.Strict().DecodeString(p.Nonce)
	if err != nil || len(nonce) != 32 {
		return ErrProof
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(p.Signature)
	if err != nil || !ed25519.Verify(publicKey, p.message(encodedToken, method, path, body), signature) {
		return ErrProof
	}
	return nil
}

func (p Proof) message(encodedToken, method, path string, body []byte) []byte {
	tokenHash := sha256.Sum256([]byte(encodedToken))
	bodyHash := sha256.Sum256(body)
	// An array gives unambiguous boundaries even if a caller supplies a newline.
	message, _ := json.Marshal([]interface{}{"replicated-installation-request/v1", method, path,
		hex.EncodeToString(tokenHash[:]), p.Timestamp, p.Nonce, hex.EncodeToString(bodyHash[:])})
	return message
}
