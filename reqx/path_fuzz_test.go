package reqx

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] path wildcard 解析与存在性判断在任意 pattern/name 输入下保持稳定且不 panic。
// - [✓] `BindPathValues` 与 `ParamInt` 在任意 pattern/path value 输入下维持稳定的公开契约。
// - [✓] 本次 path 解析重构已补 fuzz 覆盖，满足“query/path/header 解析优先评估 fuzz”的仓库规范。

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func FuzzPathWildcardHelpers(f *testing.F) {
	f.Add("", "")
	f.Add("/users/{id}", "id")
	f.Add("GET /files/{path...}/{$}", "path")
	f.Add("/users/{ id :int }", "id")
	f.Add("/users/{id", "id")
	f.Add("/users/{account_id}", "id")

	f.Fuzz(func(t *testing.T, pattern, name string) {
		names := pathWildcardNames(pattern)
		for i, wildcard := range names {
			if strings.TrimSpace(wildcard) == "" {
				t.Fatalf("names[%d] = %q, want non-blank wildcard", i, wildcard)
			}
			if wildcard == "$" {
				t.Fatalf("names[%d] = %q, want anonymous wildcard filtered out", i, wildcard)
			}
		}

		want := false
		trimmedName := strings.TrimSpace(name)
		for _, wildcard := range names {
			if wildcard == trimmedName {
				want = true
				break
			}
		}

		if got := pathWildcardExists(pattern, name); got != want {
			t.Fatalf("pathWildcardExists(%q, %q) = %v, want %v (names = %#v)", pattern, name, got, want, names)
		}
	})
}

func FuzzPathPublicAPIs(f *testing.F) {
	f.Add("/users/{id}", "42")
	f.Add("/users/{id}", " 42 ")
	f.Add("/users/{id}", " ")
	f.Add("", "42")
	f.Add("", " ")
	f.Add("/users/{other}", "42")
	f.Add("/users/{id", "42")
	f.Add("GET /users/{id}/{$}", "oops")

	f.Fuzz(func(t *testing.T, pattern, rawValue string) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Pattern = pattern
		req.SetPathValue("id", rawValue)

		var dst struct {
			ID string `param:"id"`
		}
		dst.ID = "existing"

		if err := BindPathValues(req, &dst); err != nil {
			t.Fatalf("BindPathValues() error = %v", err)
		}

		trimmed := strings.TrimSpace(rawValue)
		if trimmed != "" {
			if dst.ID != trimmed {
				t.Fatalf("BindPathValues() id = %q, want %q", dst.ID, trimmed)
			}
		} else if pathWildcardExists(pattern, "id") {
			if dst.ID != "" {
				t.Fatalf("BindPathValues() id = %q, want empty string", dst.ID)
			}
		} else if dst.ID != "existing" {
			t.Fatalf("BindPathValues() id = %q, want preserved existing value", dst.ID)
		}

		got, err := ParamInt(req, "id")
		if trimmed == "" {
			violation := assertSingleViolation(t, err)
			if violation.Field != "id" || violation.Code != ViolationCodeRequired {
				t.Fatalf("violation = %#v, want required id violation", violation)
			}
			return
		}

		want, parseErr := strconv.Atoi(trimmed)
		if parseErr != nil {
			violation := assertSingleViolation(t, err)
			if violation.Field != "id" || violation.Code != ViolationCodeType {
				t.Fatalf("violation = %#v, want type id violation", violation)
			}
			return
		}

		if err != nil {
			t.Fatalf("ParamInt() error = %v", err)
		}
		if got != want {
			t.Fatalf("ParamInt() = %d, want %d", got, want)
		}
	})
}
