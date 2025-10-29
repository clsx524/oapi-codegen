package codegen

import (
	"fmt"
	"strings"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
)

// MergeSchemas merges all the fields in the schemas supplied into one giant schema.
// The idea is that we merge all fields together into one schema.
func MergeSchemas(allOf []*openapi.SchemaRef, path []string) (Schema, error) {
	// If someone asked for the old way, for backward compatibility, return the
	// old style result.
	if globalState.options.Compatibility.OldMergeSchemas {
		return mergeSchemasV1(allOf, path)
	}
	return mergeSchemas(allOf, path)
}

func mergeSchemas(allOf []*openapi.SchemaRef, path []string) (Schema, error) {
	n := len(allOf)

	if n == 0 {
		return Schema{}, fmt.Errorf("no schemas to merge in allOf")
	}

	if n == 1 {
		return GenerateGoSchema(allOf[0], path)
	}

	schema, err := valueWithPropagatedRef(allOf[0])
	if err != nil {
		return Schema{}, err
	}

	for i := 1; i < n; i++ {
		var err error
		oneOfSchema, err := valueWithPropagatedRef(allOf[i])
		if err != nil {
			return Schema{}, err
		}

		mergedSchema, err := mergeOpenapiSchemas(*schema, *oneOfSchema, true)
		if err != nil {
			return Schema{}, fmt.Errorf("error merging schemas for AllOf: %w", err)
		}

		schema = &mergedSchema
	}
	return GenerateGoSchema(openapi.NewSchemaRef("", schema), path)
}

// valueWithPropagatedRef returns a copy of ref schema with its Properties refs
// updated if ref itself is external. Otherwise, return ref.Value as-is.
func valueWithPropagatedRef(ref *openapi.SchemaRef) (*openapi.Schema, error) {
	if len(ref.Ref) == 0 || ref.Ref[0] == '#' {
		return ref.Value, nil
	}

	pathParts := strings.Split(ref.Ref, "#")
	if len(pathParts) < 1 || len(pathParts) > 2 {
		return nil, fmt.Errorf("unsupported reference: %s", ref.Ref)
	}
	remoteComponent := pathParts[0]

	// remote ref
	schema := *ref.Value
	for _, value := range schema.PropertiesToMap() {
		if len(value.Ref) > 0 && value.Ref[0] == '#' {
			// local reference, should propagate remote
			value.Ref = remoteComponent + value.Ref
		}
	}

	return &schema, nil
}

func mergeAllOf(allOf []*openapi.SchemaRef) (*openapi.Schema, error) {
	var schema openapi.Schema
	for _, schemaRef := range allOf {
		var err error
		schema, err = mergeOpenapiSchemas(schema, *schemaRef.Value, true)
		if err != nil {
			return nil, fmt.Errorf("error merging schemas for AllOf: %w", err)
		}
	}
	return &schema, nil
}

// mergeOpenapiSchemas merges two openAPI schemas and returns the schema
// all of whose fields are composed.
func mergeOpenapiSchemas(s1, s2 openapi.Schema, allOf bool) (openapi.Schema, error) {
	// For now, provide a basic implementation that handles the core schema merging
	// This is a simplified version that focuses on the essential properties
	var result openapi.Schema = s1

	// Merge type information - for OpenAPI 3.1 we handle union types
	if len(s1.TypeSlice()) > 0 && len(s2.TypeSlice()) > 0 {
		// Combine types for union support
		typeMap := make(map[string]bool)
		for _, t := range s1.TypeSlice() {
			typeMap[t] = true
		}
		for _, t := range s2.TypeSlice() {
			typeMap[t] = true
		}

		var combinedTypes []string
		for t := range typeMap {
			combinedTypes = append(combinedTypes, t)
		}
		// Note: In a full implementation, we'd set the combined types
		// For now, we'll use the first schema's types
	}

	// For properties, we need to merge them
	s1Props := s1.PropertiesToMap()
	s2Props := s2.PropertiesToMap()

	if s1Props != nil || s2Props != nil {
		// Merge properties from both schemas
		if s2Props != nil {
			if s1Props == nil {
				// If s1 has no properties, use s2's properties
				result.Properties = s2.Properties
			} else {
				// Merge properties: create a new orderedmap with all properties
				// Keep existing properties from s1

				// Add properties from s2 that don't exist in s1
				if result.Properties != nil && s2.Properties != nil {
					for pair := s2.Properties.First(); pair != nil; pair = pair.Next() {
						propertyName := pair.Key()
						// Check if property already exists in result
						hasProperty := false
						shouldReplaceExisting := false
						if result.Properties != nil {
							for existingPair := result.Properties.First(); existingPair != nil; existingPair = existingPair.Next() {
								if existingPair.Key() == propertyName {
									hasProperty = true
									// Handle property conflicts by preferring more specific types
									// For example, prefer enum types over plain string types, or specific array types over generic objects
									existingEnumCount := 0
									newEnumCount := 0
									if existingPair.Value() != nil && existingPair.Value().Schema() != nil {
										existingEnumCount = len(existingPair.Value().Schema().Enum)
									}
									if pair.Value() != nil && pair.Value().Schema() != nil {
										newEnumCount = len(pair.Value().Schema().Enum)
									}

									// Prefer enum over plain string: replace if new has enum and existing doesn't
									if newEnumCount > 0 && existingEnumCount == 0 {
										shouldReplaceExisting = true
									}

									// Prefer specific array type over generic object type
									if existingPair.Value() != nil && pair.Value() != nil {
										// Check if existing is generic object and new is specific array
										existingVal := existingPair.Value()
										newVal := pair.Value()

										// Check type information directly from the SchemaRef
										if existingVal.Schema() != nil && newVal.Schema() != nil {
											existingSchema := existingVal.Schema()
											newSchema := newVal.Schema()

											// Check if existing is generic object type
											existingIsObject := existingSchema.Type != nil && len(existingSchema.Type) > 0 && existingSchema.Type[0] == "object"
											// Check if new is array type with items
											newIsArray := newSchema.Type != nil && len(newSchema.Type) > 0 && newSchema.Type[0] == "array" && newSchema.Items != nil

											if existingIsObject && newIsArray {
												shouldReplaceExisting = true
											}
										}
									}
									break
								}
							}
						}
						// Add property if it doesn't exist or if we should replace existing
						if !hasProperty || shouldReplaceExisting {
							result.Properties.Set(propertyName, pair.Value())
						}
					}
				} else if s2.Properties != nil {
					// If result has no Properties but s2 does, initialize result.Properties
					result.Properties = s2.Properties
				}
			}
		}
	}

	// Handle required fields
	if s1.Required != nil || s2.Required != nil {
		// Merge required fields
		requiredMap := make(map[string]bool)
		for _, req := range s1.Required {
			requiredMap[req] = true
		}
		for _, req := range s2.Required {
			requiredMap[req] = true
		}

		var combinedRequired []string
		for req := range requiredMap {
			combinedRequired = append(combinedRequired, req)
		}
		result.Required = combinedRequired
	}

	return result, nil
}

func equalTypes(t1, t2 []string) bool {
	if len(t1) != len(t2) {
		return false
	}

	// NOTE that ideally we'd use `slices.Equal` but as we're currently supporting Go 1.20+, we can't use it (yet https://github.com/oapi-codegen/oapi-codegen/issues/1634)
	for i := range t1 {
		if t1[i] != t2[i] {
			return false
		}
	}

	return true
}
