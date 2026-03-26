package openapi

import (
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kanata996/chix/internal/reqmeta"
)

type OperationConfig struct {
	Method             string
	Path               string
	OperationID        string
	Summary            string
	Description        string
	Tags               []string
	SuccessStatus      int
	SuccessDescription string
	Responses          []ResponseConfig
}

type ResponseConfig struct {
	Status      int
	Description string
	Headers     map[string]HeaderDoc
	ContentType string
	ModelType   reflect.Type
	NoBody      bool
}

type schemaKey struct {
	typ               reflect.Type
	includeValidation bool
}

type buildState struct {
	schemaNamer SchemaNamer
	names       map[schemaKey]string
	usedNames   map[string]schemaKey
}

func newBuildState(schemaNamer SchemaNamer) *buildState {
	return &buildState{
		schemaNamer: schemaNamer,
		names:       map[schemaKey]string{},
		usedNames:   map[string]schemaKey{},
	}
}

var timeType = reflect.TypeOf(time.Time{})

func NewOperationDoc[In any, Out any](doc *Document, operation OperationConfig, problemType reflect.Type) *OperationDoc {
	state := ensureBuildState(doc)
	inputType := typeOf[In]()
	outputType := typeOf[Out]()
	collector := schemaBuilder{
		doc:      doc,
		state:    state,
		visiting: map[schemaKey]bool{},
	}
	problemSchema := collector.responseSchemaFor(problemType)

	parameters := collector.parametersFor(inputType)
	requestSchema := collector.requestBodySchema(inputType)

	status := operation.SuccessStatus
	if status == 0 {
		status = defaultSuccessStatus(strings.ToUpper(operation.Method))
	}

	operationDoc := &OperationDoc{
		OperationID: operationID(operation),
		Summary:     operation.Summary,
		Description: operation.Description,
		Tags:        operation.Tags,
		Parameters:  parameters,
		Responses: map[string]ResponseDoc{
			"default": problemResponseDoc("Unexpected error", problemSchema),
		},
	}
	if !hasExplicitSuccessResponse(operation.Responses) {
		operationDoc.Responses[strconv.Itoa(status)] = successResponseDoc(operation.SuccessDescription, status, collector.responseSchemaFor(outputType))
	}

	if hasRequestDecodeFailures(inputType) {
		operationDoc.Responses["400"] = problemResponseDoc("Invalid request", problemSchema)
	}
	if hasValidationRules(inputType) {
		operationDoc.Responses["422"] = problemResponseDoc("Request validation failed", problemSchema)
	}
	for _, response := range operation.Responses {
		operationDoc.Responses[strconv.Itoa(response.Status)] = responseDoc(response, outputType, problemSchema, &collector)
	}

	if requestSchema != nil {
		_, bodyRequired := requestBodyInfo(inputType)
		operationDoc.RequestBody = &RequestBody{
			Required: bodyRequired,
			Content: map[string]MediaType{
				"application/json": {Schema: requestSchema},
			},
		}
	}

	return operationDoc
}

func successResponseDoc(description string, status int, schema *Schema) ResponseDoc {
	response := ResponseDoc{
		Description: successResponseDescription(description),
	}
	if responseStatusAllowsBody(status) {
		contentType := responseContentTypeForDoc("", "application/json")
		response.Content = map[string]MediaType{
			contentType: {Schema: schema},
		}
	}
	return response
}

func responseDoc(config ResponseConfig, outputType reflect.Type, problemSchema *Schema, builder *schemaBuilder) ResponseDoc {
	response := ResponseDoc{
		Description: explicitResponseDescription(config.Status, config.Description),
		Headers:     cloneHeaderDocs(config.Headers),
	}
	if config.NoBody || !responseStatusAllowsBody(config.Status) {
		return response
	}

	contentType := defaultResponseContentType(config.Status)
	schema := problemSchema
	if isSuccessStatus(config.Status) {
		schemaType := outputType
		if config.ModelType != nil {
			schemaType = config.ModelType
		}
		schema = builder.responseSchemaFor(schemaType)
	}
	contentType = responseContentTypeForDoc(config.ContentType, contentType)

	response.Content = map[string]MediaType{
		contentType: {Schema: schema},
	}
	return response
}

func successResponseDescription(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Successful response"
	}
	return value
}

