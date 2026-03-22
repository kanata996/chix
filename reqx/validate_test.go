package reqx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/reqx"
)

func TestValidateReturnsNilWhenValidatorIsNil(t *testing.T) {
	req := createUserRequest{Name: "alice"}

	if err := reqx.Validate(&req, nil); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateReturnsNilWhenNoViolations(t *testing.T) {
	req := createUserRequest{Name: "alice"}

	err := reqx.Validate(&req, func(value *createUserRequest) []reqx.Violation {
		if value.Name != "alice" {
			return []reqx.Violation{{Field: "name", Code: "invalid", Message: "is invalid"}}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsNilDestination(t *testing.T) {
	err := reqx.Validate[createUserRequest](nil, func(value *createUserRequest) []reqx.Violation {
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "chix/reqx: destination must not be nil" {
		t.Fatalf("error = %q, want destination must not be nil", got)
	}
}

func TestValidateNormalizesDefaultViolationFields(t *testing.T) {
	h := chix.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		req := createUserRequest{}
		return reqx.Validate(&req, func(value *createUserRequest) []reqx.Violation {
			return []reqx.Violation{{Field: "name"}}
		})
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	h.ServeHTTP(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "invalid_request", "request contains invalid fields")
	assertFirstDetail(t, rr, "name", "invalid", "is invalid")
}
