package reqx

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] 内部默认配置会保留 body/query/header 的基线绑定行为。

import "testing"

// defaultBindConfig 会返回内部默认绑定配置。
func TestDefaultBindConfig(t *testing.T) {
	cfg := defaultBindConfig()

	if cfg.body.maxBodyBytes != defaultMaxBodyBytes {
		t.Fatalf("maxBodyBytes = %d, want %d", cfg.body.maxBodyBytes, defaultMaxBodyBytes)
	}
	if !cfg.body.allowUnknownFields {
		t.Fatal("body.allowUnknownFields = false, want true")
	}
	if !cfg.query.allowUnknownFields {
		t.Fatal("query.allowUnknownFields = false, want true")
	}
	if !cfg.header.allowUnknownFields {
		t.Fatal("header.allowUnknownFields = false, want true")
	}
}
