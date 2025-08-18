package issue1825

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverlayApply(t *testing.T) {
	spec, err := GetSwagger()
	require.NoError(t, err)

	extensionValue := spec.Info.Extensions.GetOrZero("x-overlay-applied")
	require.NotNil(t, extensionValue, "Extension x-overlay-applied should exist")

	// Decode the YAML node value
	var extensionStringValue string
	err = extensionValue.Decode(&extensionStringValue)
	require.NoError(t, err)

	require.Equal(t, extensionStringValue, "structured-overlay")
}
