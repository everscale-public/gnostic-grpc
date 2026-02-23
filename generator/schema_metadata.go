package generator

import (
	openapiv3 "github.com/google/gnostic/openapiv3"
)

// SchemaInfo holds required and nullable metadata for a single component schema.
type SchemaInfo struct {
	requiredFields map[string]bool
	nullableFields map[string]bool
}

// SchemaMetadata extracts and stores required/nullable information from an OpenAPI document
// so that the proto generator can emit proto3 optional for qualifying fields.
type SchemaMetadata struct {
	schemas map[string]*SchemaInfo
}

// NewSchemaMetadata walks the components/schemas of an openapiv3.Document and
// extracts the required array and per-property nullable flag for each schema.
func NewSchemaMetadata(doc *openapiv3.Document) *SchemaMetadata {
	sm := &SchemaMetadata{
		schemas: make(map[string]*SchemaInfo),
	}
	if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return sm
	}
	for _, pair := range doc.Components.Schemas.AdditionalProperties {
		schema := pair.Value.GetSchema()
		if schema == nil {
			continue
		}
		info := &SchemaInfo{
			requiredFields: make(map[string]bool),
			nullableFields: make(map[string]bool),
		}
		for _, r := range schema.Required {
			info.requiredFields[r] = true
		}
		if schema.Properties != nil {
			for _, propPair := range schema.Properties.AdditionalProperties {
				propSchema := propPair.Value.GetSchema()
				if propSchema != nil && propSchema.Nullable {
					info.nullableFields[propPair.Name] = true
				}
			}
		}
		sm.schemas[pair.Name] = info
	}
	return sm
}

// ShouldBeOptional returns true if the field should be emitted as proto3 optional.
// A field qualifies if it is nullable OR not listed in the schema's required array.
// Returns false for unknown types (conservative default).
func (sm *SchemaMetadata) ShouldBeOptional(typeName, fieldName string) bool {
	if sm == nil {
		return false
	}
	info, ok := sm.schemas[typeName]
	if !ok {
		return false
	}
	if info.nullableFields[fieldName] {
		return true
	}
	if !info.requiredFields[fieldName] {
		return true
	}
	return false
}
