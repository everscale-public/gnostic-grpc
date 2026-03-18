// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang/protobuf/descriptor"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/ptypes/empty"
	openapiv3 "github.com/google/gnostic/openapiv3"
	surface_v1 "github.com/google/gnostic/surface"
	"google.golang.org/genproto/googleapis/api/annotations"

	"github.com/everscale-public/gnostic-grpc/utils"
)

// Gathers all symbolic references we generated in recursive calls.
var generatedSymbolicReferences = make(map[string]bool, 0)

// Uses the output of gnostic to return a dpb.FileDescriptorSet (in bytes). 'renderer' contains
// the 'model' (surface model) which has all the relevant data to create the dpb.FileDescriptorSet.
// There are four main steps:
//  1. buildSymbolicReferences 	recursively executes this plugin to generate all FileDescriptorSet based on symbolic
//     references. A symbolic reference is a URL to another OpenAPI description inside the
//     current description.
//  2. buildDependencies to build all static FileDescriptorProto we need.
//  3. buildAllMessageDescriptors is called to create all messages which will be rendered in .proto
//  4. buildAllServiceDescriptors is called to create an RPC service which will be rendered in .proto
func (renderer *Renderer) runFileDescriptorSetGenerator() (fdSet *dpb.FileDescriptorSet, err error) {
	syntax := "proto3"
	n := renderer.Package + ".proto"

	protoToBeRendered := &dpb.FileDescriptorProto{
		Name:    &n,
		Package: &renderer.Package,
		Syntax:  &syntax,
	}

	symbolicReferenceDependencies, err := buildSymbolicReferences(renderer)
	if err != nil {
		return nil, err
	}
	dependencies := buildDependencies()
	dependencies = append(dependencies, symbolicReferenceDependencies...)
	dependencyNames := getNamesOfDependenciesThatWillBeImported(dependencies, renderer.Model.Methods)
	protoToBeRendered.Dependency = dependencyNames

	allMessages, err := buildAllMessageDescriptors(renderer)
	if err != nil {
		return nil, err
	}
	protoToBeRendered.MessageType = allMessages

	allServices, err := buildAllServiceDescriptors(protoToBeRendered.MessageType, renderer)
	if err != nil {
		return nil, err
	}
	protoToBeRendered.Service = allServices

	sourceCodeInfo, err := buildSourceCodeInfo(renderer.Model.Types)
	if err != nil {
		return nil, err
	}
	protoToBeRendered.SourceCodeInfo = sourceCodeInfo

	fileOptions := renderer.buildFileOptions()
	protoToBeRendered.Options = fileOptions

	allFileDescriptors := append(symbolicReferenceDependencies, dependencies...)
	allFileDescriptors = append(allFileDescriptors, protoToBeRendered)
	fdSet = &dpb.FileDescriptorSet{
		File: allFileDescriptors,
	}

	return fdSet, err
}

// buildSourceCodeInfo builds the object which holds additional information, such as the description from OpenAPI
// components. This information will be rendered as a comment in the final .proto file.
func buildSourceCodeInfo(types []*surface_v1.Type) (sourceCodeInfo *dpb.SourceCodeInfo, err error) {
	allLocations := make([]*dpb.SourceCodeInfo_Location, 0)
	for idx, surfaceType := range types {
		location := &dpb.SourceCodeInfo_Location{
			Path:            []int32{4, int32(idx)},
			LeadingComments: &surfaceType.Description,
		}
		allLocations = append(allLocations, location)
	}
	sourceCodeInfo = &dpb.SourceCodeInfo{
		Location: allLocations,
	}
	return sourceCodeInfo, nil
}

// resolveRef returns the ref as-is for HTTP(S) URLs, or resolves it to a clean absolute
// file path for local file references so that gnostic can find the file.
// sourceFile is the absolute path of the OpenAPI file that contains the $ref.
func resolveRef(ref string, sourceFile string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref)
	}
	if sourceFile != "" {
		return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), ref))
	}
	abs, err := filepath.Abs(ref)
	if err != nil {
		return ref
	}
	return filepath.Clean(abs)
}

