// Copyright 2025 oapi-codegen contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package openapi provides an abstraction layer for OpenAPI document processing
// that supports both OpenAPI 3.0 and 3.1 specifications via libopenapi.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// Global variables to store component schemas for reference restoration
var globalComponentSchemas map[string]*base.Schema
var globalComponentSchemaNames map[*base.Schema]string

// nullHandler is a slog handler that discards all log messages
type nullHandler struct{}

func (h *nullHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (h *nullHandler) Handle(context.Context, slog.Record) error { return nil }
func (h *nullHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return h }
func (h *nullHandler) WithGroup(name string) slog.Handler        { return h }

// T represents an OpenAPI document, providing compatibility with kin-openapi's openapi3.T
type T struct {
	*v3.Document
	version           string
	Paths             *Paths
	Components        *Components
	OpenAPI           string               // OpenAPI version string for compatibility
	Webhooks          map[string]*PathItem // OpenAPI 3.1 webhooks support
	JSONSchemaDialect string               // OpenAPI 3.1 $schema support
	Info              *Info                // Enhanced Info object with OpenAPI 3.1 features
	Servers           []*Server            // Enhanced Server objects with OpenAPI 3.1 features
}

// Schema provides compatibility wrapper for OpenAPI schemas
type Schema struct {
	*base.Schema
	// Extensions for kin-openapi compatibility
	Nullable             bool
	Example              interface{}
	Examples             []interface{}
	Extensions           map[string]interface{}
	AdditionalProperties AdditionalPropertiesItem
	Discriminator        *Discriminator
	// Additional fields for compatibility
	Items *SchemaRef
	AnyOf []*SchemaRef
	OneOf []*SchemaRef

	// JSON Schema Draft 2020-12 keywords
	Const                 interface{}
	If                    *SchemaRef
	Then                  *SchemaRef
	Else                  *SchemaRef
	PatternProperties     map[string]*SchemaRef
	UnevaluatedItems      *SchemaRef
	UnevaluatedProperties *SchemaRef
	Contains              *SchemaRef
	PrefixItems           []*SchemaRef
	DependentRequired     map[string][]string
	DependentSchemas      map[string]*SchemaRef
}

// Enum returns the enum values as a slice of interfaces instead of YAML nodes
func (s *Schema) Enum() []interface{} {
	if s == nil || s.Schema == nil {
		return nil
	}

	// Extract actual values from YAML nodes
	if s.Schema.Enum == nil {
		return nil
	}

	result := make([]interface{}, len(s.Schema.Enum))
	for i, node := range s.Schema.Enum {
		if node != nil {
			// Decode the YAML node value
			var value interface{}
			if err := node.Decode(&value); err == nil {
				result[i] = value
			} else {
				// Fallback to string representation if decode fails
				result[i] = node.Value
			}
		}
	}
	return result
}

// SchemaRef provides reference wrapper for schemas
type SchemaRef struct {
	Ref        string
	Value      *Schema
	Extensions map[string]interface{}
	// Additional fields for compatibility
	Items                *SchemaRef
	AdditionalProperties AdditionalPropertiesItem
	OneOf                []*SchemaRef
	AnyOf                []*SchemaRef
	AllOf                []*SchemaRef
	Not                  *SchemaRef
}

// Parameter represents an OpenAPI parameter
type Parameter struct {
	*v3.Parameter
	Extensions map[string]interface{}
	Schema     *SchemaRef
	Examples   map[string]*ExampleRef
	Content    map[string]*MediaType
}

// WrapParameter creates a Parameter wrapper
func WrapParameter(param *v3.Parameter) *Parameter {
	if param == nil {
		return nil
	}

	wrapped := &Parameter{
		Parameter: param,
	}

	// Convert Extensions from ordered map to regular map
	if param.Extensions != nil {
		wrapped.Extensions = make(map[string]interface{})
		for pair := param.Extensions.First(); pair != nil; pair = pair.Next() {
			wrapped.Extensions[pair.Key()] = pair.Value()
		}
	}

	// Convert Schema
	if param.Schema != nil {
		wrapped.Schema = SchemaProxyToRef(param.Schema)
	}

	// Convert Examples
	if param.Examples != nil {
		wrapped.Examples = make(map[string]*ExampleRef)
		for pair := param.Examples.First(); pair != nil; pair = pair.Next() {
			wrapped.Examples[pair.Key()] = &ExampleRef{
				Value: &Example{Example: pair.Value()},
			}
		}
	}

	// Convert Content
	if param.Content != nil {
		wrapped.Content = ContentToMap(param.Content)
	}

	return wrapped
}

// IsRequired returns whether the parameter is required (handles pointer to bool)
func (p *Parameter) IsRequired() bool {
	if p.Required == nil {
		return false
	}
	return *p.Required
}

// ParameterRef provides reference wrapper for parameters
type ParameterRef struct {
	Ref   string
	Value *Parameter
}

// Operation represents an OpenAPI operation
type Operation struct {
	*v3.Operation
	Responses   *Responses
	RequestBody *RequestBodyRef
	Callbacks   map[string]*CallbackRef
	OperationID string // For compatibility with kin-openapi
}

// WrapOperation creates an Operation wrapper
func WrapOperation(operation *v3.Operation) *Operation {
	if operation == nil {
		return nil
	}

	wrapped := &Operation{
		Operation: operation,
	}

	// Set OperationID for compatibility
	if operation.OperationId != "" {
		wrapped.OperationID = operation.OperationId
	}

	// Wrap responses
	if operation.Responses != nil {
		wrapped.Responses = &Responses{responses: operation.Responses}
	}

	// Wrap request body
	if operation.RequestBody != nil {
		wrapped.RequestBody = &RequestBodyRef{
			Value: WrapRequestBody(operation.RequestBody),
		}
	}

	// Wrap callbacks
	if operation.Callbacks != nil {
		wrapped.Callbacks = make(map[string]*CallbackRef)
		for pair := operation.Callbacks.First(); pair != nil; pair = pair.Next() {
			wrapped.Callbacks[pair.Key()] = &CallbackRef{
				Value: &Callback{Callback: pair.Value()},
			}
		}
	}

	return wrapped
}

// Responses represents OpenAPI responses
type Responses struct {
	responses *v3.Responses
}

// Map returns the responses as a map
func (r *Responses) Map() map[string]*ResponseRef {
	if r == nil || r.responses == nil || r.responses.Codes == nil {
		return nil
	}

	result := make(map[string]*ResponseRef)
	for pair := r.responses.Codes.First(); pair != nil; pair = pair.Next() {
		code := pair.Key()
		response := pair.Value()
		result[code] = &ResponseRef{
			Value: WrapResponse(response),
		}
	}
	return result
}

// Value returns a response by status code
func (r *Responses) Value(code string) *ResponseRef {
	if r.responses == nil || r.responses.Codes == nil {
		return nil
	}

	for pair := r.responses.Codes.First(); pair != nil; pair = pair.Next() {
		if pair.Key() == code {
			return &ResponseRef{
				Value: WrapResponse(pair.Value()),
			}
		}
	}
	return nil
}

// Response represents an OpenAPI response
type Response struct {
	*v3.Response
	Content    map[string]*MediaType
	Headers    map[string]*HeaderRef
	Links      map[string]*LinkRef
	Extensions map[string]interface{}
}

// WrapResponse creates a Response wrapper with converted Content
func WrapResponse(response *v3.Response) *Response {
	if response == nil {
		return nil
	}

	wrapped := &Response{
		Response: response,
		Content:  ContentToMap(response.Content),
	}

	// Convert Headers
	if response.Headers != nil {
		wrapped.Headers = make(map[string]*HeaderRef)
		for pair := response.Headers.First(); pair != nil; pair = pair.Next() {
			wrapped.Headers[pair.Key()] = &HeaderRef{
				Value: WrapHeader(pair.Value()),
			}
		}
	}

	// Convert Links
	if response.Links != nil {
		wrapped.Links = make(map[string]*LinkRef)
		for pair := response.Links.First(); pair != nil; pair = pair.Next() {
			wrapped.Links[pair.Key()] = &LinkRef{
				Value: &Link{Link: pair.Value()},
			}
		}
	}

	// Convert Extensions
	if response.Extensions != nil {
		wrapped.Extensions = make(map[string]interface{})
		for pair := response.Extensions.First(); pair != nil; pair = pair.Next() {
			wrapped.Extensions[pair.Key()] = pair.Value()
		}
	}

	return wrapped
}

// ResponseRef provides reference wrapper for responses
type ResponseRef struct {
	Ref   string
	Value *Response
}

// RequestBody represents an OpenAPI request body
type RequestBody struct {
	*v3.RequestBody
	Content    map[string]*MediaType
	Extensions map[string]interface{}
}

// WrapRequestBody creates a RequestBody wrapper with converted Content
func WrapRequestBody(requestBody *v3.RequestBody) *RequestBody {
	if requestBody == nil {
		return nil
	}

	wrapped := &RequestBody{
		RequestBody: requestBody,
		Content:     ContentToMap(requestBody.Content),
	}

	// Convert Extensions
	if requestBody.Extensions != nil {
		wrapped.Extensions = make(map[string]interface{})
		for pair := requestBody.Extensions.First(); pair != nil; pair = pair.Next() {
			wrapped.Extensions[pair.Key()] = pair.Value()
		}
	}

	return wrapped
}

// IsRequired returns whether the request body is required (handles pointer to bool)
func (rb *RequestBody) IsRequired() bool {
	if rb.Required == nil {
		return false
	}
	return *rb.Required
}

// RequestBodyRef provides reference wrapper for request bodies
type RequestBodyRef struct {
	Ref   string
	Value *RequestBody
}

// MediaType represents an OpenAPI media type
type MediaType struct {
	*v3.MediaType
	Schema   *SchemaRef
	Encoding map[string]*Encoding
	Examples map[string]*ExampleRef
}

// Encoding represents media type encoding
type Encoding struct {
	*v3.Encoding
}

// WrapMediaType creates a MediaType wrapper
func WrapMediaType(mediaType *v3.MediaType) *MediaType {
	if mediaType == nil {
		return nil
	}

	wrapped := &MediaType{
		MediaType: mediaType,
	}

	// Convert Schema
	if mediaType.Schema != nil {
		wrapped.Schema = SchemaProxyToRef(mediaType.Schema)
	}

	// Convert Encoding
	if mediaType.Encoding != nil {
		wrapped.Encoding = make(map[string]*Encoding)
		for pair := mediaType.Encoding.First(); pair != nil; pair = pair.Next() {
			wrapped.Encoding[pair.Key()] = &Encoding{Encoding: pair.Value()}
		}
	}

	// Convert Examples
	if mediaType.Examples != nil {
		wrapped.Examples = make(map[string]*ExampleRef)
		for pair := mediaType.Examples.First(); pair != nil; pair = pair.Next() {
			wrapped.Examples[pair.Key()] = &ExampleRef{
				Value: &Example{Example: pair.Value()},
			}
		}
	}

	return wrapped
}

// Components represents OpenAPI components
type Components struct {
	Schemas         map[string]*SchemaRef
	Parameters      map[string]*ParameterRef
	Responses       map[string]*ResponseRef
	RequestBodies   map[string]*RequestBodyRef
	Headers         map[string]*HeaderRef
	SecuritySchemes map[string]*SecuritySchemeRef
	Examples        map[string]*ExampleRef
	Callbacks       map[string]*CallbackRef
	Links           map[string]*LinkRef
	PathItems       map[string]*PathItemRef // OpenAPI 3.1 adds pathItems to components
}

// PathItemRef provides reference wrapper for path items in components
type PathItemRef struct {
	Ref   string
	Value *PathItem
}

// HeaderRef provides reference wrapper for headers
type HeaderRef struct {
	Ref   string
	Value *Header
}

// Header represents an OpenAPI header
type Header struct {
	*v3.Header
	Schema *SchemaRef
}

// WrapHeader creates a Header wrapper
func WrapHeader(header *v3.Header) *Header {
	if header == nil {
		return nil
	}

	wrapped := &Header{
		Header: header,
	}

	// Convert Schema
	if header.Schema != nil {
		wrapped.Schema = SchemaProxyToRef(header.Schema)
	}

	return wrapped
}

// SecuritySchemeRef provides reference wrapper for security schemes
type SecuritySchemeRef struct {
	Ref   string
	Value *SecurityScheme
}

// SecurityScheme represents an OpenAPI security scheme
type SecurityScheme struct {
	*v3.SecurityScheme
}

// ExampleRef provides reference wrapper for examples
type ExampleRef struct {
	Ref   string
	Value *Example
}

// Example represents an OpenAPI example
type Example struct {
	*base.Example
}

// CallbackRef provides reference wrapper for callbacks
type CallbackRef struct {
	Ref   string
	Value *Callback
}

// Callback represents an OpenAPI callback
type Callback struct {
	*v3.Callback
}

// Map returns the callback expressions as a map
func (c *Callback) Map() map[string]*PathItem {
	if c.Callback == nil || c.Expression == nil {
		return nil
	}

	result := make(map[string]*PathItem)
	for pair := c.Expression.First(); pair != nil; pair = pair.Next() {
		result[pair.Key()] = WrapPathItem(pair.Value())
	}
	return result
}

// LinkRef provides reference wrapper for links
type LinkRef struct {
	Ref   string
	Value *Link
}

// Link represents an OpenAPI link
type Link struct {
	*v3.Link
}

// Discriminator represents an OpenAPI discriminator
type Discriminator struct {
	*base.Discriminator
}

// MappingToMap converts the discriminator mapping to a regular map
func (d *Discriminator) MappingToMap() map[string]string {
	if d.Discriminator == nil || d.Mapping == nil {
		return nil
	}

	result := make(map[string]string)
	for pair := d.Mapping.First(); pair != nil; pair = pair.Next() {
		result[pair.Key()] = pair.Value()
	}
	return result
}

// Paths represents OpenAPI paths
type Paths struct {
	paths *v3.Paths
}

// Map returns the paths as a map
func (p *Paths) Map() map[string]*PathItem {
	if p.paths == nil {
		return nil
	}

	result := make(map[string]*PathItem)
	for pathPairs := p.paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		pathName := pathPairs.Key()
		pathItem := pathPairs.Value()
		result[pathName] = WrapPathItem(pathItem)
	}
	return result
}

