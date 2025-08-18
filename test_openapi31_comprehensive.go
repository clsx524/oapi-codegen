package main

import (
	"fmt"
	"log"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
)

func main() {
	spec := `
openapi: 3.1.0
info:
  title: OpenAPI 3.1 Comprehensive Test
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
                $ref: '#/components/schemas/Response'
components:
  schemas:
    Response:
      type: object
      properties:
        # Union types (OpenAPI 3.1 feature)
        unionField:
          type: [string, number]
        # Nullable with union (OpenAPI 3.1 feature)  
        nullableField:
          type: [string, "null"]
        # Const (OpenAPI 3.1 feature)
        constField:
          const: "fixed-value"
        # If/Then/Else (OpenAPI 3.1 feature)
        conditionalField:
          if:
            properties:
              type:
                const: "A"
          then:
            properties:
              dataA:
                type: string
          else:
            properties:
              dataB:
                type: number
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
      responses:
        '200':
          description: Success
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		log.Fatalf("Failed to load spec: %v", err)
	}

	if !swagger.IsOpenAPI31() {
		log.Fatal("Spec is not recognized as OpenAPI 3.1")
	}

	opts := codegen.Configuration{
		PackageName: "testapi",
		Generate: codegen.GenerateOptions{
			Models: true,
		},
	}

	code, err := codegen.Generate(swagger, opts)
	if err != nil {
		log.Fatalf("Failed to generate code: %v", err)
	}

	fmt.Printf("✅ OpenAPI 3.1 support is working!\n")
	fmt.Printf("Generated %d bytes of code\n", len(code))
}
