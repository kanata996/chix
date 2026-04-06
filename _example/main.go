package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/kanata996/chix"
	chixmw "github.com/kanata996/chix/middleware"
	"github.com/kanata996/chix/resp"
)

type createAccountRequest struct {
	OrgID string `param:"org_id"`
	Name  string `json:"name" validate:"required,min=3,max=64"`
}

func (r *createAccountRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
}

type account struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type accountStore struct {
	mu        sync.RWMutex
	nextID    int
	accounts  map[string]account
	nameIndex map[string]string
}

func newAccountStore() *accountStore {
	return &accountStore{
		accounts:  make(map[string]account),
		nameIndex: make(map[string]string),
	}
}

func (s *accountStore) Create(orgID, name string) (account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexKey := orgID + ":" + strings.ToLower(name)
	if existingID, exists := s.nameIndex[indexKey]; exists {
		return account{}, resp.Conflict("account_name_conflict", fmt.Sprintf("account %q already exists in org %q", name, orgID), map[string]any{
			"field":  "name",
			"in":     "body",
			"code":   "already_exists",
			"detail": fmt.Sprintf("account %q already exists", name),
			"id":     existingID,
		})
	}

	s.nextID++
	acct := account{
		ID:        fmt.Sprintf("acct_%06d", s.nextID),
		OrgID:     orgID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	s.accounts[acct.ID] = acct
	s.nameIndex[indexKey] = acct.ID

	return acct, nil
}

func (s *accountStore) Get(orgID, accountID string) (account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acct, ok := s.accounts[accountID]
	if !ok || acct.OrgID != orgID {
		return account{}, false
	}

	return acct, true
}

func newRouter(logger *slog.Logger, store *accountStore) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             httplog.SchemaECS,
		RecoverPanics:      true,
		LogRequestHeaders:  []string{"Content-Type", "Origin"},
		LogResponseHeaders: []string{"Content-Type"},
	}))
	r.Use(chixmw.RequestLogAttrs())

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = chix.NoContent(w, r)
	})

	r.Post("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := chix.BindAndValidate(r, &req, chix.WithMaxBodyBytes(1<<20)); err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		acct, err := store.Create(req.OrgID, req.Name)
		if err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		_ = chix.Created(w, r, acct)
	})

	r.Get("/orgs/{org_id}/accounts/{account_id}", func(w http.ResponseWriter, r *http.Request) {
		orgID, err := chix.ParamString(r, "org_id")
		if err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		accountID, err := chix.ParamString(r, "account_id")
		if err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		acct, ok := store.Get(orgID, accountID)
		if !ok {
			_ = chix.WriteError(w, r, resp.NotFound("account_not_found", "account not found"))
			return
		}

		_ = chix.OK(w, r, acct)
	})

	return r
}

func newLogger() *slog.Logger {
	logFormat := httplog.SchemaECS.Concise(false)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: logFormat.ReplaceAttr,
	})

	return slog.New(handler).With(
		slog.String("app", "chix-example"),
		slog.String("version", "dev"),
		slog.String("env", "local"),
	)
}

func main() {
	logger := newLogger()
	slog.SetDefault(logger)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           newRouter(logger, newAccountStore()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("server starting", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited", slog.Any("error", err))
		os.Exit(1)
	}
}
