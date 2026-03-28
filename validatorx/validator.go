package validatorx

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	playgroundvalidator "github.com/go-playground/validator/v10"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/internal/binding"
)

func Adapter[I any](v *playgroundvalidator.Validate) chix.Validator[I] {
	if v == nil {
		v = playgroundvalidator.New(playgroundvalidator.WithRequiredStructEnabled())
	}

	inputType := reflect.TypeOf((*I)(nil)).Elem()

	return func(_ context.Context, input *I) []chix.Violation {
		if input == nil {
			return nil
		}

		err := v.Struct(input)
		if err == nil {
			return nil
		}

		var validationErrs playgroundvalidator.ValidationErrors
		if errors.As(err, &validationErrs) {
			return violations(inputType, validationErrs)
		}

		var invalidErr *playgroundvalidator.InvalidValidationError
		if errors.As(err, &invalidErr) {
			panic(invalidErr)
		}

		panic(err)
	}
}

func violations(inputType reflect.Type, validationErrs playgroundvalidator.ValidationErrors) []chix.Violation {
	result := make([]chix.Violation, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		location, ok := binding.LookupFieldLocation(inputType, fieldErr.StructNamespace())
		if !ok {
			location = binding.FieldLocation{
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

func message(field string, fieldErr playgroundvalidator.FieldError) string {
	if field == "" {
		field = fieldErr.Field()
	}
	if param := fieldErr.Param(); param != "" {
		return fmt.Sprintf("%s failed %s=%s validation", field, fieldErr.Tag(), param)
	}
	return fmt.Sprintf("%s failed %s validation", field, fieldErr.Tag())
}
