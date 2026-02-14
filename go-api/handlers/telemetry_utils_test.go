package handlers

import (
	"strings"
	"testing"
)

func TestSanitizeJSONEscapes(t *testing.T) {
	t.Parallel()

	raw := []byte("{\"topic\":\"\\tenants/a/devices/b/telemetry/slot/1\",\"clientid\":\"\\mclient\",\"payload\":\\{\"value\":1}}")
	got := sanitizeJSONEscapes(raw)

	// Invalid escapes should be removed, preserving structural JSON escapes.
	if string(got) == string(raw) {
		t.Fatalf("sanitizeJSONEscapes should alter invalid escaped payload")
	}
	if !strings.Contains(string(got), "\"topic\":\"tenants/a/devices/b/telemetry/slot/1\"") {
		t.Fatalf("sanitized topic mismatch: %s", string(got))
	}
	if !strings.Contains(string(got), "\"clientid\":\"mclient\"") {
		t.Fatalf("sanitized clientid mismatch: %s", string(got))
	}
}

func TestToInt64(t *testing.T) {
	t.Parallel()

	if v, err := toInt64(int64(7)); err != nil || v != 7 {
		t.Fatalf("toInt64(int64) = (%d, %v), want (7, nil)", v, err)
	}
	if v, err := toInt64(int(3)); err != nil || v != 3 {
		t.Fatalf("toInt64(int) = (%d, %v), want (3, nil)", v, err)
	}
	if _, err := toInt64("nope"); err == nil {
		t.Fatalf("toInt64(string) expected error")
	}
}
