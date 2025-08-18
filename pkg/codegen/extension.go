package codegen

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

const (
	// extPropGoType overrides the generated type definition.
	extPropGoType = "x-go-type"
	// extPropGoTypeSkipOptionalPointer specifies that optional fields should
	// be the type itself instead of a pointer to the type.
	extPropGoTypeSkipOptionalPointer = "x-go-type-skip-optional-pointer"
	// extPropGoImport specifies the module to import which provides above type
	extPropGoImport = "x-go-type-import"
	// extGoName is used to override a field name
	extGoName = "x-go-name"
	// extGoTypeName is used to override a generated typename for something.
	extGoTypeName        = "x-go-type-name"
	extPropGoJsonIgnore  = "x-go-json-ignore"
	extPropOmitEmpty     = "x-omitempty"
	extPropOmitZero      = "x-omitzero"
	extPropExtraTags     = "x-oapi-codegen-extra-tags"
	extEnumVarNames      = "x-enum-varnames"
	extEnumNames         = "x-enumNames"
	extDeprecationReason = "x-deprecated-reason"
	extOrder             = "x-order"
	// extOapiCodegenOnlyHonourGoName is to be used to explicitly enforce the generation of a field as the `x-go-name` extension has describe it.
	// This is intended to be used alongside the `allow-unexported-struct-field-names` Compatibility option
	extOapiCodegenOnlyHonourGoName = "x-oapi-codegen-only-honour-go-name"
)

// Helper function to decode YAML nodes to Go values
func decodeYamlNode(node interface{}, target interface{}) error {
	if yamlNode, ok := node.(*yaml.Node); ok {
		return yamlNode.Decode(target)
	}
	// If it's not a YAML node, try direct assignment
	switch t := target.(type) {
	case *string:
		if s, ok := node.(string); ok {
			*t = s
			return nil
		}
	case *bool:
		if b, ok := node.(bool); ok {
			*t = b
			return nil
		}
	case *[]string:
		if slice, ok := node.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, v := range slice {
				if s, ok := v.(string); ok {
					result[i] = s
				} else {
					return fmt.Errorf("failed to convert slice element to string: %T", v)
				}
			}
			*t = result
			return nil
		}
	case *map[string]string:
		if m, ok := node.(map[string]interface{}); ok {
			result := make(map[string]string, len(m))
			for k, v := range m {
				if s, ok := v.(string); ok {
					result[k] = s
				} else {
					return fmt.Errorf("failed to convert map value to string: %T", v)
				}
			}
			*t = result
			return nil
		}
	}
	return fmt.Errorf("unsupported type conversion from %T to %T", node, target)
}

func extString(extPropValue interface{}) (string, error) {
	var result string
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return "", err
	}
	return result, nil
}

func extTypeName(extPropValue interface{}) (string, error) {
	return extString(extPropValue)
}

func extParsePropGoTypeSkipOptionalPointer(extPropValue interface{}) (bool, error) {
	var result bool
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return false, err
	}
	return result, nil
}

func extParseGoFieldName(extPropValue interface{}) (string, error) {
	return extString(extPropValue)
}

func extParseOmitEmpty(extPropValue interface{}) (bool, error) {
	var result bool
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return false, err
	}
	return result, nil
}

func extParseOmitZero(extPropValue interface{}) (bool, error) {
	var result bool
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return false, err
	}
	return result, nil
}

func extExtraTags(extPropValue interface{}) (map[string]string, error) {
	var result map[string]string
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func extParseGoJsonIgnore(extPropValue interface{}) (bool, error) {
	var result bool
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return false, err
	}
	return result, nil
}

func extParseEnumVarNames(extPropValue interface{}) ([]string, error) {
	var result []string
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func extParseDeprecationReason(extPropValue interface{}) (string, error) {
	return extString(extPropValue)
}

func extParseOapiCodegenOnlyHonourGoName(extPropValue interface{}) (bool, error) {
	var result bool
	if err := decodeYamlNode(extPropValue, &result); err != nil {
		return false, err
	}
	return result, nil
}
