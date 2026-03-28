package validatorx

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/internal/inputschema"
)

func Adapter[I any](v *validator.Validate) chix.Validator[I] {
	if v == nil {
		v = validator.New(validator.WithRequiredStructEnabled())
	}

	inputType := reflect.TypeOf((*I)(nil)).Elem()
	schema, err := inputschema.Load(inputType)
	if err != nil {
		panic(err)
	}

	return func(_ context.Context, input *I) []chix.Violation {
		if input == nil {
			return nil
		}

		err := v.Struct(input)
		if err == nil {
			return nil
		}

		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			return violations(schema, validationErrs)
		}

		var invalidErr *validator.InvalidValidationError
		if errors.As(err, &invalidErr) {
			panic(invalidErr)
		}

		panic(err)
	}
}

func violations(schema *inputschema.Schema, validationErrs validator.ValidationErrors) []chix.Violation {
	result := make([]chix.Violation, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		location, ok := inputschema.Location{}, false
		if schema != nil {
			location, ok = schema.LookupLocation(fieldErr.StructNamespace())
		}
		if !ok {
			location = inputschema.Location{
				Source: "body",
				Field:  fieldErr.Field(),
			}
		}

		result = append(result, chix.Violation{
			Source:  location.Source,
			Field:   location.Field,
			Code:    fieldErr.Tag(),
			Message: message(location.Field, fieldErr),
		})
	}
	return result
}

func message(field string, fieldErr validator.FieldError) string {
	if field == "" {
		field = fieldErr.Field()
	}
	if param := fieldErr.Param(); param != "" {
		return fmt.Sprintf("%s failed %s=%s validation", field, fieldErr.Tag(), param)
	}
	return fmt.Sprintf("%s failed %s validation", field, fieldErr.Tag())
}
