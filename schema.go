package main

import (
	"strings"

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

	fSchema := map[string]any{
		"name":        string(field.Name()),
		"type":        fType,
		"repeated":    field.IsList(),
		"label":       label,
		"hidden":      hidden,
		"placeholder": placeholder,
		"hint":        hint,
		"validate":    map[string]any{},
		"isEnum":      field.Kind() == protoreflect.EnumKind,
		"isMessage":   field.Kind() == protoreflect.MessageKind,
	}

	if field.ContainingOneof() != nil {
		fSchema["oneofGroup"] = string(field.ContainingOneof().Name())
	}

	return fSchema
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
