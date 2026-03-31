package main

import "embed"

//go:embed openapi
var openapiDir embed.FS

var (
	openAPISpec []byte
	docsHTML    []byte
)

func init() {
	var err error
	openAPISpec, err = openapiDir.ReadFile("openapi/openapi.yaml")
	if err != nil {
		panic(err)
	}
	docsHTML, err = openapiDir.ReadFile("openapi/docs.html")
	if err != nil {
		panic(err)
	}
}
