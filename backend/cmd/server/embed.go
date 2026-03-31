package main

import "embed"

//go:embed openapi/openapi.yaml
var openAPISpec []byte

//go:embed openapi/docs.html
var docsHTML []byte
