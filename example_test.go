package chix_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kanata996/chix"
)

func ExampleHandle() {
	type createUserInput struct {
		ID      string `path:"id"`
		Verbose bool   `query:"verbose"`
		Name    string `json:"name"`
	}

	type createUserOutput struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Verbose bool   `json:"verbose"`
	}

	rt := chix.New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users/{id}", chix.Handle(rt, chix.Operation[createUserInput, createUserOutput]{
		Method: http.MethodPost,
	}, func(_ context.Context, input *createUserInput) (*createUserOutput, error) {
		return &createUserOutput{
			ID:      input.ID,
			Name:    input.Name,
			Verbose: input.Verbose,
		}, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users/u_1?verbose=true", strings.NewReader(`{"name":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 201
	// {"data":{"id":"u_1","name":"Ada","verbose":true}}
}

func ExampleHandle_validationFailure() {
	type createUserInput struct {
		Name string `json:"name"`
	}

	rt := chix.New()
	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", chix.Handle(rt, chix.Operation[createUserInput, struct{}]{
		Method: http.MethodPost,
		Validate: func(_ context.Context, input *createUserInput) []chix.Violation {
			if input.Name == "" {
				return []chix.Violation{{
					Source:  "body",
					Field:   "name",
					Code:    "required",
					Message: "name is required",
				}}
			}
			return nil
		},
	}, func(_ context.Context, _ *createUserInput) (*struct{}, error) {
		return &struct{}{}, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 422
	// {"error":{"code":"invalid_request","message":"invalid request","details":[{"source":"body","field":"name","code":"required","message":"name is required"}]}}
}

func ExampleHandle_errorMapper() {
	type createUserInput struct {
		Name string `json:"name"`
	}

	type createUserOutput struct {
		Name string `json:"name"`
	}

	var errUserExists = errors.New("user exists")

	rt := chix.New(chix.WithErrorMapper(func(err error) *chix.HTTPError {
		if errors.Is(err, errUserExists) {
			return &chix.HTTPError{
				Status:  http.StatusConflict,
				Code:    "user_exists",
				Message: "user already exists",
			}
		}
		return nil
	}))

	router := chi.NewRouter()
	router.Method(http.MethodPost, "/users", chix.Handle(rt, chix.Operation[createUserInput, createUserOutput]{
		Method: http.MethodPost,
	}, func(_ context.Context, input *createUserInput) (*createUserOutput, error) {
		if input.Name == "taken" {
			return nil, errUserExists
		}
		return &createUserOutput{Name: input.Name}, nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"taken"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 409
	// {"error":{"code":"user_exists","message":"user already exists","details":[]}}
}
