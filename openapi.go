package chix

import (
	"reflect"
	"strings"

	internalopenapi "github.com/kanata996/chix/internal/openapi"
)

type Document = internalopenapi.Document
type Info = internalopenapi.Info
type Path = internalopenapi.Path
type OperationDoc = internalopenapi.OperationDoc
type Parameter = internalopenapi.Parameter
type RequestBody = internalopenapi.RequestBody
type ResponseDoc = internalopenapi.ResponseDoc
type HeaderDoc = internalopenapi.HeaderDoc
type MediaType = internalopenapi.MediaType
type Components = internalopenapi.Components
type Schema = internalopenapi.Schema

type OpenAPISchemaNameContext struct {
	Type    reflect.Type
	Request bool
}

type OpenAPISchemaNamer func(OpenAPISchemaNameContext) string

func newDocument(config Config) *Document {
	return internalopenapi.NewDocument(internalopenapi.Config{
		Title:       config.Title,
		Version:     config.Version,
		Description: config.Description,
		SchemaNamer: adaptOpenAPISchemaNamer(config.OpenAPISchemaNamer),
	})
}

func newOperationDoc[In any, Out any](doc *Document, operation Operation) *OperationDoc {
	method := strings.ToUpper(operation.Method)
	responses := make([]internalopenapi.ResponseConfig, 0, len(operation.Responses))
	for _, response := range operation.Responses {
		responses = append(responses, internalopenapi.ResponseConfig{
			Status:      response.Status,
			Description: response.Description,
			Headers:     response.Headers,
			NoBody:      response.NoBody,
		})
	}

	return internalopenapi.NewOperationDoc[In, Out](doc, internalopenapi.OperationConfig{
		Method:             method,
		Path:               operation.Path,
		OperationID:        operation.OperationID,
		Summary:            operation.Summary,
		Description:        operation.Description,
		Tags:               operation.Tags,
		SuccessStatus:      successStatus(method, operation.SuccessStatus, operation.Responses),
		SuccessDescription: operation.SuccessDescription,
		Responses:          responses,
	}, reflect.TypeOf(Problem{}))
}

func addOperation(doc *Document, method, path string, operation *OperationDoc) {
	internalopenapi.AddOperation(doc, method, path, operation)
}

func adaptOpenAPISchemaNamer(namer OpenAPISchemaNamer) internalopenapi.SchemaNamer {
	if namer == nil {
		return nil
	}

	return func(ctx internalopenapi.SchemaNameContext) string {
		return namer(OpenAPISchemaNameContext{
			Type:    ctx.Type,
			Request: ctx.Request,
		})
	}
}