// Value returns a path item by path
func (p *Paths) Value(path string) *PathItem {
	if p.paths == nil {
		return nil
	}

	// Iterate through path items to find the matching path
	for pathPairs := p.paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		if pathPairs.Key() == path {
			return WrapPathItem(pathPairs.Value())
		}
	}
	return nil
}

// Find returns a path item by path (alias for Value for kin-openapi compatibility)
func (p *Paths) Find(path string) *PathItem {
	return p.Value(path)
}

// PathItem represents an OpenAPI path item
type PathItem struct {
	*v3.PathItem
	Parameters []*ParameterRef
	Ref        string
}

// WrapPathItem creates a PathItem wrapper
func WrapPathItem(pathItem *v3.PathItem) *PathItem {
	if pathItem == nil {
		return nil
	}

	wrapped := &PathItem{
		PathItem: pathItem,
	}

	// Convert Parameters to ParameterRef slice
	if pathItem.Parameters != nil {
		wrapped.Parameters = ParametersToRefSlice(pathItem.Parameters)
	}

	return wrapped
}

// SetOperation sets an operation on the path item
func (pi *PathItem) SetOperation(method string, operation *Operation) {
	if pi.PathItem == nil {
		return
	}

	var op *v3.Operation
	if operation != nil {
		op = operation.Operation
	}

	// Convert to lowercase for case-insensitive matching
	method = strings.ToLower(method)

	switch method {
	case "get":
		pi.Get = op
	case "post":
		pi.Post = op
	case "put":
		pi.Put = op
	case "delete":
		pi.Delete = op
	case "options":
		pi.Options = op
	case "head":
		pi.Head = op
	case "patch":
		pi.Patch = op
	case "trace":
		pi.Trace = op
	}
}

