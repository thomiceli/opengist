package api

import (
	_ "embed"

	"github.com/thomiceli/opengist/internal/web/context"
)

//go:embed openapi.yaml
var openapiYAML []byte

// OpenAPISpec serves the embedded OpenAPI YAML spec.
// Import this URL into Postman / Insomnia / Bruno / openapi-generator etc.
func OpenAPISpec(ctx *context.Context) error {
	ctx.Response().Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, err := ctx.Response().Write(openapiYAML)
	return err
}
