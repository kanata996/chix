package main

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/middleware"
)

var (
	errInviteExpired = errors.New("invite expired")
	errUserConflict  = errors.New("user conflict")
	errUserNotFound  = errors.New("user not found")
)

type createUserRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type patchProfileRequest struct {
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
}

type refreshSessionRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type listUsersQuery struct {
	Page  *int   `query:"page"`
	Limit *int   `query:"limit"`
	Role  string `query:"role"`
}

type exportReportQuery struct {
	Format string `query:"format"`
}

// main 启动一个可直接运行的 chi 示例服务。
func main() {
	log.Fatal(http.ListenAndServe(":8080", newRouter()))
}

// newRouter 组装示例路由，集中展示 chix 在 chi 中的推荐接法。
func newRouter() http.Handler {
	mapUserError := func(err error) *chix.Error {
		if errors.Is(err, errUserNotFound) {
			return chix.DomainError(http.StatusNotFound, "user_not_found", "user not found")
		}
		return nil
	}
	mapInviteError := func(err error) *chix.Error {
		if errors.Is(err, errInviteExpired) {
			return chix.DomainError(http.StatusGone, "invite_expired", "invite has expired")
		}
		return nil
	}
	mapConflictError := func(err error) *chix.Error {
		if errors.Is(err, errUserConflict) {
			return chix.DomainError(http.StatusConflict, "user_conflict", "user already exists")
		}
		return nil
	}

	handlerOptions := []chix.Option{
		chix.WithErrorMapper(chix.ChainMappers(mapUserError, mapInviteError)),
		chix.WithErrorMappers(mapConflictError),
	}

	wrap := func(next chix.Handler) http.HandlerFunc {
		return chix.Wrap(next, handlerOptions...)
	}
	writeError := func(w http.ResponseWriter, r *http.Request, err error) {
		chix.WriteError(w, r, err, handlerOptions...)
	}

	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				writeError(w, r, chix.RequestError(
					http.StatusUnauthorized,
					"unauthorized",
					"missing authorization token",
				))
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer())
	r.Use(authMiddleware)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, chix.RequestError(http.StatusNotFound, "route_not_found", "route not found"))
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, chix.RequestError(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"))
	})

	r.Get("/users", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query listUsersQuery
		if err := chix.DecodeAndValidateQuery(r, &query, validateListUsersQuery); err != nil {
			return err
		}

		page := 1
		if query.Page != nil {
			page = *query.Page
		}
		limit := 20
		if query.Limit != nil {
			limit = *query.Limit
		}
		role := strings.TrimSpace(query.Role)
		if role == "" {
			role = "member"
		}

		users := []map[string]any{
			{"id": "u_1", "role": role},
			{"id": "u_2", "role": role},
		}
		if limit == 1 {
			users = users[:1]
		}

		return chix.WriteMeta(w, http.StatusOK, users, map[string]any{
			"page":  page,
			"limit": limit,
			"count": len(users),
		})
	}))

	r.Get("/users/{userID}", wrap(func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "userID")
		if id == "missing" {
			return errUserNotFound
		}

		return chix.Write(w, http.StatusOK, map[string]any{
			"id":   id,
			"name": "alice",
		})
	}))

	r.Post("/users", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req createUserRequest
		if err := chix.DecodeAndValidateJSON(r, &req, validateCreateUserRequest, chix.WithMaxBodyBytes(128)); err != nil {
			return err
		}

		name := strings.TrimSpace(req.Name)
		if strings.EqualFold(name, "taken") {
			return errUserConflict
		}

		role := strings.TrimSpace(req.Role)
		if role == "" {
			role = "member"
		}

		return chix.Write(w, http.StatusCreated, map[string]any{
			"id":   "u_new",
			"name": name,
			"role": role,
		})
	}))

	r.Patch("/users/{userID}/profile", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req patchProfileRequest
		if err := chix.DecodeJSON(r, &req, chix.AllowUnknownFields()); err != nil {
			return err
		}

		req.DisplayName = strings.TrimSpace(req.DisplayName)
		req.Timezone = strings.TrimSpace(req.Timezone)

		if err := chix.Validate(&req, validatePatchProfileRequest); err != nil {
			return err
		}

		return chix.Write(w, http.StatusOK, map[string]any{
			"id":           chi.URLParam(r, "userID"),
			"display_name": req.DisplayName,
			"timezone":     req.Timezone,
		})
	}))

	r.Post("/sessions/refresh", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var req refreshSessionRequest
		if err := chix.DecodeJSON(r, &req, chix.AllowEmptyBody()); err != nil {
			return err
		}

		token := strings.TrimSpace(req.RefreshToken)
		if token == "" {
			token = "cookie_refresh_token"
		}

		return chix.Write(w, http.StatusOK, map[string]any{
			"session_id":    "s_new",
			"refresh_token": token,
		})
	}))

	r.Get("/reports/export", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query exportReportQuery
		if err := chix.DecodeQuery(r, &query, chix.AllowUnknownQueryFields()); err != nil {
			return err
		}

		format := strings.TrimSpace(query.Format)
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "csv" {
			return chix.RequestError(
				http.StatusNotAcceptable,
				"unsupported_format",
				"format must be json or csv",
				map[string]any{"field": "format", "allowed": []string{"json", "csv"}},
			)
		}

		return chix.Write(w, http.StatusOK, map[string]any{
			"download_url": "/downloads/report." + format,
		})
	}))

	r.Post("/invites/{code}/accept", wrap(func(w http.ResponseWriter, r *http.Request) error {
		if chi.URLParam(r, "code") == "expired" {
			return errInviteExpired
		}
		return chix.WriteEmpty(w, http.StatusNoContent)
	}))

	r.Post("/users/{userID}/suspend", wrap(func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "userID")
		if id == "u_suspended" {
			return chix.DomainError(
				http.StatusConflict,
				"user_already_suspended",
				"user already suspended",
				map[string]any{"user_id": id},
			)
		}
		return chix.WriteEmpty(w, http.StatusNoContent)
	}))

	r.Get("/internal/upstream", wrap(func(w http.ResponseWriter, r *http.Request) error {
		return chix.InternalError(
			http.StatusBadGateway,
			"upstream_unavailable",
			"upstream unavailable",
		)
	}))

	r.Get("/internal/unmapped", wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("storage dependency failed")
	}))

	r.Get("/partial", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("partial")); err != nil {
			panic(err)
		}
		writeError(w, r, chix.RequestError(http.StatusUnauthorized, "unauthorized", "unauthorized"))
	})

	r.Get("/panic", wrap(func(w http.ResponseWriter, r *http.Request) error {
		panic("boom")
	}))

	return r
}

