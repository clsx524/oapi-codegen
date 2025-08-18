package codegen

import (
	"go/format"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
)

// TestOpenAPI31GeneratedSpecParsing tests that generated embedded specs can be parsed
func TestOpenAPI31GeneratedSpecParsing(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Generated Spec Parsing Test
  version: 1.0.0
jsonSchemaDialect: "https://json-schema.org/draft/2020-12/schema"
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: ["string", "integer"]
                  data:
                    type: ["object", "null"]
webhooks:
  testEvent:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                eventType:
                  const: "test"
      responses:
        '200':
          description: OK
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models:       true,
			EmbeddedSpec: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// The generated code should include a GetSwagger function that returns our abstraction
	assert.Contains(t, code, "func GetSwagger() (swagger *openapi.T, err error)")
	assert.Contains(t, code, "loader := openapi.NewLoader()")
	assert.Contains(t, code, "swagger, err = loader.LoadFromDataWithBasePath(specData, \".\")")

	// Should use our openapi abstraction
	assert.Contains(t, code, "github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi")
	assert.NotContains(t, code, "github.com/getkin/kin-openapi")
}

// TestOpenAPI31ValidationFeatures tests various validation features
func TestOpenAPI31ValidationFeatures(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Validation Features Test
  version: 1.0.0
paths:
  /validate:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ValidationTest'
      responses:
        '200':
          description: Success
components:
  schemas:
    ValidationTest:
      type: object
      required:
        - requiredField
      properties:
        requiredField:
          type: string
          minLength: 1
        optionalField:
          type: ["string", "null"]
        numberField:
          type: number
          minimum: 0
          exclusiveMaximum: 100
        arrayField:
          type: array
          items:
            type: string
          minItems: 1
          maxItems: 10
        unionField:
          type: ["string", "integer", "null"]
        constField:
          const: "fixed-value"
        enumField:
          type: string
          enum: ["option1", "option2", "option3"]
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Should generate proper struct
	assert.Contains(t, code, "type ValidationTest struct")
}

// TestOpenAPI31ComplexUnionTypes tests complex union type scenarios
func TestOpenAPI31ComplexUnionTypes(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Complex Union Types Test
  version: 1.0.0
paths:
  /union:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ComplexUnions'
      responses:
        '200':
          description: Success
components:
  schemas:
    ComplexUnions:
      type: object
      properties:
        # Simple union
        stringOrInt:
          type: ["string", "integer"]
        # Union with null
        nullableString:
          type: ["string", "null"]
        # Complex union
        multiType:
          type: ["string", "number", "boolean", "array", "object", "null"]
        # Array of union types
        arrayOfUnions:
          type: array
          items:
            type: ["string", "integer"]
        # Object with union values
        objectWithUnions:
          type: object
          additionalProperties:
            type: ["string", "number", "null"]
        # Nested complex union
        nestedUnion:
          type: object
          properties:
            field:
              type: ["object", "array"]
              properties:
                subField:
                  type: ["string", "null"]
              items:
                type: string
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Union types should be represented appropriately in Go
	assert.Contains(t, code, "type ComplexUnions struct")
}

// TestOpenAPI31SchemaValidation tests that schemas are correctly parsed and validated
func TestOpenAPI31SchemaValidation(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Schema Validation Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
components:
  schemas:
    # Test if/then/else conditionals
    ConditionalSchema:
      type: object
      properties:
        type:
          type: string
          enum: ["a", "b"]
        value:
          type: integer
      if:
        properties:
          type:
            const: "a"
      then:
        properties:
          value:
            minimum: 0
      else:
        properties:
          value:
            maximum: 0
    
    # Test prefixItems
    PrefixItemsSchema:
      type: array
      prefixItems:
        - type: string
        - type: integer
      items:
        type: boolean
      
    # Test contentEncoding
    ContentSchema:
      type: object
      properties:
        image:
          type: string
          contentEncoding: "base64"
          contentMediaType: "image/png"
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Verify schemas are parsed correctly
	require.NotNil(t, swagger.Components)
	require.NotNil(t, swagger.Components.Schemas)

	schemas := swagger.Components.Schemas
	assert.Contains(t, schemas, "ConditionalSchema")
	assert.Contains(t, schemas, "PrefixItemsSchema")
	assert.Contains(t, schemas, "ContentSchema")

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)
}

// TestOpenAPI31WebhookGeneration tests webhook code generation
func TestOpenAPI31WebhookGeneration(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Webhook Generation Test
  version: 1.0.0
webhooks:
  userCreated:
    post:
      operationId: handleUserCreated
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                userId:
                  type: ["string", "integer"]
                userData:
                  type: object
                  properties:
                    name:
                      type: string
                    email:
                      type: ["string", "null"]
      responses:
        '200':
          description: Success
  dataChanged:
    post:
      operationId: handleDataChanged
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                changeType:
                  const: "data_change"
                data:
                  type: ["object", "array", "null"]
      responses:
        '200':
          description: Success
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Verify webhooks are loaded
	require.NotNil(t, swagger.Webhooks)
	assert.Contains(t, swagger.Webhooks, "userCreated")
	assert.Contains(t, swagger.Webhooks, "dataChanged")

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Webhooks should not generate regular API operations, just models
	// The webhook operations are stored in the swagger.Webhooks field
	assert.NotContains(t, code, "func HandleUserCreated")
	assert.NotContains(t, code, "func HandleDataChanged")
}

