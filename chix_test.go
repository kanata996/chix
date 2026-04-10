package chix

import (
	"encoding/json"
	"strings"
	"testing"
)

type rootPayloadMap map[string]any

func decodeRootPayload(t *testing.T, body []byte) rootPayloadMap {
	t.Helper()

	var payload rootPayloadMap
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", strings.TrimSpace(string(body)), err)
	}
	return payload
}
