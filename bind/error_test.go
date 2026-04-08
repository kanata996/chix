package bind

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/kanata996/chix/errx"
)

// 测试清单：
// - 标记说明：[✓] 已核对且已有真实覆盖；[x] 尚未完成，不得作为验收依据。
// - [✓] `NewBindingError` 会复制原始 values，收敛为 `400 bad_request`，并保留底层 cause。
// - [✓] `BindingError.Error()` 在 nil、有 cause、无 cause 三种形态下都输出稳定文本。
// - [✓] `BindingError.Unwrap()` 和 `MarshalJSON()` 对 nil receiver 与正常对象都保持稳定语义。
// - [✓] `invalidJSONError` / `unsupportedMediaTypeError` 的状态码、错误码、detail 与错误链行为稳定。
func TestNewBindingErrorClonesValues(t *testing.T) {
	cause := errors.New("invalid syntax")
	values := []string{"oops"}

	err := NewBindingError("page", values, "query param", cause)
	bindingErr, ok := err.(*BindingError)
	if !ok {
		t.Fatalf("error type = %T, want *BindingError", err)
	}

	values[0] = "changed"
	if !reflect.DeepEqual(bindingErr.Values, []string{"oops"}) {
		t.Fatalf("Values = %#v, want %#v", bindingErr.Values, []string{"oops"})
	}
	if bindingErr.Field != "page" || bindingErr.Status() != http.StatusBadRequest || bindingErr.Code() != "bad_request" || bindingErr.Detail() != "query param" {
		t.Fatalf("binding error = %#v", bindingErr)
	}
	if !errors.Is(bindingErr, cause) {
		t.Fatalf("errors.Is(bindingErr, cause) = false, want true")
	}
}

func TestBindingErrorError(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var err *BindingError
		if got := err.Error(); got != "" {
			t.Fatalf("Error() = %q, want empty", got)
		}
	})

	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("invalid syntax")
		err := &BindingError{
			Field:     "page",
			HTTPError: errx.NewHTTPErrorWithCause(http.StatusBadRequest, "", "query param", cause),
		}

		if got := err.Error(); got != "code=400, message=query param, err=invalid syntax, field=page" {
			t.Fatalf("Error() = %q", got)
		}
	})

	t.Run("without cause", func(t *testing.T) {
		err := &BindingError{
			Field:     "page",
			HTTPError: errx.NewHTTPError(http.StatusBadRequest, "", "query param"),
		}

		if got := err.Error(); got != "code=400, message=query param, field=page" {
			t.Fatalf("Error() = %q", got)
		}
	})
}

func TestBindingErrorUnwrap(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var err *BindingError
		if got := err.Unwrap(); got != nil {
			t.Fatalf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("returns embedded http error", func(t *testing.T) {
		httpErr := errx.NewHTTPError(http.StatusBadRequest, "", "query param")
		err := &BindingError{Field: "page", HTTPError: httpErr}

		if got := err.Unwrap(); got != httpErr {
			t.Fatalf("Unwrap() = %v, want same HTTPError", got)
		}
	})
}

func TestBindingErrorMarshalJSON(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var err *BindingError
		data, marshalErr := err.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("MarshalJSON() error = %v", marshalErr)
		}
		if string(data) != "{}" {
			t.Fatalf("MarshalJSON() = %s, want {}", data)
		}
	})

	t.Run("public shape", func(t *testing.T) {
		err := &BindingError{
			Field:     "page",
			Values:    []string{"oops"},
			HTTPError: errx.NewHTTPError(http.StatusBadRequest, "", "query param"),
		}

		data, marshalErr := err.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("MarshalJSON() error = %v", marshalErr)
		}

		var got map[string]string
		if unmarshalErr := json.Unmarshal(data, &got); unmarshalErr != nil {
			t.Fatalf("json.Unmarshal() error = %v", unmarshalErr)
		}

		want := map[string]string{
			"field":   "page",
			"message": "query param",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %#v, want %#v", got, want)
		}
	})
}

func TestBodyErrorHelpers(t *testing.T) {
	cause := errors.New("invalid character")

	err := invalidJSONError(cause)
	assertHTTPError(t, err, http.StatusBadRequest, CodeInvalidJSON, "request body must be valid JSON")
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}

	err = unsupportedMediaTypeError()
	assertHTTPError(t, err, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
}
