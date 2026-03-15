package main

import (
	"github.com/kanata996/chix"
	"github.com/kanata996/chix/reqx"
	"github.com/kanata996/chix/resp"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type createItemRequest struct {
	Name string `json:"name" validate:"required"`
}

func main() {
	r := chi.NewRouter()
	r.Get("/items/{uuid}", getItem)
	r.Post("/items", createItem)

	_ = http.ListenAndServe(":8080", r)
}

func getItem(w http.ResponseWriter, r *http.Request) {
	itemUUID, err := chix.Path(r).UUID("uuid")
	if err != nil {
		resp.Problem(w, r, err)
		return
	}

	resp.Success(w, map[string]any{
		"uuid": itemUUID,
	})
}

func createItem(w http.ResponseWriter, r *http.Request) {
	var body createItemRequest
	if err := reqx.DecodeJSON(w, r, &body); err != nil {
		resp.Problem(w, r, err)
		return
	}
	if err := reqx.ValidateBody(&body); err != nil {
		resp.Problem(w, r, err)
		return
	}

	resp.Created(w, map[string]any{
		"name": body.Name,
	})
}
