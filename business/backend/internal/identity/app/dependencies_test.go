package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

func TestNewAccessIdentityAcceptsSeedAndExpandedPrivateKey(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	values := []string{
		base64.RawURLEncoding.EncodeToString(privateKey.Seed()),
		base64.RawURLEncoding.EncodeToString(privateKey),
	}
	for _, value := range values {
		issuer, verifier, err := newAccessIdentity(config.AccessIdentity{
			Issuer: "liveshop-identity", KeyID: "test-key", PrivateKey: value,
		})
		if err != nil || issuer == nil || verifier == nil {
			t.Fatalf("valid Ed25519 material rejected: %v", err)
		}
	}
}

func TestNewAccessIdentityRejectsMalformedKey(t *testing.T) {
	issuer, verifier, err := newAccessIdentity(config.AccessIdentity{
		Issuer: "liveshop-identity", KeyID: "test-key", PrivateKey: "invalid",
	})
	if err == nil || issuer != nil || verifier != nil {
		t.Fatal("malformed key was accepted")
	}
}
