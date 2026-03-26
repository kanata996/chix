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
	return internalopenapi.NewOperationDoc[In, Out](doc, internalopenapi.OperationConfig{
		Method:             method,
		Path:               operation.Path,
		OperationID:        operation.OperationID,
		Summary:            operation.Summary,
		Description:        operation.Description,
		Tags:               operation.Tags,
		SuccessStatus:      successStatus(method, operation.SuccessStatus),
		SuccessDescription: operation.SuccessDescription,
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
