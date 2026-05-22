package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	provider "github.com/kylemistele/pulumi-resend/provider"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// newTestServer creates an integration server for the Resend provider.
func newTestServer(t *testing.T) integration.Server {
	t.Helper()

	ctx := context.Background()
	server, err := integration.NewServer(
		ctx,
		"resend",
		semver.MustParse("0.1.0-dev"),
		integration.WithProvider(provider.Provider()),
	)
	if err != nil {
		t.Fatalf("failed to create integration server: %v", err)
	}
	return server
}

func TestGetSchema(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	resp, err := server.GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}

	if resp.Schema == "" {
		t.Fatal("GetSchema returned empty schema")
	}

	// Verify schema is valid JSON
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Schema), &schema); err != nil {
		t.Fatalf("GetSchema returned invalid JSON: %v", err)
	}

	// Verify provider metadata
	if name, ok := schema["name"].(string); !ok || name != "resend" {
		t.Errorf("expected schema name 'resend', got %v", schema["name"])
	}

	// Verify resources exist
	resources, ok := schema["resources"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing resources")
	}

	expectedResources := []string{
		"resend:index:ApiKey",
		"resend:index:Domain",
		"resend:index:DomainVerification",
		"resend:index:Template",
		"resend:index:Webhook",
	}
	for _, res := range expectedResources {
		if _, ok := resources[res]; !ok {
			t.Errorf("schema missing resource: %s", res)
		}
	}

	// Verify function exists
	functions, ok := schema["functions"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing functions")
	}
	if _, ok := functions["resend:index:sendEmail"]; !ok {
		t.Error("schema missing function: resend:index:sendEmail")
	}
}

func TestConfigure(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure with empty inputs (should use env var)
	err := server.Configure(p.ConfigureRequest{
		Args: property.Map{},
	})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
}

func TestConfigureWithApiKey(t *testing.T) {
	// Clear any existing env var
	t.Setenv("RESEND_API_KEY", "")

	server := newTestServer(t)

	// Configure with explicit API key
	err := server.Configure(p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{
			"apiKey": property.New("re_explicit_key").WithSecret(true),
		}),
	})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
}

func TestConfigureMissingApiKey(t *testing.T) {
	// Clear any existing env var
	t.Setenv("RESEND_API_KEY", "")

	server := newTestServer(t)

	// Configure without API key should fail
	err := server.Configure(p.ConfigureRequest{
		Args: property.Map{},
	})
	if err == nil {
		t.Fatal("Configure should have failed without API key")
	}
}

func TestApiKeyCheckInputs(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Check valid ApiKey inputs
	resp, err := server.Check(p.CheckRequest{
		Urn: presource.URN("urn:pulumi:test::test::resend:index:ApiKey::test-key"),
		Inputs: property.NewMap(map[string]property.Value{
			"name": property.New("test-api-key"),
		}),
	})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(resp.Failures) > 0 {
		t.Errorf("Check returned unexpected failures: %v", resp.Failures)
	}
}

func TestDomainCheckInputs(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Check valid Domain inputs
	resp, err := server.Check(p.CheckRequest{
		Urn: presource.URN("urn:pulumi:test::test::resend:index:Domain::test-domain"),
		Inputs: property.NewMap(map[string]property.Value{
			"name": property.New("test.example.com"),
		}),
	})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(resp.Failures) > 0 {
		t.Errorf("Check returned unexpected failures: %v", resp.Failures)
	}
}

func TestTemplateCheckInputs(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Check valid Template inputs
	resp, err := server.Check(p.CheckRequest{
		Urn: presource.URN("urn:pulumi:test::test::resend:index:Template::test-template"),
		Inputs: property.NewMap(map[string]property.Value{
			"name": property.New("welcome-email"),
			"html": property.New("<h1>Welcome</h1>"),
		}),
	})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(resp.Failures) > 0 {
		t.Errorf("Check returned unexpected failures: %v", resp.Failures)
	}
}

func TestWebhookCheckInputs(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Check valid Webhook inputs
	resp, err := server.Check(p.CheckRequest{
		Urn: presource.URN("urn:pulumi:test::test::resend:index:Webhook::test-webhook"),
		Inputs: property.NewMap(map[string]property.Value{
			"endpoint": property.New("https://example.com/webhook"),
			"events": property.New([]property.Value{
				property.New("email.sent"),
				property.New("email.delivered"),
			}),
		}),
	})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(resp.Failures) > 0 {
		t.Errorf("Check returned unexpected failures: %v", resp.Failures)
	}
}

func TestApiKeyLifecycleTest(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Test the lifecycle check and diff operations
	lifecycle := integration.LifeCycleTest{
		Resource: tokens.Type("resend:index:ApiKey"),
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":       property.New("test-key"),
				"permission": property.New("sending_access"),
			}),
			// We expect failure since we're not actually calling the API
			ExpectFailure: true,
		},
	}

	// The lifecycle test will fail on Create since we don't have a real API,
	// but it validates the provider is wired up correctly
	lifecycle.Run(t, server)
}

func TestDomainLifecycleTest(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_key")

	server := newTestServer(t)

	// Configure the provider first
	if err := server.Configure(p.ConfigureRequest{Args: property.Map{}}); err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Test the lifecycle check and diff operations
	lifecycle := integration.LifeCycleTest{
		Resource: tokens.Type("resend:index:Domain"),
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":   property.New("test.example.com"),
				"region": property.New("us-east-1"),
			}),
			// We expect failure since we're not actually calling the API
			ExpectFailure: true,
		},
	}

	lifecycle.Run(t, server)
}
