package codegen

import (
	"fmt"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

type RefWrapper struct {
	Ref       string
	HasValue  bool
	SourceRef interface{}
}

func walkSwagger(swagger *openapi.T, doFn func(RefWrapper) (bool, error)) error {
	if swagger == nil || swagger.Paths == nil {
		return nil
	}

	for _, p := range swagger.Paths.Map() {
		for _, param := range p.Parameters {
			_ = walkParameterRef(param, doFn)
		}
		for _, op := range p.Operations() {
			_ = walkOperation(op, doFn)
		}
	}

	_ = walkComponents(swagger.Components, doFn)

	return nil
}

func walkOperation(op *openapi.Operation, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if op == nil {
		return nil
	}

	for _, param := range openapi.ParametersToRefSlice(op.Parameters) {
		_ = walkParameterRef(param, doFn)
	}

	_ = walkRequestBodyRef(op.RequestBody, doFn)

	if op.Responses != nil {
		for _, response := range op.Responses.Map() {
			_ = walkResponseRef(response, doFn)
		}
	}

	for _, callback := range op.Callbacks {
		_ = walkCallbackRef(callback, doFn)
	}

	return nil
}

func walkComponents(components *openapi.Components, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if components == nil {
		return nil
	}

	for _, schema := range components.Schemas {
		_ = walkSchemaRef(schema, doFn)
	}

	for _, param := range components.Parameters {
		_ = walkParameterRef(param, doFn)
	}

	for _, header := range components.Headers {
		_ = walkHeaderRef(header, doFn)
	}

	for _, requestBody := range components.RequestBodies {
		_ = walkRequestBodyRef(requestBody, doFn)
	}

	for _, response := range components.Responses {
		_ = walkResponseRef(response, doFn)
	}

	for _, securityScheme := range components.SecuritySchemes {
		_ = walkSecuritySchemeRef(securityScheme, doFn)
	}

	for _, example := range components.Examples {
		_ = walkExampleRef(example, doFn)
	}

	for _, link := range components.Links {
		_ = walkLinkRef(link, doFn)
	}

	for _, callback := range components.Callbacks {
		_ = walkCallbackRef(callback, doFn)
	}

	return nil
}

func walkSchemaRef(ref *openapi.SchemaRef, doFn func(RefWrapper) (bool, error)) error {
	visited := make(map[*openapi.Schema]bool)
	return walkSchemaRefWithVisited(ref, doFn, visited)
}

func walkSchemaRefWithVisited(ref *openapi.SchemaRef, doFn func(RefWrapper) (bool, error), visited map[*openapi.Schema]bool) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}

	// Check for circular reference based on the underlying schema
	if ref.Value != nil && visited[ref.Value] {
		return nil
	}
	if ref.Value != nil {
		visited[ref.Value] = true
	}

	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	for _, schemaRef := range ref.OneOf {
		_ = walkSchemaRefWithVisited(schemaRef, doFn, visited)
	}

	for _, schemaRef := range ref.AnyOf {
		_ = walkSchemaRefWithVisited(schemaRef, doFn, visited)
	}

	for _, schemaRef := range ref.AllOf {
		_ = walkSchemaRefWithVisited(schemaRef, doFn, visited)
	}

	_ = walkSchemaRefWithVisited(ref.Not, doFn, visited)
	_ = walkSchemaRefWithVisited(ref.Items, doFn, visited)

	// Convert visited map to base.Schema format
	baseVisited := make(map[*base.Schema]bool)
	for schema, isVisited := range visited {
		if schema != nil && schema.Schema != nil {
			baseVisited[schema.Schema] = isVisited
		}
	}

	for _, propRef := range ref.Value.PropertiesToMapWithVisited(baseVisited) {
		_ = walkSchemaRefWithVisited(propRef, doFn, visited)
	}

	_ = walkSchemaRefWithVisited(ref.AdditionalProperties.Schema, doFn, visited)

	return nil
}

func walkParameterRef(ref *openapi.ParameterRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	_ = walkSchemaRef(ref.Value.Schema, doFn)

	for _, example := range ref.Value.Examples {
		_ = walkExampleRef(example, doFn)
	}

	for _, mediaType := range ref.Value.Content {
		if mediaType == nil {
			continue
		}
		_ = walkSchemaRef(mediaType.Schema, doFn)

		for _, example := range mediaType.Examples {
			_ = walkExampleRef(example, doFn)
		}
	}

	return nil
}

func walkRequestBodyRef(ref *openapi.RequestBodyRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	for _, mediaType := range ref.Value.Content {
		if mediaType == nil {
			continue
		}
		_ = walkSchemaRef(mediaType.Schema, doFn)

		for _, example := range mediaType.Examples {
			_ = walkExampleRef(example, doFn)
		}
	}

	return nil
}