// validateCreateUserRequest 展示 JSON body 解码后的业务前置校验。
func validateCreateUserRequest(value *createUserRequest) []chix.Violation {
	var violations []chix.Violation

	if strings.TrimSpace(value.Name) == "" {
		violations = append(violations, chix.Violation{
			Field:   "name",
			Code:    "required",
			Message: "is required",
		})
	}

	role := strings.TrimSpace(value.Role)
	if role != "" && role != "member" && role != "admin" {
		violations = append(violations, chix.Violation{
			Field:   "role",
			Code:    "one_of",
			Message: "must be member or admin",
		})
	}

	return violations
}

// validateListUsersQuery 展示 query 参数解码后的边界校验。
func validateListUsersQuery(value *listUsersQuery) []chix.Violation {
	var violations []chix.Violation

	if value.Page != nil && *value.Page < 1 {
		violations = append(violations, chix.Violation{
			Field:   "page",
			Code:    "min",
			Message: "must be at least 1",
		})
	}
	if value.Limit != nil && (*value.Limit < 1 || *value.Limit > 100) {
		violations = append(violations, chix.Violation{
			Field:   "limit",
			Code:    "range",
			Message: "must be between 1 and 100",
		})
	}

	role := strings.TrimSpace(value.Role)
	if role != "" && role != "member" && role != "admin" {
		violations = append(violations, chix.Violation{
			Field:   "role",
			Code:    "one_of",
			Message: "must be member or admin",
		})
	}

	return violations
}

// validatePatchProfileRequest 展示 DecodeJSON 与 Validate 分步使用时的校验方式。
func validatePatchProfileRequest(value *patchProfileRequest) []chix.Violation {
	var violations []chix.Violation

	if value.DisplayName == "" && value.Timezone == "" {
		violations = append(violations, chix.Violation{
			Field:   "profile",
			Code:    "required",
			Message: "display_name or timezone is required",
		})
	}
	if value.DisplayName != "" && len(value.DisplayName) < 3 {
		violations = append(violations, chix.Violation{
			Field:   "display_name",
			Code:    "min_length",
			Message: "must be at least 3 characters",
		})
	}

	return violations
}
