package gateway_test

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"testing"

	"github.com/bolaji/baki/internal/config"
	"github.com/bolaji/baki/internal/gateway"
)

func TestPaystackGateway_VerifyWebhookSignature(t *testing.T) {
	secret := "test_secret_webhook_key_123"
	cfg := &config.Config{
		PaystackWebhookKey: secret,
	}

	gw := gateway.NewPaystackGateway(cfg)
	payload := []byte(`{"event":"charge.success","data":{"reference":"BAKI-REF-123"}}`)

	// Calculate valid signature
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(payload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !gw.VerifyWebhookSignature(payload, validSig) {
		t.Errorf("expected valid signature to be accepted")
	}

	invalidSig := "invalidsignature123456789"
	if gw.VerifyWebhookSignature(payload, invalidSig) {
		t.Errorf("expected invalid signature to be rejected")
	}
}
