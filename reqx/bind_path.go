package reqx

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func BindPathValues(r *http.Request, target any) error {
	if r == nil {
		return errorsf("request must not be nil")
	}
	if target == nil {
		return errorsf("destination must not be nil")
	}
	return bindPathValuesDefault(r, target)
}

func pathValuesForPlan(r *http.Request, plan *valueDecodePlan) url.Values {
	values := url.Values{}
	if r == nil || plan == nil {
		return values
	}

	for _, fieldSpec := range plan.fields {
		rawValues, ok := pathParamValues(r, fieldSpec.name)
		if !ok {
			continue
		}
		values[fieldSpec.name] = rawValues
	}

	return values
}

func pathParamValues(r *http.Request, name string) ([]string, bool) {
	if r == nil {
		return nil, false
	}

	value := strings.TrimSpace(r.PathValue(name))
	if value != "" {
		return []string{value}, true
	}
	if !pathWildcardExists(r.Pattern, name) {
		return nil, false
	}
	return []string{value}, true
}

func pathWildcardExists(pattern, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	for _, wildcard := range pathWildcardNames(pattern) {
		if wildcard == name {
			return true
		}
	}

	return false
}

func pathWildcardNames(pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}

	names := make([]string, 0, 2)
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}

		end := strings.IndexByte(pattern[i+1:], '}')
		if end < 0 {
			break
		}

		token := strings.TrimSpace(pattern[i+1 : i+1+end])
		token = strings.TrimSuffix(token, "...")
		token, _, _ = strings.Cut(token, ":")
		token = strings.TrimSpace(token)
		if token != "" && token != "$" {
			names = append(names, token)
		}

		i += end + 1
	}

	return names
}

func requiredPathParamValues(r *http.Request, name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("reqx: path param name must not be empty")
	}

	rawValues, ok := pathParamValues(r, name)
	if !ok || len(rawValues) == 0 || (len(rawValues) == 1 && rawValues[0] == "") {
		return nil, invalidFieldError(newViolation(name, ViolationInPath, ViolationCodeRequired, violationDetailRequired))
	}

	return rawValues, nil
}
