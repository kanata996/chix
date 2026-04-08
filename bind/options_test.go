package bind

import "testing"

func TestDefaultBindConfig(t *testing.T) {
	cfg := defaultBindConfig()

	if cfg.body.maxBodyBytes != defaultMaxBodyBytes {
		t.Fatalf("maxBodyBytes = %d, want %d", cfg.body.maxBodyBytes, defaultMaxBodyBytes)
	}
	if !cfg.body.allowUnknownFields {
		t.Fatal("body.allowUnknownFields = false, want true")
	}
}
