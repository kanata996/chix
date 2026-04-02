package resp

import (
	"strings"
	"testing"
)

type logDetailStruct struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func TestSafeErrorLogDetailsFastPath(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"field": "name",
			"code":  "required",
			"count": 2,
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetails() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}

	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if detail["field"] != "name" || detail["code"] != "required" || detail["count"] != 2 {
		t.Fatalf("detail = %#v, want field/code/count preserved", detail)
	}
}

func TestSafeErrorLogDetailsFallbackPreservesMarshalableStruct(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		logDetailStruct{
			Field: "name",
			Code:  "required",
		},
	})
	if !ok {
		t.Fatal("safeErrorLogDetails() ok = false, want true")
	}

	items, ok := safeDetails.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("safeDetails = %#v, want one []any item", safeDetails)
	}

	detail, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want map[string]any", items[0])
	}
	if detail["field"] != "name" || detail["code"] != "required" {
		t.Fatalf("detail = %#v, want field/code preserved", detail)
	}
}

func TestSafeErrorLogDetailsDropsOversizedPayload(t *testing.T) {
	safeDetails, ok := safeErrorLogDetails([]any{
		map[string]any{
			"message": strings.Repeat("a", maxLoggedErrorDetailsSize),
		},
	})
	if ok || safeDetails != nil {
		t.Fatalf("safeErrorLogDetails() = (%#v, %v), want (nil, false)", safeDetails, ok)
	}
}
