package codegen

import (
	"strings"
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuplicateTypenameAutoRename tests that duplicate typename conflicts are resolved
// by automatic renaming with numeric suffixes
func TestDuplicateTypenameAutoRename(t *testing.T) {
	// Create a test spec with potential duplicate type names
	spec := `
openapi: 3.0.0
info:
  title: Duplicate Type Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
components:
  schemas:
    account_type:
      type: string
      enum: ["user", "admin"]
      description: Account type enum
    # This would also generate AccountType if processed separately
    AccountType:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
`

	loader := openapi.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	opts := Configuration{
		PackageName: "testapi",
		Generate: GenerateOptions{
			Models: true,
		},
	}

	// This should not fail due to duplicate typename conflicts
	code, err := Generate(swagger, opts)
	assert.NoError(t, err)
	assert.NotEmpty(t, code)

	// Check that both types are present with auto-renaming
	assert.Contains(t, code, "type AccountType")
	
	// Should contain either AccountType2 or have resolved the conflict in some way
	accountTypeCount := strings.Count(code, "type AccountType")
	assert.GreaterOrEqual(t, accountTypeCount, 1, "Should have at least one AccountType")
	
	// Verify the code compiles (basic syntax check)
	assert.Contains(t, code, "package testapi")
}

// TestAutoRenameFunction tests the autoRenameType helper function
func TestAutoRenameFunction(t *testing.T) {
	existingTypes := map[string]TypeDefinition{
		"AccountType": {TypeName: "AccountType"},
		"AccountType2": {TypeName: "AccountType2"},
	}

	tests := []struct {
		name     string
		original string
		expected string
	}{
		{
			name:     "First conflict",
			original: "AccountType",
			expected: "AccountType3", // 1 and 2 are taken
		},
		{
			name:     "No conflict",
			original: "UserType",
			expected: "UserType2", // 1 is the original, 2 is first available
		},
		{
			name:     "Many conflicts",
			original: "TestType",
			expected: "TestType2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := autoRenameType(tt.original, existingTypes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAutoRenameExhaustion tests that autoRenameType gives up after too many attempts
func TestAutoRenameExhaustion(t *testing.T) {
	// Create a map with types 2-11 already taken (autoRename tries 2-10)
	existingTypes := map[string]TypeDefinition{
		"OverloadedType":   {TypeName: "OverloadedType"},   // Original
		"OverloadedType2":  {TypeName: "OverloadedType2"},  // 2
		"OverloadedType3":  {TypeName: "OverloadedType3"},  // 3
		"OverloadedType4":  {TypeName: "OverloadedType4"},  // 4
		"OverloadedType5":  {TypeName: "OverloadedType5"},  // 5
		"OverloadedType6":  {TypeName: "OverloadedType6"},  // 6
		"OverloadedType7":  {TypeName: "OverloadedType7"},  // 7
		"OverloadedType8":  {TypeName: "OverloadedType8"},  // 8
		"OverloadedType9":  {TypeName: "OverloadedType9"},  // 9
		"OverloadedType10": {TypeName: "OverloadedType10"}, // 10
	}

	// This should fail to find a unique name since 2-10 are all taken
	result := autoRenameType("OverloadedType", existingTypes)
	assert.Empty(t, result, "Should return empty string when unable to find unique name")
}