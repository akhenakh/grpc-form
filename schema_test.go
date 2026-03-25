package main

import (
	"testing"

	testv1 "github.com/akhenakh/grpc-form/gen/test/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestExtractStringValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	tests := []struct {
		name          string
		fieldName     string
		expectedRules map[string]any
	}{
		{
			name:      "string_min_max",
			fieldName: "string_min_max",
			expectedRules: map[string]any{
				"minLen": int(5),
				"maxLen": int(100),
			},
		},
		{
			name:      "string_exact_len",
			fieldName: "string_exact_len",
			expectedRules: map[string]any{
				"len": int(10),
			},
		},
		{
			name:      "string_email",
			fieldName: "string_email",
			expectedRules: map[string]any{
				"email": true,
			},
		},
		{
			name:      "string_uuid",
			fieldName: "string_uuid",
			expectedRules: map[string]any{
				"uuid": true,
			},
		},
		{
			name:      "string_pattern",
			fieldName: "string_pattern",
			expectedRules: map[string]any{
				"pattern": "^[A-Z][a-z]+$",
			},
		},
		{
			name:      "string_prefix_suffix",
			fieldName: "string_prefix_suffix",
			expectedRules: map[string]any{
				"prefix": "pre_",
				"suffix": "_suf",
			},
		},
		{
			name:      "string_in",
			fieldName: "string_in",
			expectedRules: map[string]any{
				"in": []string{"one", "two", "three"},
			},
		},
		{
			name:      "string_const",
			fieldName: "string_const",
			expectedRules: map[string]any{
				"const": "fixed_value",
			},
		},
		{
			name:      "string_hostname",
			fieldName: "string_hostname",
			expectedRules: map[string]any{
				"hostname": true,
			},
		},
		{
			name:      "string_uri",
			fieldName: "string_uri",
			expectedRules: map[string]any{
				"uri": true,
			},
		},
		{
			name:      "string_ip",
			fieldName: "string_ip",
			expectedRules: map[string]any{
				"ip": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := msg.Fields().ByName(protoreflect.Name(tt.fieldName))
			require.NotNil(t, field, "field %s not found", tt.fieldName)

			rules := extractValidationRules(field)

			for key, expected := range tt.expectedRules {
				actual, ok := rules[key]
				assert.True(t, ok, "expected rule %s to exist", key)
				assert.Equal(t, expected, actual, "rule %s mismatch", key)
			}
		})
	}
}

func TestExtractIntValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	tests := []struct {
		name          string
		fieldName     string
		expectedRules map[string]any
	}{
		{
			name:      "int32_range",
			fieldName: "int32_range",
			expectedRules: map[string]any{
				"gte": int(0),
				"lte": int(100),
				"min": int(0),
				"max": int(100),
			},
		},
		{
			name:      "int64_gt_lt",
			fieldName: "int64_gt_lt",
			expectedRules: map[string]any{
				"gt": int(0),
				"lt": int(1000),
			},
		},
		{
			name:      "uint32_in",
			fieldName: "uint32_in",
			expectedRules: map[string]any{
				"in": []int{1, 2, 5, 10},
			},
		},
		{
			name:      "int32_const",
			fieldName: "int32_const",
			expectedRules: map[string]any{
				"const": int(42),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := msg.Fields().ByName(protoreflect.Name(tt.fieldName))
			require.NotNil(t, field, "field %s not found", tt.fieldName)

			rules := extractValidationRules(field)

			for key, expected := range tt.expectedRules {
				actual, ok := rules[key]
				assert.True(t, ok, "expected rule %s to exist", key)
				assert.Equal(t, expected, actual, "rule %s mismatch", key)
			}
		})
	}
}

func TestExtractFloatValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	tests := []struct {
		name          string
		fieldName     string
		expectedRules map[string]any
	}{
		{
			name:      "float_range",
			fieldName: "float_range",
			expectedRules: map[string]any{
				"gte": float64(0.0),
				"lte": float64(1.0),
				"min": float64(0.0),
				"max": float64(1.0),
			},
		},
		{
			name:      "double_finite",
			fieldName: "double_finite",
			expectedRules: map[string]any{
				"finite": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := msg.Fields().ByName(protoreflect.Name(tt.fieldName))
			require.NotNil(t, field, "field %s not found", tt.fieldName)

			rules := extractValidationRules(field)

			for key, expected := range tt.expectedRules {
				actual, ok := rules[key]
				assert.True(t, ok, "expected rule %s to exist", key)
				assert.Equal(t, expected, actual, "rule %s mismatch", key)
			}
		})
	}
}

func TestExtractBoolValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	field := msg.Fields().ByName(protoreflect.Name("bool_const"))
	require.NotNil(t, field)

	rules := extractValidationRules(field)
	assert.True(t, rules["const"].(bool))
}

func TestExtractEnumValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	tests := []struct {
		name          string
		fieldName     string
		expectedRules map[string]any
	}{
		{
			name:      "enum_field",
			fieldName: "enum_field",
			expectedRules: map[string]any{
				"definedOnly": true,
			},
		},
		{
			name:      "enum_in",
			fieldName: "enum_in",
			expectedRules: map[string]any{
				"in": []int32{1, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := msg.Fields().ByName(protoreflect.Name(tt.fieldName))
			require.NotNil(t, field, "field %s not found", tt.fieldName)

			rules := extractValidationRules(field)

			for key, expected := range tt.expectedRules {
				actual, ok := rules[key]
				assert.True(t, ok, "expected rule %s to exist", key)
				assert.Equal(t, expected, actual, "rule %s mismatch", key)
			}
		})
	}
}