// Operations returns all operations for this path item
func (pi *PathItem) Operations() map[string]*Operation {
	if pi.PathItem == nil {
		return nil
	}

	operations := make(map[string]*Operation)
	if pi.Get != nil {
		operations["GET"] = WrapOperation(pi.Get)
	}
	if pi.Post != nil {
		operations["POST"] = WrapOperation(pi.Post)
	}
	if pi.Put != nil {
		operations["PUT"] = WrapOperation(pi.Put)
	}
	if pi.Delete != nil {
		operations["DELETE"] = WrapOperation(pi.Delete)
	}
	if pi.Options != nil {
		operations["OPTIONS"] = WrapOperation(pi.Options)
	}
	if pi.Head != nil {
		operations["HEAD"] = WrapOperation(pi.Head)
	}
	if pi.Patch != nil {
		operations["PATCH"] = WrapOperation(pi.Patch)
	}
	if pi.Trace != nil {
		operations["TRACE"] = WrapOperation(pi.Trace)
	}

	return operations
}

// GetOperation returns an operation by HTTP method
func (pi *PathItem) GetOperation(method string) *Operation {
	if pi.PathItem == nil {
		return nil
	}

	switch strings.ToUpper(method) {
	case "GET":
		if pi.Get != nil {
			return WrapOperation(pi.Get)
		}
	case "POST":
		if pi.Post != nil {
			return WrapOperation(pi.Post)
		}
	case "PUT":
		if pi.Put != nil {
			return WrapOperation(pi.Put)
		}
	case "DELETE":
		if pi.Delete != nil {
			return WrapOperation(pi.Delete)
		}
	case "OPTIONS":
		if pi.Options != nil {
			return WrapOperation(pi.Options)
		}
	case "HEAD":
		if pi.Head != nil {
			return WrapOperation(pi.Head)
		}
	case "PATCH":
		if pi.Patch != nil {
			return WrapOperation(pi.Patch)
		}
	case "TRACE":
		if pi.Trace != nil {
			return WrapOperation(pi.Trace)
		}
	}
	return nil
}

// Loader provides document loading functionality
type Loader struct {
	IsExternalRefsAllowed bool
	IgnoreMissingRefs     bool
}

// NewLoader creates a new OpenAPI document loader
func NewLoader() *Loader {
	return &Loader{
		IsExternalRefsAllowed: true,
	}
}

// LoadFromFile loads an OpenAPI document from a file
func (l *Loader) LoadFromFile(filePath string) (*T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Extract directory for base path resolution and make it absolute
	basePath := ""
	if idx := strings.LastIndex(filePath, "/"); idx != -1 {
		basePath = filePath[:idx]
	} else if idx := strings.LastIndex(filePath, "\\"); idx != -1 {
		basePath = filePath[:idx]
	}

	// Convert to absolute path if we have a base path
	if basePath != "" {
		if absBasePath, err := filepath.Abs(basePath); err == nil {
			basePath = absBasePath
		}
	}

	return l.LoadFromDataWithBasePath(data, basePath)
}