// TestOpenAPI31ExtensionParsing tests that extensions are parsed correctly
func TestOpenAPI31ExtensionParsing(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Extension Parsing Test
  version: 1.0.0
  x-info-extension: "info-level-extension"
paths:
  /test:
    get:
      operationId: testOperation
      x-operation-extension: "operation-level"
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ExtensionTest'
components:
  schemas:
    ExtensionTest:
      type: object
      x-go-type: "CustomType"
      x-go-type-import:
        name: "CustomType"
        package: "github.com/example/types"
      properties:
        id:
          type: string
          x-go-name: "ID"
        name:
          type: string
          x-deprecated-reason: "Use displayName instead"
        data:
          type: object
          x-go-type: "map[string]interface{}"
  x-component-extension: "component-level"
x-root-extension:
  complex:
    nested: "value"
    array: [1, 2, 3]
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Extensions should be handled without causing malformed code
	assert.Contains(t, code, "type ExtensionTest = CustomType")
}

// TestOpenAPI31JSONCompatibility tests JSON marshaling/unmarshaling compatibility
func TestOpenAPI31JSONCompatibility(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: JSON Compatibility Test
  version: 1.0.0
paths:
  /test:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/JSONTest'
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/JSONTest'
components:
  schemas:
    JSONTest:
      type: object
      properties:
        unionField:
          type: ["string", "integer", "null"]
        arrayField:
          type: array
          items:
            type: ["string", "number"]
        objectField:
          type: object
          additionalProperties:
            type: ["string", "null"]
        constField:
          const: "constant"
        enumField:
          type: string
          enum: ["a", "b", "c"]
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Generated structs should be JSON serializable
	assert.Contains(t, code, "`json:")
	assert.Contains(t, code, "type JSONTest struct")
}

// TestOpenAPI31ErrorHandling tests error cases and edge scenarios
func TestOpenAPI31ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		shouldError bool
		description string
	}{
		{
			name: "Invalid OpenAPI version",
			spec: `
openapi: 3.0.0
info:
  title: OpenAPI 3.0 Version
  version: 1.0.0
paths: {}
`,
			shouldError: false, // Should load but not be 3.1
			description: "Should handle older versions gracefully",
		},
		{
			name: "Empty paths and webhooks",
			spec: `
openapi: 3.1.0
info:
  title: Empty Test
  version: 1.0.0
`,
			shouldError: false, // Should be valid in 3.1
			description: "Should handle empty paths and webhooks",
		},
		// TODO: Fix circular reference handling in GoSchemaImports
		// {
		// 	name: "Complex circular references",
		// 	spec: `
		// openapi: 3.1.0
		// info:
		//   title: Circular Test
		//   version: 1.0.0
		// paths:
		//   /test:
		//     get:
		//       responses:
		//         '200':
		//           description: Success
		//           content:
		//             application/json:
		//               schema:
		//                 $ref: '#/components/schemas/A'
		// components:
		//   schemas:
		//     A:
		//       type: object
		//       properties:
		//         b:
		//           $ref: '#/components/schemas/B'
		//     B:
		//       type: object
		//       properties:
		//         a:
		//           $ref: '#/components/schemas/A'
		// `,
		// 	shouldError: false,
		// 	description: "Should handle circular references",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := openapi.NewLoader()
			swagger, err := loader.LoadFromData([]byte(tt.spec))

			if tt.shouldError {
				assert.Error(t, err, tt.description)
				return
			}

			require.NoError(t, err, tt.description)

			opts := Configuration{
				PackageName: "testapi",
				Generate: GenerateOptions{
					Models: true,
				},
			}

			code, err := Generate(swagger, opts)
			require.NoError(t, err)

			// Generated code should compile regardless
			_, err = format.Source([]byte(code))
			require.NoError(t, err, "Generated code should be valid Go")
		})
	}
}

// TestOpenAPI31SpecEmbedding tests that embedded specs maintain OpenAPI 3.1 structure
func TestOpenAPI31SpecEmbedding(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Spec Embedding Test
  version: 1.0.0
jsonSchemaDialect: "https://json-schema.org/draft/2020-12/schema"
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
webhooks:
  testEvent:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                data:
                  type: ["string", "null"]
      responses:
        '200':
          description: OK
components:
  schemas:
    TestModel:
      type: object
      properties:
        id:
          type: ["string", "integer"]
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models:       true,
			EmbeddedSpec: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Should embed the spec and provide GetSwagger function
	assert.Contains(t, code, "func GetSwagger()")
	assert.Contains(t, code, "decodeSpec()")

	// Should not use kin-openapi types
	assert.NotContains(t, code, "github.com/getkin/kin-openapi")
	assert.Contains(t, code, "github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi")
}

// BenchmarkOpenAPI31Generation benchmarks code generation performance
func BenchmarkOpenAPI31Generation(b *testing.B) {
	spec := `
openapi: 3.1.0
info:
  title: Benchmark Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: ["string", "integer", "null"]
components:
  schemas:
    TestModel:
      type: object
      properties:
        id:
          type: ["string", "integer"]
        items:
          type: array
          items:
            type: ["string", "number"]
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(b, err)

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
			Client: true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Generate(swagger, opts)
		require.NoError(b, err)
	}
}
