PROTOC_GEN_GO_VERSION := v1.27.1
GOBIN := $(shell go env GOPATH)/bin
PROTOC_GEN_GO := $(GOBIN)/protoc-gen-go

.PHONY: all build test clean generate

all: generate build

# Install protoc-gen-go if not present or wrong version
$(PROTOC_GEN_GO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

# Generate Go code from proto files
incompatibility/incompatibility-report.pb.go: incompatibility/incompatibility-report.proto $(PROTOC_GEN_GO)
	protoc \
		--proto_path=incompatibility \
		--go_out=incompatibility \
		--go_opt=paths=source_relative \
		incompatibility-report.proto

generate: incompatibility/incompatibility-report.pb.go

build: generate
	go build ./...

test: generate
	go test ./...

clean:
	rm -f incompatibility/incompatibility-report.pb.go
