package api

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type openAPIServers struct {
	OpenAPI string `yaml:"openapi"`
	Info    struct {
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Servers []struct {
		URL         string `yaml:"url"`
		Description string `yaml:"description"`
	} `yaml:"servers"`
	Paths map[string]interface{} `yaml:"paths"`
}

func TestOpenAPISpec_ValidityAndServers(t *testing.T) {
	require.NotEmpty(t, OpenAPISpec, "OpenAPISpec must not be empty")

	var spec openAPIServers
	err := yaml.Unmarshal(OpenAPISpec, &spec)
	require.NoError(t, err, "OpenAPI specification must be valid YAML")

	// Validate OpenAPI Version and Title
	assert.True(t, strings.HasPrefix(spec.OpenAPI, "3.0"), "OpenAPI version must be 3.0.x")
	assert.NotEmpty(t, spec.Info.Title, "API title must not be empty")

	// Validate Servers list
	require.GreaterOrEqual(t, len(spec.Servers), 2, "Servers list must have at least 2 entries")

	// Default (first) server must be Render production
	assert.Equal(t, "https://quickcart-go.onrender.com", spec.Servers[0].URL)
	assert.Contains(t, spec.Servers[0].Description, "Production")

	// Secondary server must be local development
	assert.Equal(t, "http://localhost:8080", spec.Servers[1].URL)
	assert.Contains(t, spec.Servers[1].Description, "Local Development")

	// Validate critical paths exist
	assert.Contains(t, spec.Paths, "/", "Root path / must be documented")
	assert.Contains(t, spec.Paths, "/healthz", "Healthz path must be documented")
	assert.Contains(t, spec.Paths, "/readyz", "Readyz path must be documented")
	assert.Contains(t, spec.Paths, "/api/v1/auth/login", "Auth routes must be preserved")
	assert.Contains(t, spec.Paths, "/api/v1/checkout", "Checkout route must be preserved")
	assert.Contains(t, spec.Paths, "/api/v1/orders", "Order routes must be preserved")
}

func TestSwaggerUIHTML(t *testing.T) {
	assert.NotEmpty(t, SwaggerUIHTML)
	assert.Contains(t, SwaggerUIHTML, "SwaggerUIBundle")
	assert.Contains(t, SwaggerUIHTML, "/api/openapi.yaml")
}
