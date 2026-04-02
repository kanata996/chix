package resp

import (
	"strings"
	"testing"
)

type logDetailStruct struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// 已经是安全 map 结构的 details 会走快路径并保留原有键值。
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

// 快路径失败时，可序列化的结构体 details 仍会被回退为安全日志对象。
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

// 超出日志预算的 details 会被整体丢弃，避免错误日志失控。
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
