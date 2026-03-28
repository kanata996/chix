package binding

import (
	"fmt"
	"reflect"
	"strings"
)

type FieldLocation struct {
	Source string
	Field  string
}

func LookupFieldLocation(t reflect.Type, structNamespace string) (FieldLocation, bool) {
	t = indirectType(t)
	if t.Kind() != reflect.Struct {
		return FieldLocation{}, false
	}

	parts := strings.Split(structNamespace, ".")
	if len(parts) <= 1 {
		return FieldLocation{}, false
	}

	location, ok := lookupFieldLocation(t, parts[1:], "", nil)
	if !ok || location.Source == "" || location.Field == "" {
		return FieldLocation{}, false
	}
	return location, true
}

func lookupFieldLocation(t reflect.Type, parts []string, source string, path []string) (FieldLocation, bool) {
	field, ok := findStructFieldByName(t, parts[0])
	if !ok {
		return FieldLocation{}, false
	}

	nextSource := source
	nextPath := append([]string(nil), path...)

	if nextSource == "" {
		if candidateSource, candidateName, ok, err := parameterSource(field); err == nil && ok {
			nextSource = candidateSource
			nextPath = append(nextPath, candidateName)
		} else if candidateName, ok := bodyFieldName(field); ok {
			nextSource = "body"
			nextPath = append(nextPath, candidateName)
		}
	} else {
		nextPath = append(nextPath, fieldLocationName(field, nextSource))
	}

	if len(parts) == 1 {
		return FieldLocation{
			Source: nextSource,
			Field:  strings.Join(nextPath, "."),
		}, true
	}

	nextType := indirectType(field.Type)
	if nextType.Kind() != reflect.Struct {
		return FieldLocation{
			Source: nextSource,
			Field:  strings.Join(nextPath, "."),
		}, true
	}

	return lookupFieldLocation(nextType, parts[1:], nextSource, nextPath)
}

func fieldLocationName(field reflect.StructField, source string) string {
	switch source {
	case "path", "query":
		if candidateSource, candidateName, ok, err := parameterSource(field); err == nil && ok && candidateSource == source {
			return candidateName
		}
	case "body":
		if candidateName, ok := bodyFieldName(field); ok {
			return candidateName
		}
	}
	return field.Name
}

func findStructFieldByName(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Name == name {
			return field, true
		}
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := indirectType(field.Type)
		anonymousStruct := field.Anonymous && field.Tag == "" && fieldType.Kind() == reflect.Struct
		if !anonymousStruct {
			continue
		}

		if nested, ok := findStructFieldByName(fieldType, name); ok {
			return nested, true
		}
	}

	return reflect.StructField{}, false
}

func parameterSource(field reflect.StructField) (source string, name string, ok bool, err error) {
	var matches []string

	for _, candidate := range []string{"path", "query"} {
		taggedName := tagName(field.Tag.Get(candidate))
		if taggedName != "" {
			source = candidate
			name = taggedName
			matches = append(matches, candidate)
		}
	}

	if len(matches) > 1 {
		return "", "", false, fmt.Errorf("chix: field %q declares multiple parameter sources", field.Name)
	}
	if len(matches) == 0 {
		return "", "", false, nil
	}

	return source, name, true, nil
}

func explicitBodyTag(field reflect.StructField) bool {
	_, ok := bodyFieldName(field)
	return ok
}

func bodyFieldName(field reflect.StructField) (string, bool) {
	tag, ok := field.Tag.Lookup("json")
	if !ok || tag == "-" {
		return "", false
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	return name, true
}

func tagName(tag string) string {
	if tag == "" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}