// buildSymbolicReferences recursively generates all .proto definitions to external OpenAPI descriptions (URLs or local
// file paths to other descriptions inside the current description).
func buildSymbolicReferences(renderer *Renderer) (symbolicFileDescriptors []*dpb.FileDescriptorProto, err error) {
	symbolicReferences := renderer.Model.SymbolicReferences
	symbolicReferences = trimAndRemoveDuplicates(symbolicReferences)

	for _, ref := range symbolicReferences {
		resolvedRef := resolveRef(ref, renderer.SourceFile)

		// Skip self-references: gnostic's compiler cache can include the source file itself.
		if resolvedRef == renderer.SourceFile {
			continue
		}

		// Deduplicate by resolved path so that "./common.yaml" and "/abs/path/to/common.yaml"
		// are recognized as the same file.
		if _, alreadyGenerated := generatedSymbolicReferences[resolvedRef]; !alreadyGenerated {
			generatedSymbolicReferences[resolvedRef] = true

			// Lets get the standard gnostic output from the symbolic reference.
			cmd := exec.Command("gnostic", "--pb-out=-", resolvedRef)
			b, err := cmd.Output()
			if err != nil {
				return nil, err
			}

			// Construct an OpenAPI document v3.
			document, err := utils.CreateOpenAPIDocFromGnosticOutput(b)
			if err != nil {
				return nil, err
			}

			// Create the surface model. Keep in mind that this resolves the references of the symbolic reference again!
			surfaceModel, err := surface_v1.NewModelFromOpenAPI3(document, ref)
			if err != nil {
				return nil, err
			}

			// Prepare surface model for recursive call. TODO: Keep discovery documents in mind.
			inputDocumentType := "openapi.v3.Document"
			if document.Openapi == "2.0.0" {
				inputDocumentType = "openapi.v2.Document"
			}
			NewProtoLanguageModel().Prepare(surfaceModel, inputDocumentType)

			// Recursively call the generator.
			externalMetadata := NewSchemaMetadata(document)
			recursiveRenderer := NewRenderer(surfaceModel, externalMetadata)
			recursiveRenderer.SourceFile = resolvedRef
			fileName := path.Base(ref)
			recursiveRenderer.Package = strings.TrimSuffix(fileName, filepath.Ext(fileName))
			newFdSet, err := recursiveRenderer.runFileDescriptorSetGenerator()
			if err != nil {
				return nil, err
			}
			renderer.SymbolicFdSets = append(renderer.SymbolicFdSets, newFdSet)

			symbolicProto := getLast(newFdSet.File)
			symbolicFileDescriptors = append(symbolicFileDescriptors, symbolicProto)
		}
	}
	return symbolicFileDescriptors, nil
}

