package generator

import (
	"testing"

	"github.com/golang/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func TestProcessProto3OptionalFields(t *testing.T) {
	name1, name2, name3 := "field_a", "field_b", "field_c"
	var n1, n2, n3 int32 = 1, 2, 3
	tStr := dpb.FieldDescriptorProto_TYPE_STRING
	tInt := dpb.FieldDescriptorProto_TYPE_INT32
	label := dpb.FieldDescriptorProto_LABEL_OPTIONAL

	msg := &dpb.DescriptorProto{
		Name: proto.String("TestMessage"),
		Field: []*dpb.FieldDescriptorProto{
			{Name: &name1, Number: &n1, Type: &tStr, Label: &label, Proto3Optional: proto.Bool(true)},
			{Name: &name2, Number: &n2, Type: &tInt, Label: &label},
			{Name: &name3, Number: &n3, Type: &tStr, Label: &label, Proto3Optional: proto.Bool(true)},
		},
	}

	processProto3OptionalFields(msg)

	// Should have created 2 synthetic oneofs (for field_a and field_c)
	if len(msg.OneofDecl) != 2 {
		t.Fatalf("expected 2 oneof declarations, got %d", len(msg.OneofDecl))
	}

	// First oneof should be _field_a
	if msg.OneofDecl[0].GetName() != "_field_a" {
		t.Errorf("expected oneof name '_field_a', got '%s'", msg.OneofDecl[0].GetName())
	}
	// Second oneof should be _field_c
	if msg.OneofDecl[1].GetName() != "_field_c" {
		t.Errorf("expected oneof name '_field_c', got '%s'", msg.OneofDecl[1].GetName())
	}

	// field_a should have OneofIndex = 0
	if msg.Field[0].OneofIndex == nil || *msg.Field[0].OneofIndex != 0 {
		t.Errorf("expected field_a OneofIndex=0, got %v", msg.Field[0].OneofIndex)
	}
	// field_b should have no OneofIndex
	if msg.Field[1].OneofIndex != nil {
		t.Errorf("expected field_b OneofIndex=nil, got %v", *msg.Field[1].OneofIndex)
	}
	// field_c should have OneofIndex = 1
	if msg.Field[2].OneofIndex == nil || *msg.Field[2].OneofIndex != 1 {
		t.Errorf("expected field_c OneofIndex=1, got %v", msg.Field[2].OneofIndex)
	}
}

func TestProcessProto3OptionalFieldsNameCollision(t *testing.T) {
	name1 := "field_x"
	existingOneof := "_field_x" // Simulates an existing name that would collide
	var n1 int32 = 1
	tStr := dpb.FieldDescriptorProto_TYPE_STRING
	label := dpb.FieldDescriptorProto_LABEL_OPTIONAL

	msg := &dpb.DescriptorProto{
		Name: proto.String("TestMessage"),
		Field: []*dpb.FieldDescriptorProto{
			{Name: &name1, Number: &n1, Type: &tStr, Label: &label, Proto3Optional: proto.Bool(true)},
		},
		OneofDecl: []*dpb.OneofDescriptorProto{
			{Name: &existingOneof},
		},
	}

	processProto3OptionalFields(msg)

	// Should have added a new oneof with "X" prefix to avoid collision
	if len(msg.OneofDecl) != 2 {
		t.Fatalf("expected 2 oneof declarations, got %d", len(msg.OneofDecl))
	}
	if msg.OneofDecl[1].GetName() != "X_field_x" {
		t.Errorf("expected oneof name 'X_field_x' to avoid collision, got '%s'", msg.OneofDecl[1].GetName())
	}
	// field_x should point to the new oneof at index 1
	if msg.Field[0].OneofIndex == nil || *msg.Field[0].OneofIndex != 1 {
		t.Errorf("expected field_x OneofIndex=1, got %v", msg.Field[0].OneofIndex)
	}
}

func TestProcessProto3OptionalFieldsNoOptional(t *testing.T) {
	name1 := "field_a"
	var n1 int32 = 1
	tStr := dpb.FieldDescriptorProto_TYPE_STRING
	label := dpb.FieldDescriptorProto_LABEL_OPTIONAL

	msg := &dpb.DescriptorProto{
		Name: proto.String("TestMessage"),
		Field: []*dpb.FieldDescriptorProto{
			{Name: &name1, Number: &n1, Type: &tStr, Label: &label},
		},
	}

	processProto3OptionalFields(msg)

	if len(msg.OneofDecl) != 0 {
		t.Errorf("expected 0 oneof declarations when no optional fields, got %d", len(msg.OneofDecl))
	}
}