// LoadFromURI loads an OpenAPI document from a URI
func (l *Loader) LoadFromURI(uri *url.URL) (*T, error) {
	// Use HTTP client to fetch the content
	client := &http.Client{}
	resp, err := client.Get(uri.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URI %s: %w", uri.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URI %s: status %d", uri.String(), resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", uri.String(), err)
	}

	return l.LoadFromData(data)
}

// LoadFromData loads an OpenAPI document from byte data
func (l *Loader) LoadFromData(data []byte) (*T, error) {
	return l.LoadFromDataWithBasePath(data, "")
}

// LoadFromDataWithBasePath loads an OpenAPI document from byte data with a base path for resolving references
func (l *Loader) LoadFromDataWithBasePath(data []byte, basePath string) (*T, error) {
	// If IgnoreMissingRefs is enabled, preprocess the data to remove problematic example references
	if l.IgnoreMissingRefs {
		var err error
		data, err = l.preprocessDataForMissingRefs(data)
		if err != nil {
			return nil, fmt.Errorf("failed to preprocess data: %w", err)
		}
	}

	// Create libopenapi document configuration
	config := &datamodel.DocumentConfiguration{
		AllowFileReferences:   l.IsExternalRefsAllowed,
		AllowRemoteReferences: l.IsExternalRefsAllowed,
	}

	// If IgnoreMissingRefs is enabled, configure a null logger to suppress file not found errors
	if l.IgnoreMissingRefs {
		config.Logger = slog.New(&nullHandler{})
	}

	// Set base path for local file references
	if basePath != "" {
		// Convert to absolute path for better resolution
		if absPath, err := filepath.Abs(basePath); err == nil {
			config.BasePath = absPath
		} else {
			config.BasePath = basePath
		}
	} else {
		// No base path set, so no base path for local file references
	}

	document, err := libopenapi.NewDocumentWithConfiguration(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Build V3 model - this will handle reference resolution
	docModel, errs := document.BuildV3Model()
	if docModel == nil {
		errMsg := "document model is nil"
		if len(errs) > 0 {
			errMsg = fmt.Sprintf("document model is nil, errors: %v", errs)
		}
		return nil, fmt.Errorf("failed to build document model: %s", errMsg)
	}
	if len(errs) > 0 && !l.IgnoreMissingRefs {
		// For now, continue with warnings but log them
		// In the future we might want to make this configurable
	}

	return l.wrapDocument(&docModel.Model), nil
}

// LoadFromDataWithPath loads an OpenAPI document from byte data with a path context
func (l *Loader) LoadFromDataWithPath(data []byte, path *url.URL) (*T, error) {
	if path != nil {
		return l.LoadFromDataWithBasePath(data, path.String())
	}
	return l.LoadFromData(data)
}

// wrapDocument creates a wrapped OpenAPI document from a libopenapi model
func (l *Loader) wrapDocument(model *v3.Document) *T {
	// Get OpenAPI version
	version := "3.0"
	if model.Version != "" {
		version = model.Version
	}

	// Create wrapper
	doc := &T{
		Document: model,
		version:  version,
		OpenAPI:  version,
	}

	// Handle JSONSchemaDialect for OpenAPI 3.1
	if model.JsonSchemaDialect != "" {
		doc.JSONSchemaDialect = model.JsonSchemaDialect
	}

	// Wrap Info with OpenAPI 3.1 enhancements
	if model.Info != nil {
		doc.Info = WrapInfo(model.Info)
	}

	// Wrap Servers with OpenAPI 3.1 enhancements
	if model.Servers != nil {
		doc.Servers = make([]*Server, len(model.Servers))
		for i, server := range model.Servers {
			doc.Servers[i] = WrapServer(server)
		}
	}

	// Wrap paths if they exist (paths is optional in OpenAPI 3.1)
	if model.Paths != nil {
		doc.Paths = &Paths{paths: model.Paths}
	} else {
		// Even if paths is nil, create an empty wrapper for compatibility
		doc.Paths = &Paths{paths: nil}
	}

	// Wrap components if they exist
	if model.Components != nil {
		doc.Components = l.wrapComponents(model.Components)
	}

	// Wrap webhooks if they exist (OpenAPI 3.1 feature)
	if model.Webhooks != nil {
		doc.Webhooks = make(map[string]*PathItem)
		for pair := model.Webhooks.First(); pair != nil; pair = pair.Next() {
			doc.Webhooks[pair.Key()] = WrapPathItem(pair.Value())
		}
	}

	return doc
}

// preprocessDataForMissingRefs removes problematic example references from OpenAPI data
func (l *Loader) preprocessDataForMissingRefs(data []byte) ([]byte, error) {
	// Try to parse as YAML first, then JSON
	var specData map[string]interface{}
	var err error
	var isYAML bool

	// Try JSON first
	err = json.Unmarshal(data, &specData)
	if err != nil {
		// Try YAML
		err = yaml.Unmarshal(data, &specData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI data as JSON or YAML: %w", err)
		}
		isYAML = true
	}

	// Remove all problematic example references throughout the entire spec
	l.removeExampleReferences(specData)

	// Convert back to original format
	if isYAML {
		return yaml.Marshal(specData)
	}
	return json.Marshal(specData)
}

// removeExampleReferences recursively removes all $ref references to missing example files
func (l *Loader) removeExampleReferences(data interface{}) {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this object contains a $ref to a missing external file
		if ref, ok := v["$ref"].(string); ok {
			if strings.HasPrefix(ref, "examples/") ||
				strings.HasPrefix(ref, "markdown/") ||
				strings.Contains(ref, ".json") ||
				strings.Contains(ref, ".md") ||
				strings.Contains(ref, ".py") ||
				strings.Contains(ref, ".ts") ||
				strings.Contains(ref, ".java") ||
				strings.Contains(ref, ".rb") ||
				strings.Contains(ref, ".php") ||
				strings.Contains(ref, ".cs") ||
				strings.Contains(ref, ".sh") {
				// Replace with placeholder
				delete(v, "$ref")
				v["placeholder"] = "External reference not available"
				return
			}
		}
		// Recursively process all values in the map
		for _, value := range v {
			l.removeExampleReferences(value)
		}
	case []interface{}:
		// Recursively process all items in the array
		for _, item := range v {
			l.removeExampleReferences(item)
		}
	}
}

