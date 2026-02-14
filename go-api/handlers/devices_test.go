package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"iiot-go-api/config"
)

func TestVerifyHMAC(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ManufacturingMasterKey: "test-master-key"}
	h := &DeviceHandler{Config: cfg}

	deviceID := "11111111-1111-1111-1111-111111111111"
	ts := time.Now().UTC().Format(time.RFC3339)

	msg := deviceID + ":" + ts
	signature := hmacHex([]byte(cfg.ManufacturingMasterKey), msg)

	if !h.verifyHMAC(deviceID, ts, signature) {
		t.Fatalf("verifyHMAC should accept valid signature")
	}
	if h.verifyHMAC(deviceID, ts, strings.Repeat("0", 64)) {
		t.Fatalf("verifyHMAC should reject invalid signature")
	}
}

func TestVerifyTimestamp(t *testing.T) {
	t.Parallel()

	h := &DeviceHandler{Config: &config.Config{BootstrapMaxSkewSecs: 300}}

	if !h.verifyTimestamp(time.Now().UTC().Format(time.RFC3339)) {
		t.Fatalf("verifyTimestamp should accept current timestamp")
	}
	oldTs := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	if h.verifyTimestamp(oldTs) {
		t.Fatalf("verifyTimestamp should reject old timestamp")
	}
}

func TestGetSecretRejectsInvalidSignatureEarly(t *testing.T) {
	t.Parallel()

	h := &DeviceHandler{Config: &config.Config{
		ManufacturingMasterKey: "test-master-key",
		BootstrapMaxSkewSecs:   300,
	}}

	body := `{"device_id":"11111111-1111-1111-1111-111111111111","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `","signature":"deadbeef"}`
	req := httptest.NewRequest(http.MethodPost, "/api/devices/secret", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.GetSecret(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GetSecret status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func hmacHex(key []byte, msg string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
