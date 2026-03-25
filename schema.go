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

	if opts := field.Options(); opts != nil {
		if ext := proto.GetExtension(opts, formv1.E_Field); ext != nil {
			if fo, ok := ext.(*formv1.FieldOptions); ok && fo != nil {
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
		extractStringRules(rules, r.String_)
	case *validate.FieldRules_Bytes:
		extractBytesRules(rules, r.Bytes)
	case *validate.FieldRules_Int32:
		extractIntRules(rules, r.Int32, "int32")
	case *validate.FieldRules_Int64:
		extractIntRules(rules, r.Int64, "int64")
	case *validate.FieldRules_Uint32:
		extractIntRules(rules, r.Uint32, "uint32")
	case *validate.FieldRules_Uint64:
		extractIntRules(rules, r.Uint64, "uint64")
	case *validate.FieldRules_Sint32:
		extractIntRules(rules, r.Sint32, "sint32")
	case *validate.FieldRules_Sint64:
		extractIntRules(rules, r.Sint64, "sint64")
	case *validate.FieldRules_Fixed32:
		extractIntRules(rules, r.Fixed32, "fixed32")
	case *validate.FieldRules_Fixed64:
		extractIntRules(rules, r.Fixed64, "fixed64")
	case *validate.FieldRules_Sfixed32:
		extractIntRules(rules, r.Sfixed32, "sfixed32")
	case *validate.FieldRules_Sfixed64:
		extractIntRules(rules, r.Sfixed64, "sfixed64")
	case *validate.FieldRules_Float:
		extractFloatRules(rules, r.Float)
	case *validate.FieldRules_Double:
		extractDoubleRules(rules, r.Double)
	case *validate.FieldRules_Bool:
		extractBoolRules(rules, r.Bool)
	case *validate.FieldRules_Enum:
		extractEnumRules(rules, r.Enum)
	case *validate.FieldRules_Repeated:
		extractRepeatedRules(rules, r.Repeated)
	case *validate.FieldRules_Map:
		extractMapRules(rules, r.Map)
	case *validate.FieldRules_Duration:
		extractDurationRules(rules, r.Duration)
	case *validate.FieldRules_Timestamp:
		extractTimestampRules(rules, r.Timestamp)
	case *validate.FieldRules_Any:
		extractAnyRules(rules, r.Any)
	}

	return rules
}

func extractStringRules(rules map[string]any, sr *validate.StringRules) {
	if sr == nil {
		return
	}

	if sr.GetMinLen() > 0 {
		rules["minLen"] = int(sr.GetMinLen())
	}
	if sr.GetMaxLen() > 0 {
		rules["maxLen"] = int(sr.GetMaxLen())
	}
	if sr.GetLen() > 0 {
		rules["len"] = int(sr.GetLen())
	}
	if sr.GetMinBytes() > 0 {
		rules["minBytes"] = int(sr.GetMinBytes())
	}
	if sr.GetMaxBytes() > 0 {
		rules["maxBytes"] = int(sr.GetMaxBytes())
	}
	if sr.GetLenBytes() > 0 {
		rules["lenBytes"] = int(sr.GetLenBytes())
	}
	if sr.GetPattern() != "" {
		rules["pattern"] = sr.GetPattern()
	}
	if sr.GetPrefix() != "" {
		rules["prefix"] = sr.GetPrefix()
	}
	if sr.GetSuffix() != "" {
		rules["suffix"] = sr.GetSuffix()
	}
	if sr.GetContains() != "" {
		rules["contains"] = sr.GetContains()
	}
	if sr.GetNotContains() != "" {
		rules["notContains"] = sr.GetNotContains()
	}
	if len(sr.GetIn()) > 0 {
		rules["in"] = sr.GetIn()
	}
	if len(sr.GetNotIn()) > 0 {
		rules["notIn"] = sr.GetNotIn()
	}
	if sr.GetConst() != "" {
		rules["const"] = sr.GetConst()
	}
	if sr.GetEmail() {
		rules["email"] = true
	}
	if sr.GetUuid() {
		rules["uuid"] = true
	}
	if sr.GetTuuid() {
		rules["tuuid"] = true
	}
	if sr.GetHostname() {
		rules["hostname"] = true
	}
	if sr.GetUri() {
		rules["uri"] = true
	}
	if sr.GetUriRef() {
		rules["uriRef"] = true
	}
	if sr.GetIp() {
		rules["ip"] = true
	}
	if sr.GetIpv4() {
		rules["ipv4"] = true
	}
	if sr.GetIpv6() {
		rules["ipv6"] = true
	}
	if sr.GetIpWithPrefixlen() {
		rules["ipWithPrefixlen"] = true
	}
	if sr.GetIpPrefix() {
		rules["ipPrefix"] = true
	}
	if sr.GetAddress() {
		rules["address"] = true
	}
	if sr.GetHostAndPort() {
		rules["hostAndPort"] = true
	}
	if sr.GetWellKnownRegex() != validate.KnownRegex_KNOWN_REGEX_UNSPECIFIED {
		rules["wellKnownRegex"] = sr.GetWellKnownRegex().String()
	}
	if sr.GetStrict() {
		rules["strict"] = true
	}
}

func extractBytesRules(rules map[string]any, br *validate.BytesRules) {
	if br == nil {
		return
	}

	if br.GetMinLen() > 0 {
		rules["minLen"] = int(br.GetMinLen())
	}
	if br.GetMaxLen() > 0 {
		rules["maxLen"] = int(br.GetMaxLen())
	}
	if br.GetLen() > 0 {
		rules["len"] = int(br.GetLen())
	}
	if br.GetPattern() != "" {
		rules["pattern"] = br.GetPattern()
	}
	if len(br.GetPrefix()) > 0 {
		rules["prefix"] = string(br.GetPrefix())
	}
	if len(br.GetSuffix()) > 0 {
		rules["suffix"] = string(br.GetSuffix())
	}
	if len(br.GetContains()) > 0 {
		rules["contains"] = string(br.GetContains())
	}
	if len(br.GetIn()) > 0 {
		inStrs := make([]string, len(br.GetIn()))
		for i, b := range br.GetIn() {
			inStrs[i] = string(b)
		}
		rules["in"] = inStrs
	}
	if len(br.GetNotIn()) > 0 {
		notInStrs := make([]string, len(br.GetNotIn()))
		for i, b := range br.GetNotIn() {
			notInStrs[i] = string(b)
		}
		rules["notIn"] = notInStrs
	}
	if len(br.GetConst()) > 0 {
		rules["const"] = string(br.GetConst())
	}
	if br.GetIp() {
		rules["ip"] = true
	}
	if br.GetIpv4() {
		rules["ipv4"] = true
	}
	if br.GetIpv6() {
		rules["ipv6"] = true
	}
}

func extractIntRules[T ~int32 | ~int64 | ~uint32 | ~uint64](rules map[string]any, ir interface {
	GetConst() T
	HasGt() bool
	GetGt() T
	HasGte() bool
	GetGte() T
	HasLt() bool
	GetLt() T
	HasLte() bool
	GetLte() T
	GetIn() []T
	GetNotIn() []T
}, _ string) {
	if ir == nil {
		return
	}

	if ir.GetConst() != 0 {
		rules["const"] = int(ir.GetConst())
	}
	if ir.HasGt() {
		rules["gt"] = int(ir.GetGt())
	}
	if ir.HasGte() {
		rules["gte"] = int(ir.GetGte())
		rules["min"] = int(ir.GetGte())
	}
	if ir.HasLt() {
		rules["lt"] = int(ir.GetLt())
	}
	if ir.HasLte() {
		rules["lte"] = int(ir.GetLte())
		rules["max"] = int(ir.GetLte())
	}
	if len(ir.GetIn()) > 0 {
		inInts := make([]int, len(ir.GetIn()))
		for i, v := range ir.GetIn() {
			inInts[i] = int(v)
		}
		rules["in"] = inInts
	}
	if len(ir.GetNotIn()) > 0 {
		notInInts := make([]int, len(ir.GetNotIn()))
		for i, v := range ir.GetNotIn() {
			notInInts[i] = int(v)
		}
		rules["notIn"] = notInInts
	}
}

func extractFloatRules(rules map[string]any, fr *validate.FloatRules) {
	if fr == nil {
		return
	}

	if fr.HasGt() {
		rules["gt"] = float64(fr.GetGt())
	}
	if fr.HasGte() {
		rules["gte"] = float64(fr.GetGte())
		rules["min"] = float64(fr.GetGte())
	}
	if fr.HasLt() {
		rules["lt"] = float64(fr.GetLt())
	}
	if fr.HasLte() {
		rules["lte"] = float64(fr.GetLte())
		rules["max"] = float64(fr.GetLte())
	}
	if fr.GetFinite() {
		rules["finite"] = true
	}
}

func extractDoubleRules(rules map[string]any, dr *validate.DoubleRules) {
	if dr == nil {
		return
	}

	if dr.HasGt() {
		rules["gt"] = dr.GetGt()
	}
	if dr.HasGte() {
		rules["gte"] = dr.GetGte()
		rules["min"] = dr.GetGte()
	}
	if dr.HasLt() {
		rules["lt"] = dr.GetLt()
	}
	if dr.HasLte() {
		rules["lte"] = dr.GetLte()
		rules["max"] = dr.GetLte()
	}
	if dr.GetFinite() {
		rules["finite"] = true
	}
}

func extractBoolRules(rules map[string]any, br *validate.BoolRules) {
	if br == nil {
		return
	}
	if br.GetConst() {
		rules["const"] = true
	}
}

func extractEnumRules(rules map[string]any, er *validate.EnumRules) {
	if er == nil {
		return
	}

	if er.GetDefinedOnly() {
		rules["definedOnly"] = true
	}
	if len(er.GetIn()) > 0 {
		rules["in"] = er.GetIn()
	}
	if len(er.GetNotIn()) > 0 {
		rules["notIn"] = er.GetNotIn()
	}
	if er.GetConst() != 0 {
		rules["const"] = int(er.GetConst())
	}
}

func extractRepeatedRules(rules map[string]any, rr *validate.RepeatedRules) {
	if rr == nil {
		return
	}

	if rr.GetMinItems() > 0 {
		rules["minItems"] = int(rr.GetMinItems())
	}
	if rr.GetMaxItems() > 0 {
		rules["maxItems"] = int(rr.GetMaxItems())
	}
	if rr.GetUnique() {
		rules["unique"] = true
	}
	if rr.GetItems() != nil {
		itemRules := make(map[string]any)
		extractFieldRulesFromItems(itemRules, rr.GetItems())
		if len(itemRules) > 0 {
			rules["items"] = itemRules
		}
	}
}

func extractFieldRulesFromItems(rules map[string]any, items *validate.FieldRules) {
	if items == nil {
		return
	}
	switch t := items.GetType().(type) {
	case *validate.FieldRules_String_:
		extractStringRules(rules, t.String_)
	case *validate.FieldRules_Int32:
		extractIntRules(rules, t.Int32, "int32")
	case *validate.FieldRules_Int64:
		extractIntRules(rules, t.Int64, "int64")
	case *validate.FieldRules_Uint32:
		extractIntRules(rules, t.Uint32, "uint32")
	case *validate.FieldRules_Uint64:
		extractIntRules(rules, t.Uint64, "uint64")
	}
}

func extractMapRules(rules map[string]any, mr *validate.MapRules) {
	if mr == nil {
		return
	}

	if mr.GetMinPairs() > 0 {
		rules["minPairs"] = int(mr.GetMinPairs())
	}
	if mr.GetMaxPairs() > 0 {
		rules["maxPairs"] = int(mr.GetMaxPairs())
	}
	if mr.GetKeys() != nil {
		keyRules := make(map[string]any)
		extractFieldRulesFromItems(keyRules, mr.GetKeys())
		if len(keyRules) > 0 {
			rules["keys"] = keyRules
		}
	}
	if mr.GetValues() != nil {
		valueRules := make(map[string]any)
		extractFieldRulesFromItems(valueRules, mr.GetValues())
		if len(valueRules) > 0 {
			rules["values"] = valueRules
		}
	}
}

func extractDurationRules(rules map[string]any, dr *validate.DurationRules) {
	if dr == nil {
		return
	}

	if dr.HasGt() {
		rules["gt"] = dr.GetGt().String()
	}
	if dr.HasGte() {
		rules["gte"] = dr.GetGte().String()
	}
	if dr.HasLt() {
		rules["lt"] = dr.GetLt().String()
	}
	if dr.HasLte() {
		rules["lte"] = dr.GetLte().String()
	}
	if len(dr.GetIn()) > 0 {
		inDurations := make([]string, len(dr.GetIn()))
		for i, d := range dr.GetIn() {
			inDurations[i] = d.String()
		}
		rules["in"] = inDurations
	}
	if len(dr.GetNotIn()) > 0 {
		notInDurations := make([]string, len(dr.GetNotIn()))
		for i, d := range dr.GetNotIn() {
			notInDurations[i] = d.String()
		}
		rules["notIn"] = notInDurations
	}
	if dr.GetConst() != nil {
		rules["const"] = dr.GetConst().String()
	}
}

func extractTimestampRules(rules map[string]any, tr *validate.TimestampRules) {
	if tr == nil {
		return
	}

	if tr.HasGt() {
		rules["gt"] = tr.GetGt().String()
	}
	if tr.HasGte() {
		rules["gte"] = tr.GetGte().String()
	}
	if tr.HasLt() {
		rules["lt"] = tr.GetLt().String()
	}
	if tr.HasLte() {
		rules["lte"] = tr.GetLte().String()
	}
	if tr.GetLtNow() {
		rules["ltNow"] = true
	}
	if tr.GetGtNow() {
		rules["gtNow"] = true
	}
	if tr.GetWithin() != nil {
		rules["within"] = tr.GetWithin().String()
	}
	if tr.GetConst() != nil {
		rules["const"] = tr.GetConst().String()
	}
}

func extractAnyRules(rules map[string]any, ar *validate.AnyRules) {
	if ar == nil {
		return
	}

	if len(ar.GetIn()) > 0 {
		rules["in"] = ar.GetIn()
	}
	if len(ar.GetNotIn()) > 0 {
		rules["notIn"] = ar.GetNotIn()
	}
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