// wrapComponents converts libopenapi components to our wrapper format
func (l *Loader) wrapComponents(components *v3.Components) *Components {
	wrapped := &Components{}

	if components.Schemas != nil {
		wrapped.Schemas = make(map[string]*SchemaRef)
		visited := make(map[*base.Schema]bool)

		// First pass: collect all component schemas for reference matching
		componentSchemas := make(map[string]*base.Schema)
		componentSchemaNames := make(map[*base.Schema]string) // Map schema pointer to component name
		for pair := components.Schemas.First(); pair != nil; pair = pair.Next() {
			schemaName := pair.Key()
			schemaProxy := pair.Value()
			if schemaProxy != nil && schemaProxy.Schema() != nil {
				schema := schemaProxy.Schema()
				componentSchemas[schemaName] = schema
				componentSchemaNames[schema] = schemaName
			}
		}

		// Store component schemas globally for reference restoration
		globalComponentSchemas = componentSchemas
		globalComponentSchemaNames = componentSchemaNames

		for pair := components.Schemas.First(); pair != nil; pair = pair.Next() {
			schemaName := pair.Key()
			schemaProxy := pair.Value()

			schemaRef := SchemaProxyToRefWithVisited(schemaProxy, visited)
			if schemaRef != nil {
				wrapped.Schemas[schemaName] = schemaRef
			}
		}
	}

	// TODO: Wrap other component types as needed
	if components.Parameters != nil {
		wrapped.Parameters = make(map[string]*ParameterRef)
		// Add parameter wrapping logic
	}

	if components.Responses != nil {
		wrapped.Responses = make(map[string]*ResponseRef)
		for pair := components.Responses.First(); pair != nil; pair = pair.Next() {
			responseName := pair.Key()
			responseProxy := pair.Value()

			wrapped.Responses[responseName] = &ResponseRef{
				Value: WrapResponse(responseProxy),
			}
		}
	}

	if components.RequestBodies != nil {
		wrapped.RequestBodies = make(map[string]*RequestBodyRef)
		// Add request body wrapping logic
	}

	// Handle pathItems (OpenAPI 3.1 feature)
	if components.PathItems != nil {
		wrapped.PathItems = make(map[string]*PathItemRef)
		for pair := components.PathItems.First(); pair != nil; pair = pair.Next() {
			pathItemName := pair.Key()
			pathItem := pair.Value()

			wrapped.PathItems[pathItemName] = &PathItemRef{
				Value: WrapPathItem(pathItem),
			}
		}
	}

	return wrapped
}

// GetVersion returns the OpenAPI version of the document
func (t *T) GetVersion() string {
	return t.version
}

// IsOpenAPI31 returns true if this is an OpenAPI 3.1 document
func (t *T) IsOpenAPI31() bool {
	return t.version == "3.1.0" || t.version == "3.1"
}

// IsOpenAPI30 returns true if this is an OpenAPI 3.0 document
func (t *T) IsOpenAPI30() bool {
	return t.version == "3.0.0" || t.version == "3.0.1" || t.version == "3.0.2" || t.version == "3.0.3" || t.version == "3.0"
}

// InternalizeRefs placeholder method for compatibility
func (t *T) InternalizeRefs(ctx interface{}, options interface{}) {
	// TODO: Implement internalization if needed by libopenapi
	// For now, this is a no-op as libopenapi handles references differently
}

// MarshalJSON marshals the document to JSON
func (t *T) MarshalJSON() ([]byte, error) {
	if t.Document == nil {
		return []byte("{}"), nil
	}

	// Use libopenapi's Render method which returns YAML and then convert to JSON
	yamlBytes, err := t.Render()
	if err != nil {
		return nil, err
	}

	// Convert YAML to JSON using standard yaml.v3 library
	var yamlData interface{}
	if err := yaml.Unmarshal(yamlBytes, &yamlData); err != nil {
		return nil, err
	}

	return json.Marshal(yamlData)
}

// WrapSchema converts a libopenapi schema to our wrapper
func WrapSchema(schema *base.Schema) *Schema {
	return WrapSchemaWithVisited(schema, make(map[*base.Schema]bool))
}

