package resp

import (
	"fmt"
	"github.com/kanata996/chix/errx"
	"github.com/kanata996/chix/reqx"
	"net/http"
	"net/http/httptest"
	"strings"
)

func ExampleSuccess() {
	rec := httptest.NewRecorder()
	Success(rec, map[string]string{"id": "1"})

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 200
	// {"code":0,"data":{"id":"1"}}
}

func ExampleProblem() {
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	rec := httptest.NewRecorder()

	Problem(rec, req, reqx.BadRequest(reqx.Required(reqx.InQuery, "limit")))

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 400
	// {"code":400000,"message":"Bad Request","details":[{"in":"query","field":"limit","code":"required"}]}
}

func ExampleError() {
	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, errx.ErrUnauthorized, nil)

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 401
	// {"code":400001,"message":"Unauthorized"}
}