func TestExtractRepeatedValidationRules(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	tests := []struct {
		name          string
		fieldName     string
		expectedRules map[string]any
	}{
		{
			name:      "repeated_strings",
			fieldName: "repeated_strings",
			expectedRules: map[string]any{
				"minItems": int(1),
				"maxItems": int(10),
			},
		},
		{
			name:      "repeated_unique",
			fieldName: "repeated_unique",
			expectedRules: map[string]any{
				"unique": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := msg.Fields().ByName(protoreflect.Name(tt.fieldName))
			require.NotNil(t, field, "field %s not found", tt.fieldName)

			rules := extractValidationRules(field)

			for key, expected := range tt.expectedRules {
				actual, ok := rules[key]
				assert.True(t, ok, "expected rule %s to exist", key)
				assert.Equal(t, expected, actual, "rule %s mismatch", key)
			}
		})
	}
}

func TestExtractRequiredField(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	field := msg.Fields().ByName(protoreflect.Name("required_field"))
	require.NotNil(t, field)

	rules := extractValidationRules(field)
	assert.True(t, rules["required"].(bool))
	assert.Equal(t, 1, rules["minLen"])
}

func TestBuildFieldSchema(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()

	t.Run("extracts all field properties", func(t *testing.T) {
		field := msg.Fields().ByName(protoreflect.Name("string_email"))
		require.NotNil(t, field)

		rootSchema := map[string]any{
			"enums":    map[string]any{},
			"messages": map[string]any{},
		}

		fSchema := buildFieldSchema(rootSchema, field)

		assert.Equal(t, "string_email", fSchema["name"])
		assert.Equal(t, "string", fSchema["type"])
		assert.False(t, fSchema["repeated"].(bool))
		assert.False(t, fSchema["isEnum"].(bool))
		assert.False(t, fSchema["isMessage"].(bool))

		validateRules := fSchema["validate"].(map[string]any)
		assert.True(t, validateRules["email"].(bool))
	})

	t.Run("handles repeated fields", func(t *testing.T) {
		field := msg.Fields().ByName(protoreflect.Name("repeated_strings"))
		require.NotNil(t, field)

		rootSchema := map[string]any{
			"enums":    map[string]any{},
			"messages": map[string]any{},
		}

		fSchema := buildFieldSchema(rootSchema, field)

		assert.True(t, fSchema["repeated"].(bool))
		validateRules := fSchema["validate"].(map[string]any)
		assert.Equal(t, 1, validateRules["minItems"])
		assert.Equal(t, 10, validateRules["maxItems"])
	})

	t.Run("extracts enum field type", func(t *testing.T) {
		field := msg.Fields().ByName(protoreflect.Name("enum_field"))
		require.NotNil(t, field)

		rootSchema := map[string]any{
			"enums":    map[string]any{},
			"messages": map[string]any{},
		}

		fSchema := buildFieldSchema(rootSchema, field)

		assert.True(t, fSchema["isEnum"].(bool))
		assert.Contains(t, fSchema["type"].(string), "TestEnum")

		enums := rootSchema["enums"].(map[string]any)
		assert.Contains(t, enums, "test.v1.TestEnum")
	})
}

func TestBuildUISchema(t *testing.T) {
	t.Run("buildsschema for test message", func(t *testing.T) {
		msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()
		schema := buildUISchema(msg)

		assert.Equal(t, "", schema["title"])
		assert.Equal(t, "", schema["description"])

		fields := schema["fields"].([]map[string]any)
		assert.Greater(t, len(fields), 0)

		fieldsMap := make(map[string]map[string]any)
		for _, f := range fields {
			fieldsMap[f["name"].(string)] = f
		}

		if f, ok := fieldsMap["string_email"]; ok {
			validateRules := f["validate"].(map[string]any)
			assert.True(t, validateRules["email"].(bool))
		}

		if f, ok := fieldsMap["required_field"]; ok {
			validateRules := f["validate"].(map[string]any)
			assert.True(t, validateRules["required"].(bool))
		}
	})
}

func TestPopulateEnums(t *testing.T) {
	msg := (&testv1.TestMessage{}).ProtoReflect().Descriptor()
	field := msg.Fields().ByName(protoreflect.Name("enum_field"))
	require.NotNil(t, field)

	enumDesc := field.Enum()
	require.NotNil(t, enumDesc)

	rootSchema := map[string]any{
		"enums":    map[string]any{},
		"messages": map[string]any{},
	}

	populateEnums(rootSchema, enumDesc)

	enums := rootSchema["enums"].(map[string]any)
	enumName := string(enumDesc.FullName())

	assert.Contains(t, enums, enumName)

	enumValues := enums[enumName].([]map[string]any)
	assert.Greater(t, len(enumValues), 0)

	foundUnspecified := false
	for _, v := range enumValues {
		if v["name"] == "TEST_ENUM_UNSPECIFIED" {
			foundUnspecified = true
			assert.Equal(t, int32(0), v["number"])
		}
	}
	assert.True(t, foundUnspecified, "TEST_ENUM_UNSPECIFIED not found in enum values")
}

func TestDynamicMessageValidation(t *testing.T) {
	_ = dynamicpb.NewMessage((&testv1.TestMessage{}).ProtoReflect().Descriptor())
}
