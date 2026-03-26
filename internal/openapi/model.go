package openapi

import (
	"net/http"
	"reflect"
	"strings"
)

type Config struct {
	Title       string
	Version     string
	Description string
	SchemaNamer SchemaNamer
}

type SchemaNameContext struct {
	Type    reflect.Type
	Request bool
}

type SchemaNamer func(SchemaNameContext) string

type Document struct {
	OpenAPI    string          `json:"openapi"`
	Info       Info            `json:"info"`
	Paths      map[string]Path `json:"paths"`
	Components *Components     `json:"components,omitempty"`

	state *buildState `json:"-"`
}

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type Path struct {
	Get    *OperationDoc `json:"get,omitempty"`
	Post   *OperationDoc `json:"post,omitempty"`
	Put    *OperationDoc `json:"put,omitempty"`
	Patch  *OperationDoc `json:"patch,omitempty"`
	Delete *OperationDoc `json:"delete,omitempty"`
}

type OperationDoc struct {
	OperationID string                 `json:"operationId,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Parameters  []Parameter            `json:"parameters,omitempty"`
	RequestBody *RequestBody           `json:"requestBody,omitempty"`
	Responses   map[string]ResponseDoc `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

type ResponseDoc struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas,omitempty"`
}

type Schema struct {
	Ref                  string             `json:"$ref,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	MaxItems             *int               `json:"maxItems,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
}

func NewDocument(config Config) *Document {
	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "chix API"
	}

	version := strings.TrimSpace(config.Version)
	if version == "" {
		version = "0.1.0"
	}

	return &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       title,
			Version:     version,
			Description: strings.TrimSpace(config.Description),
		},
		Paths: map[string]Path{},
		state: newBuildState(config.SchemaNamer),
	}
}

func AddOperation(doc *Document, method, path string, operation *OperationDoc) {
	item := doc.Paths[path]
	switch strings.ToUpper(method) {
	case http.MethodGet:
		item.Get = operation
	case http.MethodPost:
		item.Post = operation
	case http.MethodPut:
		item.Put = operation
	case http.MethodPatch:
		item.Patch = operation
	case http.MethodDelete:
		item.Delete = operation
	}
	doc.Paths[path] = item
}

func ensureBuildState(doc *Document) *buildState {
	if doc.state == nil {
		doc.state = newBuildState(nil)
	}
	if doc.Components == nil {
		doc.Components = &Components{}
	}
	if doc.Components.Schemas == nil {
		doc.Components.Schemas = map[string]*Schema{}
	}
	return doc.state
}
