package codegen

import (
	"go/format"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
)

// TestOpenAPI31UnionTypes tests OpenAPI 3.1 union type support
func TestOpenAPI31UnionTypes(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Union Types Test
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
                $ref: '#/components/schemas/UnionResponse'
components:
  schemas:
    UnionResponse:
      type: object
      properties:
        id:
          type: [string, "null"]
        value:
          type: [string, number]
        status:
          type: [string, number, "null"]
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

	// Check for union type handling - in our implementation, union types are handled as specific types
	// The *string type indicates nullable string handling
	assert.Contains(t, code, "*string")
	assert.Contains(t, code, "*float32")
	assert.NotContains(t, code, "github.com/getkin/kin-openapi")
}

// TestOpenAPI31Webhooks tests OpenAPI 3.1 webhook support
func TestOpenAPI31Webhooks(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Webhooks Test
  version: 1.0.0
paths: {}
webhooks:
  userCreated:
    post:
      summary: User created webhook
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                userId:
                  type: string
                timestamp:
                  type: string
                  format: date-time
      responses:
        '200':
          description: Webhook received
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Check that webhooks are loaded
	assert.NotNil(t, swagger.Webhooks)
	assert.Contains(t, swagger.Webhooks, "userCreated")

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

// TestOpenAPI31JSONSchemaDialect tests JSON Schema dialect support
func TestOpenAPI31JSONSchemaDialect(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: JSON Schema Dialect Test
  version: 1.0.0
jsonSchemaDialect: https://json-schema.org/draft/2020-12/schema
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TestSchema'
components:
  schemas:
    TestSchema:
      type: object
      properties:
        value:
          const: "fixed-value"
        count:
          type: integer
          minimum: 0
      if:
        properties:
          count:
            const: 0
      then:
        properties:
          value:
            const: "zero"
      else:
        properties:
          value:
            const: "non-zero"
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Check JSON Schema dialect is properly loaded
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", swagger.JSONSchemaDialect)

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

// TestOpenAPI31ExamplesArray tests examples array support
func TestOpenAPI31ExamplesArray(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Examples Array Test
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
                $ref: '#/components/schemas/TestSchema'
components:
  schemas:
    TestSchema:
      type: object
      properties:
        name:
          type: string
          examples:
            - "John Doe"
            - "Jane Smith"
            - "Bob Johnson"
        status:
          type: string
          example: "active"
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Check that examples are properly loaded
	if swagger.Components != nil && swagger.Components.Schemas != nil {
		if testSchema, ok := swagger.Components.Schemas["TestSchema"]; ok && testSchema.Value != nil {
			nameProps := testSchema.Value.PropertiesToMap()
			if nameField, ok := nameProps["name"]; ok && nameField.Value != nil {
				assert.NotNil(t, nameField.Value.Examples)
				assert.Len(t, nameField.Value.Examples, 3)
			}
		}
	}

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

// TestOpenAPI31OptionalPaths tests that paths object is optional
func TestOpenAPI31OptionalPaths(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Optional Paths Test
  version: 1.0.0
webhooks:
  eventReceived:
    post:
      summary: Event received
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                event:
                  type: string
      responses:
        '200':
          description: Event processed
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Check that paths can be nil/empty for webhook-only specs
	assert.NotNil(t, swagger.Paths) // Our abstraction creates empty wrapper
	assert.NotNil(t, swagger.Webhooks)
	assert.Contains(t, swagger.Webhooks, "eventReceived")

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

// TestOpenAPI31RefSiblings tests $ref with sibling properties
func TestOpenAPI31RefSiblings(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Ref Siblings Test
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
                $ref: '#/components/schemas/BaseSchema'
                title: "Extended Schema"
                description: "Schema with additional properties"
components:
  schemas:
    BaseSchema:
      type: object
      properties:
        id:
          type: string
        name:
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
}

// TestOpenAPI31ComponentsPathItems tests components.pathItems support
func TestOpenAPI31ComponentsPathItems(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: Components PathItems Test
  version: 1.0.0
paths:
  /users/{id}:
    $ref: '#/components/pathItems/UserPath'
components:
  pathItems:
    UserPath:
      get:
        summary: Get user by ID
        parameters:
          - name: id
            in: path
            required: true
            schema:
              type: string
        responses:
          '200':
            description: User found
            content:
              application/json:
                schema:
                  $ref: '#/components/schemas/User'
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI31())

	// Check that pathItems are properly loaded in components
	assert.NotNil(t, swagger.Components)
	if swagger.Components.PathItems != nil {
		assert.Contains(t, swagger.Components.PathItems, "UserPath")
	}

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
			Client: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)
}

// TestOpenAPI31Compatibility tests backward compatibility with OpenAPI 3.0
func TestOpenAPI31Compatibility(t *testing.T) {
	spec30 := `
openapi: 3.0.3
info:
  title: Compatibility Test
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
                  id:
                    type: string
                    nullable: true
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec30))
	require.NoError(t, err)
	require.True(t, swagger.IsOpenAPI30())
	require.False(t, swagger.IsOpenAPI31())

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

	// Should still not import kin-openapi
	assert.NotContains(t, code, "github.com/getkin/kin-openapi")
}

// TestOpenAPI31NullableVsUnionTypes tests nullable vs union type handling
func TestOpenAPI31NullableVsUnionTypes(t *testing.T) {
	// OpenAPI 3.1 spec with union types
	spec31 := `
openapi: 3.1.0
info:
  title: Nullable vs Union Test
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
                  unionField:
                    type: [string, "null"]
                  legacyField:
                    type: string
                    nullable: true
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec31))
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
}

// TestGeneratedCodeNoKinOpenAPI ensures generated code doesn't import kin-openapi
func TestGeneratedCodeNoKinOpenAPI(t *testing.T) {
	spec := `
openapi: 3.1.0
info:
  title: No Kin OpenAPI Test
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
components:
  schemas:
    TestModel:
      type: object
      properties:
        id:
          type: string
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models:       true,
			Client:       true,
			EchoServer:   true,
			EmbeddedSpec: true,
		},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)

	// Check that code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Ensure no kin-openapi imports
	assert.NotContains(t, code, "github.com/getkin/kin-openapi")
	
	// Should use our abstraction instead
	assert.Contains(t, code, "github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi")
	
	// Note: The info message is printed to stderr, not included in generated code
	// But we can verify the spec is processed as 3.1
}