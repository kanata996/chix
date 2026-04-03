package reqx

// 用例清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 本轮审查发现缺口后补测。
// - [✓] `WithMaxBodyBytes` 会覆盖 body 限制，同时保留其余默认绑定配置。

import "testing"

// applyBindOptions 会保留默认标志并应用 body 大小限制。
func TestApplyBindOptions_PreservesDefaultsAndAppliesMaxBodyBytes(t *testing.T) {
	cfg := applyBindOptions(WithMaxBodyBytes(8))

	if cfg.body.maxBodyBytes != 8 {
		t.Fatalf("maxBodyBytes = %d, want 8", cfg.body.maxBodyBytes)
	}
	if !cfg.body.allowUnknownFields {
		t.Fatal("body.allowUnknownFields = false, want true")
	}
	if !cfg.body.allowEmptyBody {
		t.Fatal("body.allowEmptyBody = false, want true")
	}
	if !cfg.query.allowUnknownFields {
		t.Fatal("query.allowUnknownFields = false, want true")
	}
	if !cfg.header.allowUnknownFields {
		t.Fatal("header.allowUnknownFields = false, want true")
	}
}
