package provider

import (
	"github.com/kylemistele/pulumi-resend/provider/functions"
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
		WithResources(
			infer.Resource[*resources.ApiKey, resources.ApiKeyArgs, resources.ApiKeyState](
				resources.NewApiKey(ClientFromContext),
			),
			infer.Resource[*resources.Domain, resources.DomainArgs, resources.DomainState](
				resources.NewDomain(ClientFromContext),
			),
			infer.Resource[*resources.DomainVerification, resources.DomainVerificationArgs, resources.DomainVerificationState](
				resources.NewDomainVerification(ClientFromContext),
			),
			infer.Resource[*resources.Template, resources.TemplateArgs, resources.TemplateState](
				resources.NewTemplate(ClientFromContext),
			),
			infer.Resource[*resources.Topic, resources.TopicArgs, resources.TopicState](
				resources.NewTopic(ClientFromContext),
			),
			infer.Resource[*resources.Webhook, resources.WebhookArgs, resources.WebhookState](
				resources.NewWebhook(ClientFromContext),
			),
		).
		WithFunctions(
			infer.Function[*functions.SendEmail, functions.SendEmailArgs, functions.SendEmailResult](
				functions.NewSendEmail(ClientFromContext),
			),
		)

	return infer.Provider(builder.BuildOptions())
}
