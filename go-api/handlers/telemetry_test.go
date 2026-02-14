package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseTopic(t *testing.T) {
	t.Parallel()

	tenantID, deviceID, slot, err := parseTopic("tenants/t1/devices/d1/telemetry/slot/99")
	if err != nil {
		t.Fatalf("parseTopic unexpected error: %v", err)
	}
	if tenantID != "t1" || deviceID != "d1" || slot != 99 {
		t.Fatalf("parseTopic returned wrong values tenant=%s device=%s slot=%d", tenantID, deviceID, slot)
	}

	_, _, _, err = parseTopic("invalid/topic")
	if err == nil {
		t.Fatalf("parseTopic expected error for invalid topic")
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	now, err := parseTimestamp("")
	if err != nil {
		t.Fatalf("parseTimestamp empty unexpected error: %v", err)
	}
	if now.IsZero() {
		t.Fatalf("parseTimestamp empty returned zero time")
	}

	const ms = int64(1700000000000)
	got, err := parseTimestamp("1700000000000")
	if err != nil {
		t.Fatalf("parseTimestamp millis unexpected error: %v", err)
	}
	want := time.UnixMilli(ms).UTC()
	if !got.Equal(want) {
		t.Fatalf("parseTimestamp got %v, want %v", got, want)
	}
}

func TestWebhookRejectsInvalidTopic(t *testing.T) {
	t.Parallel()

	h := &TelemetryHandler{}
	body := `{"topic":"invalid/topic","payload":{"value":1},"timestamp":"1700000000000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/telemetry", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Webhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Webhook status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestTelemetryReadsRequireTenantContext(t *testing.T) {
	t.Parallel()

	h := &TelemetryHandler{}

	reqLatest := httptest.NewRequest(http.MethodGet, "/api/telemetry/latest?device_id=11111111-1111-1111-1111-111111111111&slot=0", nil)
	wLatest := httptest.NewRecorder()
	h.GetLatest(wLatest, reqLatest)
	if wLatest.Code != http.StatusUnauthorized {
		t.Fatalf("GetLatest status = %d, want %d", wLatest.Code, http.StatusUnauthorized)
	}

	reqSlots := httptest.NewRequest(http.MethodGet, "/api/telemetry/slots?device_id=11111111-1111-1111-1111-111111111111", nil)
	wSlots := httptest.NewRecorder()
	h.GetActiveSlots(wSlots, reqSlots)
	if wSlots.Code != http.StatusUnauthorized {
		t.Fatalf("GetActiveSlots status = %d, want %d", wSlots.Code, http.StatusUnauthorized)
	}
}