// WrapSchemaWithVisited converts a libopenapi schema to our wrapper with circular reference protection
func WrapSchemaWithVisited(schema *base.Schema, visited map[*base.Schema]bool) *Schema {
	if schema == nil {
		return nil
	}

	// Check for circular reference - if we're already processing this schema, return a minimal reference
	if visited[schema] {
		// Return a minimal schema reference to break the cycle
		return &Schema{
			Schema: schema,
		}
	}

	// Mark this schema as being processed
	visited[schema] = true

	wrapped := &Schema{
		Schema: schema,
	}

	// Handle OpenAPI 3.1 vs 3.0 compatibility
	if len(schema.Type) > 0 {
		// Check for nullable in union types (OpenAPI 3.1)
		for _, t := range schema.Type {
			if t == "null" {
				wrapped.Nullable = true
				break
			}
		}
	}

	// Handle traditional nullable field (OpenAPI 3.0 style)
	if schema.Nullable != nil && *schema.Nullable {
		wrapped.Nullable = true
	}

	// Handle examples (OpenAPI 3.1 prefers examples array over singular example)
	if schema.Examples != nil && len(schema.Examples) > 0 {
		wrapped.Examples = make([]interface{}, len(schema.Examples))
		for i, example := range schema.Examples {
			wrapped.Examples[i] = example.Value
		}
		// Set the first example as the singular example for backward compatibility
		if len(wrapped.Examples) > 0 {
			wrapped.Example = wrapped.Examples[0]
		}
	}

	// Handle legacy singular example (deprecated in OpenAPI 3.1, but still supported)
	if schema.Example != nil && wrapped.Examples == nil {
		wrapped.Example = schema.Example.Value
		// Create examples array with the singular example for OpenAPI 3.1 compatibility
		wrapped.Examples = []interface{}{schema.Example.Value}
	}

	// Handle Items for array types
	if schema.Items != nil && schema.Items.A != nil {
		itemsRef := SchemaProxyToRefWithVisited(schema.Items.A, visited)
		if itemsRef != nil {
			wrapped.Items = itemsRef
		}
	}

	// Handle AdditionalProperties
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.A != nil {
		additionalPropsRef := SchemaProxyToRefWithVisited(schema.AdditionalProperties.A, visited)
		if additionalPropsRef != nil {
			wrapped.AdditionalProperties = AdditionalPropertiesItem{
				Has:    true,
				Schema: additionalPropsRef,
			}
		}
	} else if schema.AdditionalProperties != nil {
		// Handle boolean case
		wrapped.AdditionalProperties = AdditionalPropertiesItem{
			Has:    schema.AdditionalProperties.B,
			Schema: nil,
		}
	}

	// Handle AnyOf
	if schema.AnyOf != nil {
		wrapped.AnyOf = make([]*SchemaRef, 0, len(schema.AnyOf))
		for _, proxy := range schema.AnyOf {
			anyOfRef := SchemaProxyToRefWithVisited(proxy, visited)
			if anyOfRef != nil {
				wrapped.AnyOf = append(wrapped.AnyOf, anyOfRef)
			}
		}
	}

	// Handle OneOf
	if schema.OneOf != nil {
		wrapped.OneOf = make([]*SchemaRef, 0, len(schema.OneOf))
		for _, proxy := range schema.OneOf {
			oneOfRef := SchemaProxyToRefWithVisited(proxy, visited)
			if oneOfRef != nil {
				wrapped.OneOf = append(wrapped.OneOf, oneOfRef)
			}
		}
	}

	// Handle Discriminator
	if schema.Discriminator != nil {
		wrapped.Discriminator = &Discriminator{Discriminator: schema.Discriminator}
	}

	// Convert Extensions
	if schema.Extensions != nil {
		wrapped.Extensions = make(map[string]interface{})
		for pair := schema.Extensions.First(); pair != nil; pair = pair.Next() {
			wrapped.Extensions[pair.Key()] = pair.Value()
		}
	}

	// Handle JSON Schema Draft 2020-12 keywords with nil checks
	if schema.Const != nil {
		wrapped.Const = schema.Const.Value
	}

	if schema.If != nil {
		ifRef := SchemaProxyToRefWithVisited(schema.If, visited)
		if ifRef != nil {
			wrapped.If = ifRef
		}
	}
	if schema.Then != nil {
		thenRef := SchemaProxyToRefWithVisited(schema.Then, visited)
		if thenRef != nil {
			wrapped.Then = thenRef
		}
	}
	if schema.Else != nil {
		elseRef := SchemaProxyToRefWithVisited(schema.Else, visited)
		if elseRef != nil {
			wrapped.Else = elseRef
		}
	}
	if schema.Contains != nil {
		containsRef := SchemaProxyToRefWithVisited(schema.Contains, visited)
		if containsRef != nil {
			wrapped.Contains = containsRef
		}
	}
	if schema.UnevaluatedItems != nil {
		unevaluatedItemsRef := SchemaProxyToRefWithVisited(schema.UnevaluatedItems, visited)
		if unevaluatedItemsRef != nil {
			wrapped.UnevaluatedItems = unevaluatedItemsRef
		}
	}
	if schema.UnevaluatedProperties != nil && schema.UnevaluatedProperties.A != nil {
		unevaluatedPropsRef := SchemaProxyToRefWithVisited(schema.UnevaluatedProperties.A, visited)
		if unevaluatedPropsRef != nil {
			wrapped.UnevaluatedProperties = unevaluatedPropsRef
		}
	}

	// Handle PatternProperties
	if schema.PatternProperties != nil {
		wrapped.PatternProperties = make(map[string]*SchemaRef)
		for pair := schema.PatternProperties.First(); pair != nil; pair = pair.Next() {
			patternPropRef := SchemaProxyToRefWithVisited(pair.Value(), visited)
			if patternPropRef != nil {
				wrapped.PatternProperties[pair.Key()] = patternPropRef
			}
		}
	}

	// Handle PrefixItems
	if schema.PrefixItems != nil {
		wrapped.PrefixItems = make([]*SchemaRef, 0, len(schema.PrefixItems))
		for _, item := range schema.PrefixItems {
			prefixItemRef := SchemaProxyToRefWithVisited(item, visited)
			if prefixItemRef != nil {
				wrapped.PrefixItems = append(wrapped.PrefixItems, prefixItemRef)
			}
		}
	}

	// Handle DependentRequired (check if available in libopenapi version)
	// Note: These fields may not be available in all versions of libopenapi
	// They are part of JSON Schema Draft 2020-12 support

	// Handle DependentSchemas (check if available in libopenapi version)
	// Note: These fields may not be available in all versions of libopenapi
	// They are part of JSON Schema Draft 2020-12 support

	return wrapped
}

// TypeIs checks if the schema type contains the given type string
func (s *Schema) TypeIs(typeStr string) bool {
	if s.Schema == nil {
		return false
	}
	for _, t := range s.Type {
		if t == typeStr {
			return true
		}
	}
	return false
}

// TypeSlice returns the type slice (for OpenAPI 3.1 union types)
func (s *Schema) TypeSlice() []string {
	if s.Schema == nil {
		return nil
	}
	return s.Type
}

// PropertiesToMap converts libopenapi Properties to a map for easier iteration
func (s *Schema) PropertiesToMap() map[string]*SchemaRef {
	return s.PropertiesToMapWithVisited(make(map[*base.Schema]bool))
}

// PropertiesToMapWithVisited converts libopenapi Properties to a map for easier iteration with external visited tracking
func (s *Schema) PropertiesToMapWithVisited(visited map[*base.Schema]bool) map[string]*SchemaRef {
	if s.Schema == nil || s.Properties == nil {
		return nil
	}

	result := make(map[string]*SchemaRef)
	for pair := s.Properties.First(); pair != nil; pair = pair.Next() {
		propertyName := pair.Key()
		schemaProxy := pair.Value()

		// Use SchemaProxyToRefWithVisited to ensure reference restoration works for properties
		schemaRef := SchemaProxyToRefWithVisited(schemaProxy, visited)
		if schemaRef != nil {
			result[propertyName] = schemaRef
		}
		// If schemaRef is nil, we skip this property to prevent nil pointer dereferences
	}

	return result
}

// IsReadOnly returns whether the schema is read-only (handles pointer to bool)
func (s *Schema) IsReadOnly() bool {
	if s.ReadOnly == nil {
		return false
	}
	return *s.ReadOnly
}

// IsWriteOnly returns whether the schema is write-only (handles pointer to bool)
func (s *Schema) IsWriteOnly() bool {
	if s.WriteOnly == nil {
		return false
	}
	return *s.WriteOnly
}

// IsDeprecated returns whether the schema is deprecated (handles pointer to bool)
func (s *Schema) IsDeprecated() bool {
	if s.Deprecated == nil {
		return false
	}
	return *s.Deprecated
}

