package provider

import (
	"github.com/kylemistele/pulumi-resend/provider/resources"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func Provider() p.Provider {
	builder := infer.NewProviderBuilder().
		WithDisplayName("Resend").
		WithNamespace("resend").
		WithLanguageMap(map[string]any{
			"nodejs": map[string]any{
				"packageName":          "@pulumi/resend",
				"respectSchemaVersion": true,
			},
			"go": map[string]any{
				"generateResourceContainerTypes": true,
				"importBasePath":                 "github.com/kylemistele/pulumi-resend/sdk/go/resend",
				"respectSchemaVersion":           true,
			},
			"python": map[string]any{
				"pyproject": map[string]any{
					"enabled": true,
				},
				"respectSchemaVersion": true,
			},
			"csharp": map[string]any{
				"respectSchemaVersion": true,
			},
		}).
		WithConfig(infer.Config(&Config{})).
		WithResources(infer.Resource[*resources.ApiKey, resources.ApiKeyArgs, resources.ApiKeyState](
			resources.NewApiKey(ClientFromContext),
		))

	return infer.Provider(builder.BuildOptions())
}
