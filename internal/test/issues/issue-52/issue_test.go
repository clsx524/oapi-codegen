package issue52

import (
	_ "embed"
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/stretchr/testify/require"
)

//go:embed spec.yaml
var spec []byte

func TestIssue(t *testing.T) {
	swagger, err := openapi.NewLoader().LoadFromData(spec)
	require.NoError(t, err)

	opts := codegen.Configuration{
		PackageName: "issue52",
		Generate: codegen.GenerateOptions{
			EchoServer:   true,
			Client:       true,
			Models:       true,
			EmbeddedSpec: true,
		},
	}

	_, err = codegen.Generate(swagger, opts)
	require.NoError(t, err)
}