// ResponseBodies type alias for compatibility
type ResponseBodies map[string]*ResponseRef

// findMatchingComponentSchema attempts to find a component schema that matches the given schema
// This is used to restore reference information lost during libopenapi's reference resolution
func findMatchingComponentSchema(schema *base.Schema) string {
	if schema == nil || globalComponentSchemas == nil {
		return ""
	}

	// Try exact pointer match first (most efficient)
	for componentName, componentSchema := range globalComponentSchemas {
		if schema == componentSchema {
			return componentName
		}
	}

	// If no exact match, try structural matching for resolved references
	// This handles cases where libopenapi creates a copy of the schema during resolution
	if schema.Properties != nil && schema.Properties.Len() > 0 {
		matches := []string{}
		for componentName, componentSchema := range globalComponentSchemas {
			if componentSchema != nil && schemasMatch(schema, componentSchema) {
				matches = append(matches, componentName)
			}
		}

		// If multiple matches found, this could be the source of inconsistency
		if len(matches) > 1 {
			// First priority: Look for exact semantic matches for common simple types
			for _, match := range matches {
				// For simple schemas, prefer exact name matches (e.g., "Version" for version schemas)
				if schema.Properties != nil && schema.Properties.Len() == 1 {
					// Check if this is a simple schema with one property that matches the component name
					for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
						propName := pair.Key()
						if strings.EqualFold(propName, match) {
							return match
						}
						// Handle snake_case to camelCase conversion
						if strings.EqualFold(strings.ReplaceAll(propName, "_", ""), match) {
							return match
						}
					}
				}
			}

			// Second priority: Apply property-based disambiguation for complex schemas
			hasSpecificProperties := hasJobAssignmentProperties(schema) || hasEarningRateProperties(schema)

			if hasSpecificProperties {
				// Try to find the most specific match by preferring schemas that contain
				// information about the structure (e.g., JobAssignments vs WithEarningRates)
				for _, match := range matches {
					// If we have job assignments properties, prefer JobAssignments schema
					if hasJobAssignmentProperties(schema) {
						if strings.Contains(match, "JobAssignments") {
							return match
						}
					}
					// If we have earning rate properties, prefer EarningRates schema
					if hasEarningRateProperties(schema) {
						if strings.Contains(match, "EarningRates") || strings.Contains(match, "WithEarningRates") {
							return match
						}
					}
				}
			}

			// Third priority: For remaining cases, prefer the shortest/simplest component name (likely the base type)
			shortestMatch := matches[0]
			for _, match := range matches[1:] {
				if len(match) < len(shortestMatch) {
					shortestMatch = match
				}
			}
			return shortestMatch
		} else if len(matches) == 1 {
			return matches[0]
		}
	}

	return ""
}

// hasJobAssignmentProperties checks if a schema contains job assignment-related properties
func hasJobAssignmentProperties(schema *base.Schema) bool {
	if schema.Properties == nil {
		return false
	}
	for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
		propName := pair.Key()
		if propName == "job_assignments" || propName == "jobAssignments" {
			return true
		}
	}
	return false
}

// hasEarningRateProperties checks if a schema contains earning rate-related properties
func hasEarningRateProperties(schema *base.Schema) bool {
	if schema.Properties == nil {
		return false
	}
	for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
		propName := pair.Key()
		if propName == "earning_rates" || propName == "earningRates" {
			return true
		}
	}
	return false
}

// schemasMatch checks if two schemas have the same structure (properties, types, etc.)
// This is used to identify when a resolved schema matches a component schema
func schemasMatch(schema1, schema2 *base.Schema) bool {
	if schema1 == nil || schema2 == nil {
		return schema1 == schema2
	}

	// Check basic type information
	if !stringSlicesEqual(schema1.Type, schema2.Type) {
		return false
	}

	// For OpenAPI 3.1 compatibility: Don't require Title/Description to match exactly
	// as these may differ between referenced and inline schemas
	// Instead, focus on structural properties that affect code generation

	// Check properties count and names
	props1Count := 0
	if schema1.Properties != nil {
		props1Count = schema1.Properties.Len()
	}
	props2Count := 0
	if schema2.Properties != nil {
		props2Count = schema2.Properties.Len()
	}
	if props1Count != props2Count {
		return false
	}

	// If both have properties, check if property names match
	// We don't do deep property checking to avoid infinite recursion
	if schema1.Properties != nil && schema2.Properties != nil {
		props1Names := make(map[string]bool)
		for pair := schema1.Properties.First(); pair != nil; pair = pair.Next() {
			props1Names[pair.Key()] = true
		}

		for pair := schema2.Properties.First(); pair != nil; pair = pair.Next() {
			if !props1Names[pair.Key()] {
				return false
			}
		}
	}

	// Check required fields if they exist
	if !stringSlicesEqual(schema1.Required, schema2.Required) {
		return false
	}

	// Check enum values if they exist
	if !yamlNodeSlicesEqual(schema1.Enum, schema2.Enum) {
		return false
	}

	// For object schemas, also check if both have the same additionalProperties setting
	if schema1.AdditionalProperties != nil && schema2.AdditionalProperties != nil {
		if schema1.AdditionalProperties.B != schema2.AdditionalProperties.B {
			return false
		}
	} else if schema1.AdditionalProperties != schema2.AdditionalProperties {
		return false
	}

	return true
}

// yamlNodeSlicesEqual compares two []*yaml.Node slices for equality
func yamlNodeSlicesEqual(a, b []*yaml.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v == nil && b[i] == nil {
			continue
		}
		if v == nil || b[i] == nil {
			return false
		}
		// Compare yaml node values
		if v.Value != b[i].Value || v.Kind != b[i].Kind {
			return false
		}
	}
	return true
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// SchemaProxyToRef converts a libopenapi SchemaProxy to our SchemaRef
func SchemaProxyToRef(proxy *base.SchemaProxy) *SchemaRef {
	return SchemaProxyToRefWithVisited(proxy, make(map[*base.Schema]bool))
}