// Protoreflect needs all the dependencies that are used inside of the FileDescriptorProto (that gets rendered)
// to work properly. Those dependencies are google/protobuf/empty.proto, google/api/annotations.proto,
// and "google/protobuf/descriptor.proto". For all those dependencies the corresponding
// FileDescriptorProto has to be added to the FileDescriptorSet. Protoreflect won't work
// if a reference is missing.
func buildDependencies() (dependencies []*dpb.FileDescriptorProto) {
	// Dependency to google/api/annotations.proto for gRPC-HTTP transcoding. Here a couple of problems arise:
	// 1. Problem: 	We cannot call descriptor.ForMessage(&annotations.E_Http), which would be our
	//				required dependency. However, we can call descriptor.ForMessage(&http) and
	//				then construct the extension manually.
	// 2. Problem: 	The name is set wrong.
	// 3. Problem: 	google/api/annotations.proto has a dependency to google/protobuf/descriptor.proto.
	http := annotations.Http{}
	fd, _ := descriptor.MessageDescriptorProto(&http)

	extensionName := "http"
	n := "google/api/annotations.proto"
	l := dpb.FieldDescriptorProto_LABEL_OPTIONAL
	t := dpb.FieldDescriptorProto_TYPE_MESSAGE
	tName := "google.api.HttpRule"
	extendee := ".google.protobuf.MethodOptions"

	httpExtension := &dpb.FieldDescriptorProto{
		Name:     &extensionName,
		Number:   &annotations.E_Http.Field,
		Label:    &l,
		Type:     &t,
		TypeName: &tName,
		Extendee: &extendee,
	}

	fd.Extension = append(fd.Extension, httpExtension)                        // 1. Problem
	fd.Name = &n                                                              // 2. Problem
	fd.Dependency = append(fd.Dependency, "google/protobuf/descriptor.proto") //3.rd Problem

	// Build other required dependencies
	e := empty.Empty{}
	fdp := dpb.DescriptorProto{}
	fd2, _ := descriptor.MessageDescriptorProto(&e)
	fd3, _ := descriptor.MessageDescriptorProto(&fdp)
	dependencies = []*dpb.FileDescriptorProto{fd, fd2, fd3}
	return dependencies
}

// getNamesOfDependenciesThatWillBeImported adds the dependencies to the FileDescriptorProto we want to render (the last one). This essentially
// makes the 'import'  statements inside the .proto definition.
func getNamesOfDependenciesThatWillBeImported(dependencies []*dpb.FileDescriptorProto, methods []*surface_v1.Method) (names []string) {
	// At last, we need to add the dependencies to the FileDescriptorProto in order to get them rendered.
	for _, fd := range dependencies {
		if isEmptyDependency(*fd.Name) && shouldAddEmptyDependency(methods) {
			// Reference: https://github.com/google/gnostic-grpc/issues/8
			names = append(names, *fd.Name)
			continue
		}
		names = append(names, *fd.Name)
	}
	// Sort imports so they will be rendered in a consistent order.
	sort.Strings(names)
	return names
}

// isEmptyDependency returns true if the 'name' of the dependency is empty.proto
func isEmptyDependency(name string) bool {
	return name == "google/protobuf/empty.proto"
}

// shouldAddEmptyDependency returns true if at least one request parameter or response parameter is empty
func shouldAddEmptyDependency(methods []*surface_v1.Method) bool {
	for _, method := range methods {
		if method.ParametersTypeName == "" || method.ResponsesTypeName == "" {
			return true
		}
	}
	return false
}

// trimAndRemoveDuplicates returns a list of URLs that are not duplicates (considering only the part until the first '#')
func trimAndRemoveDuplicates(urls []string) []string {
	uniqueAndTrimmedUrls := make([]string, 0)
	for _, url := range urls {
		parts := strings.Split(url, "#")
		if !utils.Contains(uniqueAndTrimmedUrls, parts[0]) {
			uniqueAndTrimmedUrls = append(uniqueAndTrimmedUrls, parts[0])
		}
	}
	return uniqueAndTrimmedUrls
}

// getLast returns the last FileDescriptorProto of the array 'protos'.
func getLast(protos []*dpb.FileDescriptorProto) *dpb.FileDescriptorProto {
	return protos[len(protos)-1]
}

// CollectLocalFileRefs scans an OpenAPI v3 document for $ref values that point to local files
// (i.e., not HTTP(S) URLs and not internal #/ refs) and adds them to the surface model's
// SymbolicReferences so that buildSymbolicReferences will process them.
func CollectLocalFileRefs(doc *openapiv3.Document, model *surface_v1.Model) {
	refs := make(map[string]bool)
	collectRefsFromSchemaOrReferences(doc, refs)

	for ref := range refs {
		if !isHTTPRef(ref) && !isInternalRef(ref) {
			// Strip the fragment (e.g., #/components/schemas/Label) to get the file path
			filePath := ref
			if idx := strings.Index(ref, "#"); idx >= 0 {
				filePath = ref[:idx]
			}
			if filePath != "" && !utils.Contains(model.SymbolicReferences, filePath) {
				model.SymbolicReferences = append(model.SymbolicReferences, filePath)
			}
		}
	}
}

func isHTTPRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

func isInternalRef(ref string) bool {
	_, err := url.ParseRequestURI(ref)
	if err == nil {
		return false
	}
	return strings.HasPrefix(ref, "#")
}

// collectRefsFromSchemaOrReferences walks the OpenAPI document's components/schemas
// to find all $ref values.
func collectRefsFromSchemaOrReferences(doc *openapiv3.Document, refs map[string]bool) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return
	}
	for _, pair := range doc.Components.Schemas.AdditionalProperties {
		collectRefsFromSchemaOrRef(pair.Value, refs)
	}
	// Also check paths for inline $refs
	if doc.Paths != nil {
		for _, pathPair := range doc.Paths.Path {
			collectRefsFromPathItem(pathPair.Value, refs)
		}
	}
}

func collectRefsFromPathItem(pathItem *openapiv3.PathItem, refs map[string]bool) {
	if pathItem == nil {
		return
	}
	ops := []*openapiv3.Operation{
		pathItem.Get, pathItem.Put, pathItem.Post, pathItem.Delete,
		pathItem.Options, pathItem.Head, pathItem.Patch, pathItem.Trace,
	}
	for _, op := range ops {
		if op == nil {
			continue
		}
		for _, param := range op.Parameters {
			if ref := param.GetReference(); ref != nil {
				refs[ref.XRef] = true
			}
		}
		if op.RequestBody != nil {
			if ref := op.RequestBody.GetReference(); ref != nil {
				refs[ref.XRef] = true
			}
			if rb := op.RequestBody.GetRequestBody(); rb != nil {
				collectRefsFromMediaTypes(rb.Content, refs)
			}
		}
		if op.Responses != nil {
			for _, respPair := range op.Responses.ResponseOrReference {
				if ref := respPair.Value.GetReference(); ref != nil {
					refs[ref.XRef] = true
				}
				if resp := respPair.Value.GetResponse(); resp != nil {
					collectRefsFromMediaTypes(resp.Content, refs)
				}
			}
		}
	}
}

func collectRefsFromMediaTypes(mediaTypes *openapiv3.MediaTypes, refs map[string]bool) {
	if mediaTypes == nil {
		return
	}
	for _, mt := range mediaTypes.AdditionalProperties {
		if mt.Value != nil && mt.Value.Schema != nil {
			collectRefsFromSchemaOrRef(mt.Value.Schema, refs)
		}
	}
}

func collectRefsFromSchemaOrRef(sor *openapiv3.SchemaOrReference, refs map[string]bool) {
	if sor == nil {
		return
	}
	if ref := sor.GetReference(); ref != nil {
		refs[ref.XRef] = true
		return
	}
	schema := sor.GetSchema()
	if schema == nil {
		return
	}
	if schema.Properties != nil {
		for _, prop := range schema.Properties.AdditionalProperties {
			collectRefsFromSchemaOrRef(prop.Value, refs)
		}
	}
	if schema.Items != nil {
		for _, item := range schema.Items.SchemaOrReference {
			collectRefsFromSchemaOrRef(item, refs)
		}
	}
	if schema.AdditionalProperties != nil {
		collectRefsFromSchemaOrRef(schema.AdditionalProperties.GetSchemaOrReference(), refs)
	}
	for _, s := range schema.OneOf {
		collectRefsFromSchemaOrRef(s, refs)
	}
	for _, s := range schema.AnyOf {
		collectRefsFromSchemaOrRef(s, refs)
	}
	for _, s := range schema.AllOf {
		collectRefsFromSchemaOrRef(s, refs)
	}
}

func (renderer *Renderer) buildFileOptions() *dpb.FileOptions {
	goPackage := ".;" + renderer.Package
	fileOptions := &dpb.FileOptions{
		GoPackage: &goPackage,
	}
	return fileOptions
}
