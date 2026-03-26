package chix

import (
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Document struct {
	OpenAPI string          `json:"openapi"`
	Info    Info            `json:"info"`
	Paths   map[string]Path `json:"paths"`
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

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
}

func newDocument(config Config) *Document {
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
	}
}

func (d *Document) addOperation(method, path string, operation *OperationDoc) {
	item := d.Paths[path]
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
	d.Paths[path] = item
}

func newOperationDoc[In any, Out any](operation Operation) *OperationDoc {
	inputType := typeOf[In]()
	outputType := typeOf[Out]()
	collector := schemaBuilder{visiting: map[reflect.Type]bool{}}

	parameters := collector.parametersFor(inputType)
	requestSchema := collector.requestBodySchema(inputType)

	status := successStatus(strings.ToUpper(operation.Method), operation.SuccessStatus)
	response := ResponseDoc{
		Description: responseDescription(operation.SuccessDescription),
	}
	if status != http.StatusNoContent {
		response.Content = map[string]MediaType{
			"application/json": {Schema: collector.schemaFor(outputType)},
		}
	}

	doc := &OperationDoc{
		OperationID: operationID(operation),
		Summary:     operation.Summary,
		Description: operation.Description,
		Tags:        operation.Tags,
		Parameters:  parameters,
		Responses: map[string]ResponseDoc{
			strconv.Itoa(status): response,
			"default": {
				Description: "Unexpected error",
				Content: map[string]MediaType{
					"application/problem+json": {Schema: collector.schemaFor(reflect.TypeOf(Problem{}))},
				},
			},
		},
	}

	if requestSchema != nil {
		_, bodyRequired := bodyInfo(inputType)
		doc.RequestBody = &RequestBody{
			Required: bodyRequired,
			Content: map[string]MediaType{
				"application/json": {Schema: requestSchema},
			},
		}
	}

	return doc
}

func responseDescription(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Successful response"
	}
	return value
}

func operationID(operation Operation) string {
	if strings.TrimSpace(operation.OperationID) != "" {
		return operation.OperationID
	}

	replacer := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return strings.Trim(replacer.ReplaceAllString(strings.ToLower(operation.Method+"_"+operation.Path), "_"), "_")
}

type schemaBuilder struct {
	visiting map[reflect.Type]bool
}

func (b *schemaBuilder) parametersFor(t reflect.Type) []Parameter {
	t = indirectType(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	var params []Parameter
	walkStructFields(t, func(field reflect.StructField, _ []int) bool {
		source, name, ok := parameterSource(field)
		if !ok {
			return true
		}
		schema := b.schemaFor(field.Type)
		schema.Description = strings.TrimSpace(field.Tag.Get("doc"))
		params = append(params, Parameter{
			Name:        name,
			In:          source,
			Required:    source == "path" || field.Tag.Get("required") == "true",
			Description: strings.TrimSpace(field.Tag.Get("doc")),
			Schema:      schema,
		})
		return true
	})

	sort.Slice(params, func(i, j int) bool {
		if params[i].In == params[j].In {
			return params[i].Name < params[j].Name
		}
		return params[i].In < params[j].In
	})

	if len(params) == 0 {
		return nil
	}
	return params
}

func (b *schemaBuilder) requestBodySchema(t reflect.Type) *Schema {
	t = indirectType(t)
	if t.Kind() != reflect.Struct {
		return b.schemaFor(t)
	}

	schema := b.structSchema(t, true)
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}
	return schema
}

func (b *schemaBuilder) schemaFor(t reflect.Type) *Schema {
	t = indirectType(t)

	if t == timeType {
		return &Schema{Type: "string", Format: "date-time"}
	}

	switch t.Kind() {
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		return &Schema{
			Type:  "array",
			Items: b.schemaFor(t.Elem()),
		}
	case reflect.Map:
		return &Schema{
			Type:                 "object",
			AdditionalProperties: b.schemaFor(t.Elem()),
		}
	case reflect.Struct:
		return b.structSchema(t, false)
	case reflect.Interface:
		return &Schema{}
	default:
		return &Schema{}
	}
}

func (b *schemaBuilder) structSchema(t reflect.Type, requestBodyRoot bool) *Schema {
	original := t
	t = indirectType(t)

	if b.visiting[original] {
		return &Schema{Type: "object"}
	}
	b.visiting[original] = true
	defer delete(b.visiting, original)

	schema := &Schema{
		Type:       "object",
		Properties: map[string]*Schema{},
	}

	walkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if requestBodyRoot && isParameterField(field) {
			return true
		}

		name, omitempty, skip := jsonFieldName(field)
		if skip || name == "" {
			return true
		}

		property := b.schemaFor(field.Type)
		if desc := strings.TrimSpace(field.Tag.Get("doc")); desc != "" {
			property.Description = desc
		}
		if field.Type.Kind() == reflect.Pointer {
			property.Nullable = true
		}

		schema.Properties[name] = property
		if !omitempty && field.Type.Kind() != reflect.Pointer {
			schema.Required = append(schema.Required, name)
		}
		return true
	})

	sort.Strings(schema.Required)
	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	return schema
}

func typeOf[T any]() reflect.Type {
	var ptr *T
	return reflect.TypeOf(ptr).Elem()
}
