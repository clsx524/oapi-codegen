package main

import (
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
)

func TestLoader(t *testing.T) {

	paths := []string{
		"../../examples/petstore-expanded/petstore-expanded.yaml",
		"https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/main/examples/petstore-expanded/petstore-expanded.yaml",
	}

	for _, v := range paths {
		t.Logf("Testing path: %s", v)
		swagger, err := util.LoadSwagger(v)
		if err != nil {
			t.Errorf("Error loading %s: %v", v, err)
			continue
		}
		if swagger == nil || swagger.Info == nil || swagger.Info.Version == "" {
			t.Errorf("Missing data for path: %s", v)
		}
	}
}