func walkResponseRef(ref *openapi.ResponseRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	for _, header := range ref.Value.Headers {
		_ = walkHeaderRef(header, doFn)
	}

	for _, mediaType := range ref.Value.Content {
		if mediaType == nil {
			continue
		}
		_ = walkSchemaRef(mediaType.Schema, doFn)

		for _, example := range mediaType.Examples {
			_ = walkExampleRef(example, doFn)
		}
	}

	for _, link := range ref.Value.Links {
		_ = walkLinkRef(link, doFn)
	}

	return nil
}

func walkCallbackRef(ref *openapi.CallbackRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	for _, pathItem := range ref.Value.Map() {
		for _, parameter := range pathItem.Parameters {
			_ = walkParameterRef(parameter, doFn)
		}
		// Use the Operations() method which returns wrapped operations
		for _, op := range pathItem.Operations() {
			_ = walkOperation(op, doFn)
		}
	}

	return nil
}

func walkHeaderRef(ref *openapi.HeaderRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	_ = walkSchemaRef(ref.Value.Schema, doFn)

	return nil
}

func walkSecuritySchemeRef(ref *openapi.SecuritySchemeRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	// NOTE: `SecuritySchemeRef`s don't contain any children that can contain refs

	return nil
}

func walkLinkRef(ref *openapi.LinkRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	return nil
}

func walkExampleRef(ref *openapi.ExampleRef, doFn func(RefWrapper) (bool, error)) error {
	// Not a valid ref, ignore it and continue
	if ref == nil {
		return nil
	}
	refWrapper := RefWrapper{Ref: ref.Ref, HasValue: ref.Value != nil, SourceRef: ref}
	shouldContinue, err := doFn(refWrapper)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}
	if ref.Value == nil {
		return nil
	}

	// NOTE: `ExampleRef`s don't contain any children that can contain refs

	return nil
}

func findComponentRefs(swagger *openapi.T) []string {
	refs := []string{}

	_ = walkSwagger(swagger, func(ref RefWrapper) (bool, error) {
		if ref.Ref != "" {
			refs = append(refs, ref.Ref)
			return false, nil
		}
		return true, nil
	})

	// TEMPORARY FIX: Since libopenapi auto-resolves $ref, the walkSwagger doesn't find them
	// For now, mark all component schemas as referenced to avoid over-pruning
	// This is a conservative approach until the pruning logic is properly fixed
	if swagger != nil && swagger.Components != nil && swagger.Components.Schemas != nil {
		for schemaName := range swagger.Components.Schemas {
			schemaRef := fmt.Sprintf("#/components/schemas/%s", schemaName)
			// Only add if not already found by walkSwagger
			found := false
			for _, existingRef := range refs {
				if existingRef == schemaRef {
					found = true
					break
				}
			}
			if !found {
				refs = append(refs, schemaRef)
			}
		}
	}

	return refs
}

func removeOrphanedComponents(swagger *openapi.T, refs []string) int {
	if swagger == nil || swagger.Components == nil {
		return 0
	}

	countRemoved := 0

	for key := range swagger.Components.Schemas {
		ref := fmt.Sprintf("#/components/schemas/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Schemas, key)
		}
	}

	for key := range swagger.Components.Parameters {
		ref := fmt.Sprintf("#/components/parameters/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Parameters, key)
		}
	}

	// securitySchemes are an exception. definitions in securitySchemes
	// are referenced directly by name. and not by $ref

	// for key, _ := range swagger.Components.SecuritySchemes {
	// 	ref := fmt.Sprintf("#/components/securitySchemes/%s", key)
	// 	if !stringInSlice(ref, refs) {
	// 		countRemoved++
	// 		delete(swagger.Components.SecuritySchemes, key)
	// 	}
	// }

	for key := range swagger.Components.RequestBodies {
		ref := fmt.Sprintf("#/components/requestBodies/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.RequestBodies, key)
		}
	}

	for key := range swagger.Components.Responses {
		ref := fmt.Sprintf("#/components/responses/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Responses, key)
		}
	}

	for key := range swagger.Components.Headers {
		ref := fmt.Sprintf("#/components/headers/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Headers, key)
		}
	}

	for key := range swagger.Components.Examples {
		ref := fmt.Sprintf("#/components/examples/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Examples, key)
		}
	}

	for key := range swagger.Components.Links {
		ref := fmt.Sprintf("#/components/links/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Links, key)
		}
	}

	for key := range swagger.Components.Callbacks {
		ref := fmt.Sprintf("#/components/callbacks/%s", key)
		if !stringInSlice(ref, refs) {
			countRemoved++
			delete(swagger.Components.Callbacks, key)
		}
	}

	return countRemoved
}

func pruneUnusedComponents(swagger *openapi.T) {
	if swagger == nil {
		return
	}
	for {
		refs := findComponentRefs(swagger)
		countRemoved := removeOrphanedComponents(swagger, refs)
		if countRemoved < 1 {
			break
		}
	}
}