func explicitResponseDescription(status int, value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if isSuccessStatus(status) {
		return "Successful response"
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return "Response"
}

func problemResponseDoc(description string, schema *Schema) ResponseDoc {
	contentType := responseContentTypeForDoc("", "application/problem+json")
	return ResponseDoc{
		Description: description,
		Content: map[string]MediaType{
			contentType: {Schema: schema},
		},
	}
}

func defaultResponseContentType(status int) string {
	if isSuccessStatus(status) {
		return "application/json"
	}
	return "application/problem+json"
}

func responseContentTypeForDoc(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err == nil && strings.TrimSpace(mediaType) != "" {
		return strings.ToLower(mediaType)
	}
	if index := strings.Index(value, ";"); index >= 0 {
		value = value[:index]
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func cloneHeaderDocs(headers map[string]HeaderDoc) map[string]HeaderDoc {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]HeaderDoc, len(headers))
	for name, header := range headers {
		cloned[name] = header
	}
	return cloned
}

func hasExplicitSuccessResponse(responses []ResponseConfig) bool {
	for _, response := range responses {
		if isSuccessStatus(response.Status) {
			return true
		}
	}
	return false
}

func responseStatusAllowsBody(status int) bool {
	if status >= 100 && status < 200 {
		return false
	}
	return status != http.StatusNoContent && status != http.StatusNotModified
}

func isSuccessStatus(status int) bool {
	return status >= 200 && status < 300
}

func operationID(operation OperationConfig) string {
	if strings.TrimSpace(operation.OperationID) != "" {
		return operation.OperationID
	}

	replacer := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return strings.Trim(replacer.ReplaceAllString(strings.ToLower(operation.Method+"_"+operation.Path), "_"), "_")
}

func defaultSuccessStatus(method string) int {
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

type schemaBuilder struct {
	doc      *Document
	state    *buildState
	visiting map[schemaKey]bool
}

func (b *schemaBuilder) parametersFor(t reflect.Type) []Parameter {
	t = reqmeta.IndirectType(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	var params []Parameter
	reqmeta.WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
		source, name, ok := reqmeta.ParameterSource(field)
		if !ok {
			return true
		}
		schema := b.requestSchemaFor(field.Type)
		applyValidationConstraints(schema, field)
		schema.Description = strings.TrimSpace(field.Tag.Get("doc"))
		params = append(params, Parameter{
			Name:        name,
			In:          source,
			Required:    source == "path" || parameterFieldRequired(field),
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
	t = reqmeta.IndirectType(t)
	if t.Kind() != reflect.Struct {
		return b.requestSchemaFor(t)
	}

	schema := b.structSchema(t, true, true)
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}
	return schema
}

func (b *schemaBuilder) requestSchemaFor(t reflect.Type) *Schema {
	return b.schemaFor(t, true)
}

func (b *schemaBuilder) responseSchemaFor(t reflect.Type) *Schema {
	return b.schemaFor(t, false)
}

func (b *schemaBuilder) schemaFor(t reflect.Type, includeValidation bool) *Schema {
	t = reqmeta.IndirectType(t)

	if t == timeType {
		return &Schema{Type: "string", Format: "date-time"}
	}

	if shouldUseComponentSchema(t) {
		return b.componentSchemaFor(t, includeValidation)
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
			Items: b.schemaFor(t.Elem(), includeValidation),
		}
	case reflect.Map:
		return &Schema{
			Type:                 "object",
			AdditionalProperties: b.schemaFor(t.Elem(), includeValidation),
		}
	case reflect.Struct:
		return b.structSchema(t, false, includeValidation)
	case reflect.Interface:
		return &Schema{}
	default:
		return &Schema{}
	}
}

func (b *schemaBuilder) structSchema(t reflect.Type, requestBodyRoot bool, includeValidation bool) *Schema {
	t = reqmeta.IndirectType(t)
	key := schemaKey{typ: t, includeValidation: includeValidation}

	if b.visiting[key] {
		return &Schema{Type: "object"}
	}
	b.visiting[key] = true
	defer delete(b.visiting, key)

	schema := &Schema{
		Type:       "object",
		Properties: map[string]*Schema{},
	}

	reqmeta.WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if requestBodyRoot && reqmeta.IsParameterField(field) {
			return true
		}

		name, omitempty, skip := reqmeta.JSONFieldName(field)
		if skip || name == "" {
			return true
		}

		property := b.schemaFor(field.Type, includeValidation)
		if desc := strings.TrimSpace(field.Tag.Get("doc")); desc != "" {
			property.Description = desc
		}
		if includeValidation {
			applyValidationConstraints(property, field)
		}
		if field.Type.Kind() == reflect.Pointer && !hasValidateRule(field, "required") {
			property.Nullable = true
		}

		schema.Properties[name] = property
		if fieldRequiredForSchema(field, omitempty, includeValidation) {
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

func shouldUseComponentSchema(t reflect.Type) bool {
	t = reqmeta.IndirectType(t)
	return t.Kind() == reflect.Struct && t != timeType && t.Name() != ""
}

func (b *schemaBuilder) componentSchemaFor(t reflect.Type, includeValidation bool) *Schema {
	t = reqmeta.IndirectType(t)
	key := schemaKey{typ: t, includeValidation: includeValidation}

	if name, ok := b.state.names[key]; ok {
		return refSchema(name)
	}

	name := b.allocateComponentName(key)
	b.state.names[key] = name
	b.doc.Components.Schemas[name] = &Schema{}

	schema := b.structSchema(t, false, includeValidation)
	*b.doc.Components.Schemas[name] = *schema

	return refSchema(name)
}

func refSchema(name string) *Schema {
	return &Schema{Ref: "#/components/schemas/" + name}
}

func (b *schemaBuilder) allocateComponentName(key schemaKey) string {
	base := b.state.baseComponentName(key)
	name := base
	index := 2

	for {
		if existing, ok := b.state.usedNames[name]; !ok || existing == key {
			b.state.usedNames[name] = key
			return name
		}
		name = fmt.Sprintf("%s%d", base, index)
		index++
	}
}

func (s *buildState) baseComponentName(key schemaKey) string {
	if s != nil && s.schemaNamer != nil {
		if name := sanitizeComponentName(s.schemaNamer(SchemaNameContext{
			Type:    key.typ,
			Request: key.includeValidation,
		})); name != "" {
			return name
		}
	}
	return defaultComponentName(key.typ, key.includeValidation)
}

func defaultComponentName(t reflect.Type, includeValidation bool) string {
	base := sanitizeComponentName(t.String())
	if name := sanitizeComponentName(t.Name()); name != "" {
		base = name
	}
	if includeValidation {
		base += "Request"
	}
	if base == "" {
		base = "Schema"
	}
	return base
}

func sanitizeComponentName(value string) string {
	replacer := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	value = replacer.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	return value
}

func typeOf[T any]() reflect.Type {
	var ptr *T
	return reflect.TypeOf(ptr).Elem()
}

func requestBodyInfo(t reflect.Type) (present bool, required bool) {
	t = reqmeta.IndirectType(t)
	if t.Kind() != reflect.Struct {
		return true, true
	}

	reqmeta.WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if reqmeta.IsParameterField(field) {
			return true
		}

		name, omitempty, skip := reqmeta.JSONFieldName(field)
		if skip || name == "" {
			return true
		}

		present = true
		if requestFieldRequired(field, omitempty) {
			required = true
		}
		return true
	})

	return present, required
}

func hasRequestDecodeFailures(t reflect.Type) bool {
	t = reqmeta.IndirectType(t)
	if t.Kind() != reflect.Struct {
		return true
	}

	if bodyPresent, _ := requestBodyInfo(t); bodyPresent {
		return true
	}

	hasParameters := false
	reqmeta.WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
		if field.PkgPath != "" && !field.Anonymous {
			return true
		}
		if reqmeta.IsParameterField(field) {
			hasParameters = true
			return false
		}
		return true
	})

	return hasParameters
}

func fieldRequiredForSchema(field reflect.StructField, omitempty bool, includeValidation bool) bool {
	if !includeValidation {
		return !omitempty && field.Type.Kind() != reflect.Pointer
	}
	return requestFieldRequired(field, omitempty)
}

func requestFieldRequired(field reflect.StructField, omitempty bool) bool {
	if field.Tag.Get("required") == "true" || hasValidateRule(field, "required") {
		return true
	}
	return !omitempty && field.Type.Kind() != reflect.Pointer
}

func parameterFieldRequired(field reflect.StructField) bool {
	return field.Tag.Get("required") == "true" || hasValidateRule(field, "required")
}

func applyValidationConstraints(schema *Schema, field reflect.StructField) {
	for _, rule := range parseValidateRules(field.Tag.Get("validate")) {
		applyValidationRule(schema, field.Type, rule.name, rule.param)
	}
}

func applyValidationRule(schema *Schema, fieldType reflect.Type, name, param string) {
	fieldType = reqmeta.IndirectType(fieldType)

	switch name {
	case "required", "omitempty":
		return
	case "email":
		if fieldType.Kind() == reflect.String {
			schema.Format = "email"
		}
	case "uuid", "uuid4":
		if fieldType.Kind() == reflect.String {
			schema.Format = "uuid"
		}
	case "oneof":
		values := parseEnumValues(param, fieldType)
		if len(values) > 0 {
			schema.Enum = values
		}
	case "min":
		applyMinConstraint(schema, fieldType, param)
	case "max":
		applyMaxConstraint(schema, fieldType, param)
	case "len":
		applyLenConstraint(schema, fieldType, param)
	}
}

func applyMinConstraint(schema *Schema, fieldType reflect.Type, param string) {
	switch fieldType.Kind() {
	case reflect.String:
		if value, ok := parseIntConstraint(param); ok {
			schema.MinLength = &value
		}
	case reflect.Slice, reflect.Array:
		if value, ok := parseIntConstraint(param); ok {
			schema.MinItems = &value
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if value, ok := parseFloatConstraint(param); ok {
			schema.Minimum = &value
		}
	}
}

func applyMaxConstraint(schema *Schema, fieldType reflect.Type, param string) {
	switch fieldType.Kind() {
	case reflect.String:
		if value, ok := parseIntConstraint(param); ok {
			schema.MaxLength = &value
		}
	case reflect.Slice, reflect.Array:
		if value, ok := parseIntConstraint(param); ok {
			schema.MaxItems = &value
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if value, ok := parseFloatConstraint(param); ok {
			schema.Maximum = &value
		}
	}
}

func applyLenConstraint(schema *Schema, fieldType reflect.Type, param string) {
	switch fieldType.Kind() {
	case reflect.String:
		if value, ok := parseIntConstraint(param); ok {
			schema.MinLength = &value
			schema.MaxLength = &value
		}
	case reflect.Slice, reflect.Array:
		if value, ok := parseIntConstraint(param); ok {
			schema.MinItems = &value
			schema.MaxItems = &value
		}
	}
}

type validateRule struct {
	name  string
	param string
}

func parseValidateRules(tag string) []validateRule {
	if strings.TrimSpace(tag) == "" {
		return nil
	}

	parts := strings.Split(tag, ",")
	rules := make([]validateRule, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, param, _ := strings.Cut(part, "=")
		rules = append(rules, validateRule{
			name:  strings.TrimSpace(name),
			param: strings.TrimSpace(param),
		})
	}
	return rules
}

func hasValidateRule(field reflect.StructField, name string) bool {
	for _, rule := range parseValidateRules(field.Tag.Get("validate")) {
		if rule.name == name {
			return true
		}
	}
	return false
}

func hasValidationRules(t reflect.Type) bool {
	visited := map[reflect.Type]bool{}
	return hasValidationRulesRecursive(reqmeta.IndirectType(t), visited)
}

func hasValidationRulesRecursive(t reflect.Type, visited map[reflect.Type]bool) bool {
	t = reqmeta.IndirectType(t)
	if t == nil {
		return false
	}
	if visited[t] {
		return false
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.Struct:
		found := false
		reqmeta.WalkStructFields(t, func(field reflect.StructField, _ []int) bool {
			if field.PkgPath != "" && !field.Anonymous {
				return true
			}
			if strings.TrimSpace(field.Tag.Get("validate")) != "" {
				found = true
				return false
			}
			if hasValidationRulesRecursive(field.Type, visited) {
				found = true
				return false
			}
			return true
		})
		return found
	case reflect.Slice, reflect.Array, reflect.Pointer:
		return hasValidationRulesRecursive(t.Elem(), visited)
	case reflect.Map:
		return hasValidationRulesRecursive(t.Elem(), visited)
	default:
		return false
	}
}

func parseEnumValues(param string, fieldType reflect.Type) []any {
	rawValues := strings.Fields(param)
	if len(rawValues) == 0 {
		return nil
	}

	values := make([]any, 0, len(rawValues))
	for _, raw := range rawValues {
		value, ok := parseEnumValue(raw, fieldType)
		if !ok {
			return nil
		}
		values = append(values, value)
	}
	return values
}

func parseEnumValue(raw string, fieldType reflect.Type) (any, bool) {
	switch fieldType.Kind() {
	case reflect.String:
		return raw, true
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		return value, err == nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, fieldType.Bits())
		return value, err == nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, fieldType.Bits())
		return value, err == nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, fieldType.Bits())
		return value, err == nil
	default:
		return nil, false
	}
}

func parseIntConstraint(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func parseFloatConstraint(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}
