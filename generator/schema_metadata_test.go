package generator

import (
	"testing"

	openapiv3 "github.com/google/gnostic/openapiv3"
)

func TestNewSchemaMetadataNilDoc(t *testing.T) {
	sm := NewSchemaMetadata(nil)
	if sm == nil {
		t.Fatal("expected non-nil SchemaMetadata for nil doc")
	}
	if sm.ShouldBeOptional("Anything", "field") {
		t.Error("expected false for nil doc")
	}
}

func TestNewSchemaMetadataEmptyComponents(t *testing.T) {
	doc := &openapiv3.Document{}
	sm := NewSchemaMetadata(doc)
	if sm.ShouldBeOptional("Anything", "field") {
		t.Error("expected false for empty components")
	}
}

func TestShouldBeOptionalUnknownType(t *testing.T) {
	doc := &openapiv3.Document{
		Components: &openapiv3.Components{
			Schemas: &openapiv3.SchemasOrReferences{
				AdditionalProperties: []*openapiv3.NamedSchemaOrReference{
					{
						Name: "Known",
						Value: &openapiv3.SchemaOrReference{
							Oneof: &openapiv3.SchemaOrReference_Schema{
								Schema: &openapiv3.Schema{
									Required: []string{"a"},
								},
							},
						},
					},
				},
			},
		},
	}
	sm := NewSchemaMetadata(doc)
	if sm.ShouldBeOptional("Unknown", "a") {
		t.Error("expected false for unknown type")
	}
}

func TestShouldBeOptionalRequired(t *testing.T) {
	doc := &openapiv3.Document{
		Components: &openapiv3.Components{
			Schemas: &openapiv3.SchemasOrReferences{
				AdditionalProperties: []*openapiv3.NamedSchemaOrReference{
					{
						Name: "Person",
						Value: &openapiv3.SchemaOrReference{
							Oneof: &openapiv3.SchemaOrReference_Schema{
								Schema: &openapiv3.Schema{
									Required: []string{"name", "age"},
									Properties: &openapiv3.Properties{
										AdditionalProperties: []*openapiv3.NamedSchemaOrReference{
											{Name: "name", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{}}}},
											{Name: "age", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{}}}},
											{Name: "email", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{}}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	sm := NewSchemaMetadata(doc)

	// Required fields should NOT be optional
	if sm.ShouldBeOptional("Person", "name") {
		t.Error("expected required field 'name' to not be optional")
	}
	if sm.ShouldBeOptional("Person", "age") {
		t.Error("expected required field 'age' to not be optional")
	}
	// Non-required fields SHOULD be optional
	if !sm.ShouldBeOptional("Person", "email") {
		t.Error("expected non-required field 'email' to be optional")
	}
}

func TestShouldBeOptionalNullable(t *testing.T) {
	doc := &openapiv3.Document{
		Components: &openapiv3.Components{
			Schemas: &openapiv3.SchemasOrReferences{
				AdditionalProperties: []*openapiv3.NamedSchemaOrReference{
					{
						Name: "Item",
						Value: &openapiv3.SchemaOrReference{
							Oneof: &openapiv3.SchemaOrReference_Schema{
								Schema: &openapiv3.Schema{
									Required: []string{"id", "nullableRequired"},
									Properties: &openapiv3.Properties{
										AdditionalProperties: []*openapiv3.NamedSchemaOrReference{
											{Name: "id", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{}}}},
											{Name: "nullableRequired", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{Nullable: true}}}},
											{Name: "nullableNotRequired", Value: &openapiv3.SchemaOrReference{Oneof: &openapiv3.SchemaOrReference_Schema{Schema: &openapiv3.Schema{Nullable: true}}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	sm := NewSchemaMetadata(doc)

	// Required + not nullable => not optional
	if sm.ShouldBeOptional("Item", "id") {
		t.Error("expected required non-nullable field 'id' to not be optional")
	}
	// Required + nullable => optional
	if !sm.ShouldBeOptional("Item", "nullableRequired") {
		t.Error("expected required but nullable field to be optional")
	}
	// Not required + nullable => optional
	if !sm.ShouldBeOptional("Item", "nullableNotRequired") {
		t.Error("expected non-required nullable field to be optional")
	}
}

func TestNilSchemaMetadataShouldBeOptional(t *testing.T) {
	var sm *SchemaMetadata
	if sm.ShouldBeOptional("Anything", "field") {
		t.Error("expected false for nil SchemaMetadata receiver")
	}
}
