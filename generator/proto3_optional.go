package generator

import (
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// processProto3OptionalFields replicates the behavior of protoc for proto3 optional fields.
// For each field with Proto3Optional=true, it creates a synthetic OneofDescriptorProto
// and sets the OneofIndex on the field. This is required for protoreflect to correctly
// render the "optional" keyword.
func processProto3OptionalFields(msgd *dpb.DescriptorProto) {
	allNames := collectAllNames(msgd)
	for _, fd := range msgd.Field {
		if fd.Proto3Optional == nil || !*fd.Proto3Optional {
			continue
		}
		oneofName := "_" + fd.GetName()
		// Ensure uniqueness by prepending "X" if the name collides.
		for allNames[oneofName] {
			oneofName = "X" + oneofName
		}
		allNames[oneofName] = true

		msgd.OneofDecl = append(msgd.OneofDecl, &dpb.OneofDescriptorProto{
			Name: &oneofName,
		})
		idx := int32(len(msgd.OneofDecl) - 1)
		fd.OneofIndex = &idx
	}
}

// collectAllNames gathers all names used within a message descriptor
// (fields, oneofs, nested messages, enums) to avoid collisions when
// generating synthetic oneof names.
func collectAllNames(msgd *dpb.DescriptorProto) map[string]bool {
	names := make(map[string]bool)
	for _, fd := range msgd.Field {
		names[fd.GetName()] = true
	}
	for _, od := range msgd.OneofDecl {
		names[od.GetName()] = true
	}
	for _, nd := range msgd.NestedType {
		names[nd.GetName()] = true
	}
	for _, ed := range msgd.EnumType {
		names[ed.GetName()] = true
	}
	return names
}
