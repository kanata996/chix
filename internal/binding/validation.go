package binding

import (
	"errors"
	"fmt"

	validator "github.com/go-playground/validator/v10"

	"github.com/kanata996/chix/internal/schema"
)

type validationDetail struct {
	Source  string `json:"source"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var defaultValidator = validator.New(validator.WithRequiredStructEnabled())

func validateStruct(input any, schema *schema.Schema) error {
	if input == nil {
		return nil
	}
	if schema != nil && !schema.HasValidation {
		return nil
	}

	err := defaultValidator.Struct(input)
	if err == nil {
		return nil
	}

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		return newInvalidRequestError(validationDetails(schema, validationErrs))
	}

	var invalidErr *validator.InvalidValidationError
	if errors.As(err, &invalidErr) {
		panic(invalidErr)
	}

	panic(err)
}

func validationDetails(schema *schema.Schema, validationErrs validator.ValidationErrors) []any {
	result := make([]any, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		location, ok := struct {
			Source string
			Field  string
		}{}, false
		if schema != nil {
			mapped, mappedOK := schema.LookupLocation(fieldErr.StructNamespace())
			location = struct {
				Source string
				Field  string
			}{
				Source: mapped.Source,
				Field:  mapped.Field,
			}
			ok = mappedOK
		}
		if !ok {
			location = struct {
				Source string
				Field  string
			}{
				Source: "body",
				Field:  fieldErr.Field(),
			}
		}

		result = append(result, validationDetail{
			Source:  location.Source,
			Field:   location.Field,
			Code:    fieldErr.Tag(),
			Message: validationMessage(location.Field, fieldErr),
		})
	}
	return result
}

func validationMessage(field string, fieldErr validator.FieldError) string {
	if field == "" {
		field = fieldErr.Field()
	}
	if param := fieldErr.Param(); param != "" {
		return fmt.Sprintf("%s failed %s=%s validation", field, fieldErr.Tag(), param)
	}
	return fmt.Sprintf("%s failed %s validation", field, fieldErr.Tag())
}
