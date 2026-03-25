package main

import (
	"strings"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	formv1 "github.com/akhenakh/grpc-form/gen/form/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func buildUISchema(msg protoreflect.MessageDescriptor) map[string]any {
	title, desc := getMessageOptions(msg)
	schema := map[string]any{
		"title":       title,
		"description": desc,
		"fields":      []map[string]any{},
		"enums":       map[string]any{},
		"messages":    map[string]any{},
	}

	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		schema["fields"] = append(schema["fields"].([]map[string]any), buildFieldSchema(schema, field))
	}

	return schema
}

func getMessageOptions(msg protoreflect.MessageDescriptor) (title, description string) {
	opts := msg.Options().(*descriptorpb.MessageOptions)
	if opts == nil {
		return
	}
	title, _ = proto.GetExtension(opts, formv1.E_Title).(string)
	description, _ = proto.GetExtension(opts, formv1.E_Description).(string)
	return
}

func buildFieldSchema(rootSchema map[string]any, field protoreflect.FieldDescriptor) map[string]any {
	fType := field.Kind().String()

	if field.Kind() == protoreflect.MessageKind {
		fType = string(field.Message().FullName())
		populateMessages(rootSchema, field.Message())
	} else if field.Kind() == protoreflect.EnumKind {
		fType = string(field.Enum().FullName())
		populateEnums(rootSchema, field.Enum())
	}

	label := cases.Title(language.Und).String(strings.ReplaceAll(string(field.Name()), "_", " "))
	hidden := false
	placeholder := ""
	hint := ""

	if opts := field.Options().(*descriptorpb.FieldOptions); opts != nil {
		if ext := proto.GetExtension(opts, formv1.E_Field); ext != nil {
			if fo, ok := ext.(*formv1.FieldOptions); ok {
				if fo.Label != "" {
					label = fo.Label
				}
				hidden = fo.Hidden
				placeholder = fo.Placeholder
				hint = fo.Hint
			}
		}
	}

	validateRules := extractValidationRules(field)

	fSchema := map[string]any{
		"name":        string(field.Name()),
		"type":        fType,
		"repeated":    field.IsList(),
		"label":       label,
		"hidden":      hidden,
		"placeholder": placeholder,
		"hint":        hint,
		"validate":    validateRules,
		"isEnum":      field.Kind() == protoreflect.EnumKind,
		"isMessage":   field.Kind() == protoreflect.MessageKind,
	}

	if field.ContainingOneof() != nil {
		fSchema["oneofGroup"] = string(field.ContainingOneof().Name())
	}

	return fSchema
}

func extractValidationRules(field protoreflect.FieldDescriptor) map[string]any {
	rules := make(map[string]any)

	fieldRules, err := protovalidate.ResolveFieldRules(field)
	if err != nil || fieldRules == nil {
		return rules
	}

	if fieldRules.GetRequired() {
		rules["required"] = true
	}

	switch r := fieldRules.GetType().(type) {
	case *validate.FieldRules_String_:
		sr := r.String_
		if sr != nil {
			if sr.GetMinLen() > 0 {
				rules["minLen"] = int(sr.GetMinLen())
			}
			if sr.GetMaxLen() > 0 {
				rules["maxLen"] = int(sr.GetMaxLen())
			}
			if sr.GetEmail() {
				rules["email"] = true
			}
			if sr.GetPattern() != "" {
				rules["pattern"] = sr.GetPattern()
			}
			if sr.GetMinBytes() > 0 {
				rules["minBytes"] = int(sr.GetMinBytes())
			}
			if sr.GetMaxBytes() > 0 {
				rules["maxBytes"] = int(sr.GetMaxBytes())
			}
		}
	case *validate.FieldRules_Int64:
		ir := r.Int64
		if ir != nil {
			if ir.GetConst() != 0 || ir.HasGt() || ir.HasGte() || ir.HasLt() || ir.HasLte() {
				if ir.HasGte() {
					rules["min"] = int(ir.GetGte())
				}
				if ir.HasLte() {
					rules["max"] = int(ir.GetLte())
				}
			}
		}
	case *validate.FieldRules_Int32:
		ir := r.Int32
		if ir != nil {
			if ir.HasGte() {
				rules["min"] = int(ir.GetGte())
			}
			if ir.HasLte() {
				rules["max"] = int(ir.GetLte())
			}
		}
	case *validate.FieldRules_Uint64:
		ur := r.Uint64
		if ur != nil {
			if ur.HasGte() {
				rules["min"] = int(ur.GetGte())
			}
			if ur.HasLte() {
				rules["max"] = int(ur.GetLte())
			}
		}
	case *validate.FieldRules_Uint32:
		ur := r.Uint32
		if ur != nil {
			if ur.HasGte() {
				rules["min"] = int(ur.GetGte())
			}
			if ur.HasLte() {
				rules["max"] = int(ur.GetLte())
			}
		}
	case *validate.FieldRules_Float:
		fr := r.Float
		if fr != nil {
			if fr.HasGte() {
				rules["min"] = fr.GetGte()
			}
			if fr.HasLte() {
				rules["max"] = fr.GetLte()
			}
		}
	case *validate.FieldRules_Double:
		dr := r.Double
		if dr != nil {
			if dr.HasGte() {
				rules["min"] = dr.GetGte()
			}
			if dr.HasLte() {
				rules["max"] = dr.GetLte()
			}
		}
	}

	return rules
}

func populateMessages(rootSchema map[string]any, msg protoreflect.MessageDescriptor) {
	msgName := string(msg.FullName())
	messagesMap := rootSchema["messages"].(map[string]any)

	if _, exists := messagesMap[msgName]; exists {
		return
	}
	messagesMap[msgName] = []map[string]any{}

	subFields := []map[string]any{}
	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		subFields = append(subFields, buildFieldSchema(rootSchema, fields.Get(i)))
	}
	messagesMap[msgName] = subFields
}

func populateEnums(rootSchema map[string]any, enum protoreflect.EnumDescriptor) {
	enumName := string(enum.FullName())
	enumsMap := rootSchema["enums"].(map[string]any)

	if _, exists := enumsMap[enumName]; exists {
		return
	}

	vals := []map[string]any{}
	enumValues := enum.Values()
	for i := 0; i < enumValues.Len(); i++ {
		v := enumValues.Get(i)
		vals = append(vals, map[string]any{
			"name":   string(v.Name()),
			"number": int32(v.Number()),
		})
	}
	enumsMap[enumName] = vals
}
