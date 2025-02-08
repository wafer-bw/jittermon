//go:build tools
// +build tools

package tools

// Tools not imported in the module but still required should be
// added here so that they are still tracked inside go.mod

import (
	_ "github.com/daixiang0/gci"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/joho/godotenv/cmd/godotenv"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
