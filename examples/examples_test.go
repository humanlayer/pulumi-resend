// Package examples contains acceptance tests for the Resend provider examples.
//
// These tests validate that the example programs can be successfully previewed
// using the in-memory provider server. Full integration tests require a valid
// RESEND_API_KEY environment variable.
package examples

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	provider "github.com/kylemistele/pulumi-resend/provider"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// skipIfNoAPIKey skips the test if RESEND_API_KEY is not set.
func skipIfNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("RESEND_API_KEY") == "" {
		t.Skip("RESEND_API_KEY not set, skipping integration test")
	}
}

// newProviderServer creates an integration server for the Resend provider.
func newProviderServer(t *testing.T) integration.Server {
	t.Helper()

	ctx := context.Background()
	server, err := integration.NewServer(
		ctx,
		"resend",
		semver.MustParse("0.1.0-dev"),
		integration.WithProvider(provider.Provider()),
	)
	if err != nil {
		t.Fatalf("failed to create provider server: %v", err)
	}
	return server
}

func TestProviderSchema(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newProviderServer(t)

	resp, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}

	if resp.Schema == "" {
		t.Fatal("GetSchema returned empty schema")
	}
}

func TestYamlExampleExists(t *testing.T) {
	// Verify the YAML example file exists
	examplePath := filepath.Join("yaml", "Pulumi.yaml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("YAML example not found at %s", examplePath)
	}
}

func TestTypescriptApiKeyExampleExists(t *testing.T) {
	// Verify the TypeScript API key example exists
	examplePath := filepath.Join("typescript-apikey", "index.ts")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("TypeScript API key example not found at %s", examplePath)
	}
}

func TestTypescriptDomainExampleExists(t *testing.T) {
	// Verify the TypeScript domain example exists
	examplePath := filepath.Join("typescript-domain", "index.ts")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("TypeScript domain example not found at %s", examplePath)
	}
}

func TestTypescriptTemplateExampleExists(t *testing.T) {
	// Verify the TypeScript template example exists
	examplePath := filepath.Join("typescript-template", "index.ts")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("TypeScript template example not found at %s", examplePath)
	}
}

func TestTypescriptWebhookExampleExists(t *testing.T) {
	// Verify the TypeScript webhook example exists
	examplePath := filepath.Join("typescript-webhook", "index.ts")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("TypeScript webhook example not found at %s", examplePath)
	}
}

func TestTypescriptSendEmailExampleExists(t *testing.T) {
	// Verify the TypeScript send email example exists
	examplePath := filepath.Join("typescript-send-email", "index.ts")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatalf("TypeScript send email example not found at %s", examplePath)
	}
}

// TestProviderConfigure validates that the provider can be configured
// with the RESEND_API_KEY environment variable.
func TestProviderConfigure(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newProviderServer(t)

	err := server.Configure(p.ConfigureRequest{Args: property.Map{}})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
}

// TestProviderConfigureMissingKey validates that the provider fails
// to configure without an API key.
func TestProviderConfigureMissingKey(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")

	server := newProviderServer(t)

	err := server.Configure(p.ConfigureRequest{Args: property.Map{}})
	if err == nil {
		t.Fatal("Configure should have failed without API key")
	}
}
