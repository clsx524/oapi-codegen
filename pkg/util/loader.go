package util

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/openapi"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"gopkg.in/yaml.v3"
)

func LoadSwagger(filePath string) (swagger *openapi.T, err error) {
	return LoadSwaggerWithIgnoreMissingRefs(filePath, false)
}

func LoadSwaggerWithIgnoreMissingRefs(filePath string, ignoreMissingRefs bool) (swagger *openapi.T, err error) {
	loader := openapi.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.IgnoreMissingRefs = ignoreMissingRefs

	u, err := url.Parse(filePath)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return loader.LoadFromURI(u)
	} else {
		return loader.LoadFromFile(filePath)
	}
}

// Deprecated: In kin-openapi v0.126.0 (https://github.com/getkin/kin-openapi/tree/v0.126.0?tab=readme-ov-file#v01260) the Circular Reference Counter functionality was removed, instead resolving all references with backtracking, to avoid needing to provide a limit to reference counts.
//
// This is now identital in method as `LoadSwagger`.
func LoadSwaggerWithCircularReferenceCount(filePath string, _ int) (swagger *openapi.T, err error) {
	return LoadSwagger(filePath)
}

type LoadSwaggerWithOverlayOpts struct {
	Path              string
	Strict            bool
	IgnoreMissingRefs bool
}

func LoadSwaggerWithOverlay(filePath string, opts LoadSwaggerWithOverlayOpts) (swagger *openapi.T, err error) {
	if opts.Path == "" {
		return LoadSwagger(filePath)
	}

	// Load the overlay
	overlay, err := loader.LoadOverlay(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load overlay: %w", err)
	}
	

	// Check if filePath is a URL, if so download the content to a temporary file
	var actualFilePath string
	var tempFile *os.File
	
	u, err := url.Parse(filePath)
	if err == nil && u.Scheme != "" && u.Host != "" {
		// It's a URL, download the content to a temporary file
		client := &http.Client{}
		resp, err := client.Get(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec from URL %s: %w", filePath, err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch spec from URL %s: status %d", filePath, resp.StatusCode)
		}
		
		// Create a temporary file
		tempFile, err = os.CreateTemp("", "openapi-spec-*.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer os.Remove(tempFile.Name()) // Clean up
		defer tempFile.Close()
		
		// Copy the response to the temporary file
		_, err = io.Copy(tempFile, resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to write spec to temporary file: %w", err)
		}
		
		actualFilePath = tempFile.Name()
	} else {
		// It's already a file path
		actualFilePath = filePath
	}


	// Load the specification
	specNode, _, err := loader.LoadEitherSpecification(actualFilePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load specification: %w", err)
	}
	
	// Apply the overlay to the specification
	err = overlay.ApplyTo(specNode)
	if err != nil {
		return nil, fmt.Errorf("failed to apply overlay: %w", err)
	}
	
	overlayedNode := specNode


	// Convert the YAML node back to bytes
	if overlayedNode != nil {
		// Serialize the overlayed node back to YAML bytes
		overlayedBytes, err := yaml.Marshal(overlayedNode)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize overlayed spec: %w", err)
		}


		// Load the overlayed spec using our normal loader with base path preservation
		loader := openapi.NewLoader()
		loader.IsExternalRefsAllowed = true
		loader.IgnoreMissingRefs = opts.IgnoreMissingRefs

		// Extract base path from the original file path for reference resolution
		basePath := ""
		
		// Check if the original filePath was a URL
		if u, err := url.Parse(filePath); err == nil && u.Scheme != "" && u.Host != "" {
			// For URLs, use the URL's base path
			u.Path = filepath.Dir(u.Path)
			basePath = u.String()
		} else {
			// For file paths, extract directory path
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
		}

		return loader.LoadFromDataWithBasePath(overlayedBytes, basePath)
	}

	return LoadSwagger(filePath)
}
