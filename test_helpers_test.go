package chix

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeRootPayload(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", strings.TrimSpace(string(body)), err)
	}
	return payload
}
