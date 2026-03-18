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
	"errors"
	"go/format"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	openapiv3 "github.com/google/gnostic/openapiv3"
	plugins "github.com/google/gnostic/plugins"
	surface "github.com/google/gnostic/surface"
)

// RunProtoGenerator generates a FileDescriptorSet from a gnostic output file.
func RunProtoGenerator(env *plugins.Environment) {
	fileName := getFilenameWithoutFileExtension(env)
	packageName, err := resolvePackageName(fileName)
	env.RespondAndExitIfError(err)

	// First pass: extract the OpenAPI document and run the checker.
	var metadata *SchemaMetadata
	var openAPIdocument *openapiv3.Document
	inputDocumentType := env.Request.Models[0].TypeUrl
	for _, model := range env.Request.Models {
		if model.TypeUrl == "openapi.v3.Document" {
			openAPIdocument = &openapiv3.Document{}
			err := proto.Unmarshal(model.Value, openAPIdocument)
			if err == nil {
				featureChecker := NewGrpcChecker(openAPIdocument)
				env.Response.Messages = featureChecker.Run()
				metadata = NewSchemaMetadata(openAPIdocument)
			}
		}
	}

	// Build the surface model from the OpenAPI document ourselves rather than
	// using gnostic's pre-built surface model. Gnostic resolves local file $refs
	// inline without creating Type entries for externally-defined schemas, which
	// causes "unresolvable reference" errors.
	if openAPIdocument != nil {
		surfaceModel, err := surface.NewModelFromOpenAPI3(openAPIdocument, env.Request.SourceName)
		env.RespondAndExitIfError(err)

		CollectLocalFileRefs(openAPIdocument, surfaceModel)
		NewProtoLanguageModel().Prepare(surfaceModel, inputDocumentType)

		renderer := NewRenderer(surfaceModel, metadata)
		renderer.Package = packageName
		if abs, err := filepath.Abs(env.Request.SourceName); err == nil {
			renderer.SourceFile = abs
		}

		err = renderer.Render(env.Response, packageName+".proto")
		env.RespondAndExitIfError(err)
		env.RespondAndExit()
	}

	err = errors.New("No OpenAPI v3 document is available.")
	env.RespondAndExitIfError(err)
}

// resolvePackageName converts a path to a valid package name or
// error if path can't be resolved or resolves to an invalid package name.
func resolvePackageName(p string) (string, error) {
	p, err := filepath.Abs(p)
	p = strings.Replace(p, "-", "_", -1)
	if err == nil {
		p = filepath.Base(p)
		_, err = format.Source([]byte("package " + p))
	}
	if err != nil {
		return "", errors.New("invalid package name " + p)
	}
	return p, nil
}

func getFilenameWithoutFileExtension(env *plugins.Environment) string {
	fileName := env.Request.SourceName
	for {
		extension := filepath.Ext(fileName)
		if extension == "" {
			break
		}
		fileName = fileName[0 : len(fileName)-len(extension)]
	}
	return fileName
}