// SchemaProxyToRefWithVisited converts a libopenapi SchemaProxy to our SchemaRef with circular reference protection
func SchemaProxyToRefWithVisited(proxy *base.SchemaProxy, visited map[*base.Schema]bool) *SchemaRef {
	if proxy == nil {
		return nil
	}

	schema := proxy.Schema()
	if schema == nil {
		return nil
	}

	wrappedSchema := WrapSchemaWithVisited(schema, visited)
	if wrappedSchema == nil {
		return nil
	}

	schemaRef := &SchemaRef{
		Value: wrappedSchema,
	}

	// Handle $ref with siblings (OpenAPI 3.1 feature)
	// If there's a reference, we can still have sibling properties
	ref := proxy.GetReference()
	if ref != "" {
		schemaRef.Ref = ref
		// In OpenAPI 3.1, properties can exist alongside $ref
		// The schemaRef.Value will contain any sibling properties
	} else if schema != nil && globalComponentSchemas != nil && globalComponentSchemaNames != nil {
		// Don't create references for component schema definitions themselves
		// (this prevents recursive type definitions like "type OAuth2Client OAuth2Client")
		if _, isComponentSchema := globalComponentSchemaNames[schema]; !isComponentSchema {
			// Try to match this schema to a component schema, even if visited
			// The visited check is mainly for preventing infinite recursion in wrapping,
			// but for reference restoration, we want to check all schemas
			matchedComponentName := findMatchingComponentSchema(schema)
			if matchedComponentName != "" {
				schemaRef.Ref = "#/components/schemas/" + matchedComponentName
			}
		}
		// Note: When isComponentSchema is true, we skip reference restoration to prevent
		// recursive type definitions like "type OAuth2Client OAuth2Client"
	}

	return schemaRef
}

// SecurityRequirements type alias for compatibility
type SecurityRequirements []SecurityRequirement

// SecurityRequirement represents a security requirement
type SecurityRequirement map[string][]string

// ConvertSecurityRequirements converts libopenapi security requirements to our format
func ConvertSecurityRequirements(reqs []*base.SecurityRequirement) SecurityRequirements {
	if reqs == nil {
		return nil
	}

	result := make(SecurityRequirements, len(reqs))
	for i, req := range reqs {
		requirement := make(SecurityRequirement)
		if req.Requirements != nil {
			for pair := req.Requirements.First(); pair != nil; pair = pair.Next() {
				requirement[pair.Key()] = pair.Value()
			}
		}
		result[i] = requirement
	}
	return result
}

// Server represents an OpenAPI server
type Server struct {
	*v3.Server
	Variables map[string]*ServerVariable
}

// ServerVariable represents a server variable with OpenAPI 3.1 enhancements
type ServerVariable struct {
	*v3.ServerVariable
	// In OpenAPI 3.1, the default field is optional for server variables
}

// WrapServer creates a Server wrapper
func WrapServer(server *v3.Server) *Server {
	if server == nil {
		return nil
	}

	wrapped := &Server{
		Server: server,
	}

	// Convert Variables
	if server.Variables != nil {
		wrapped.Variables = make(map[string]*ServerVariable)
		for pair := server.Variables.First(); pair != nil; pair = pair.Next() {
			wrapped.Variables[pair.Key()] = &ServerVariable{
				ServerVariable: pair.Value(),
			}
		}
	}

	return wrapped
}

// Info represents OpenAPI info object with 3.1 enhancements
type Info struct {
	*base.Info
	Summary string // OpenAPI 3.1 adds summary field to Info object
}

// WrapInfo creates an Info wrapper
func WrapInfo(info *base.Info) *Info {
	if info == nil {
		return nil
	}

	wrapped := &Info{
		Info: info,
	}

	// Handle Summary field for OpenAPI 3.1
	if info.Extensions != nil {
		for pair := info.Extensions.First(); pair != nil; pair = pair.Next() {
			if pair.Key() == "summary" {
				// Extensions in libopenapi are yaml.Node, need to extract string value
				if pair.Value() != nil && pair.Value().Value != "" {
					wrapped.Summary = pair.Value().Value
				}
			}
		}
	}

	return wrapped
}

// License represents OpenAPI license object with 3.1 enhancements
type License struct {
	*base.License
	Identifier string // OpenAPI 3.1 adds identifier field to License object
}

// WrapLicense creates a License wrapper
func WrapLicense(license *base.License) *License {
	if license == nil {
		return nil
	}

	wrapped := &License{
		License: license,
	}

	// Handle Identifier field for OpenAPI 3.1
	if license.Extensions != nil {
		for pair := license.Extensions.First(); pair != nil; pair = pair.Next() {
			if pair.Key() == "identifier" {
				// Extensions in libopenapi are yaml.Node, need to extract string value
				if pair.Value() != nil && pair.Value().Value != "" {
					wrapped.Identifier = pair.Value().Value
				}
			}
		}
	}

	return wrapped
}

// ContentToMap converts libopenapi Content to a map for easier iteration
func ContentToMap(content *orderedmap.Map[string, *v3.MediaType]) map[string]*MediaType {
	if content == nil {
		return nil
	}

	result := make(map[string]*MediaType)
	for pair := content.First(); pair != nil; pair = pair.Next() {
		result[pair.Key()] = WrapMediaType(pair.Value())
	}
	return result
}

// ContentKeys returns sorted keys from libopenapi Content
func ContentKeys(content *orderedmap.Map[string, *v3.MediaType]) []string {
	if content == nil {
		return nil
	}

	var keys []string
	for pair := content.First(); pair != nil; pair = pair.Next() {
		keys = append(keys, pair.Key())
	}
	return keys
}

// AdditionalPropertiesItem represents additional properties for schemas
type AdditionalPropertiesItem struct {
	Has    bool
	Schema *SchemaRef
}

// NewSchemaRef creates a new SchemaRef for compatibility
func NewSchemaRef(ref string, schema *Schema) *SchemaRef {
	return &SchemaRef{
		Ref:   ref,
		Value: schema,
	}
}

// ParametersToRefSlice converts libopenapi parameters to ParameterRef slice
func ParametersToRefSlice(params []*v3.Parameter) []*ParameterRef {
	if params == nil {
		return nil
	}

	result := make([]*ParameterRef, len(params))
	for i, param := range params {
		result[i] = &ParameterRef{
			Value: WrapParameter(param),
		}
	}
	return result
}

// SchemaProxiesToRefs converts schema proxies to SchemaRefs
func SchemaProxiesToRefs(proxies []*base.SchemaProxy) []*SchemaRef {
	if proxies == nil {
		return nil
	}

	result := make([]*SchemaRef, len(proxies))
	for i, proxy := range proxies {
		result[i] = SchemaProxyToRef(proxy)
	}
	return result
}
