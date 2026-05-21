package provider

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/kylemistele/pulumi-resend/provider/client"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type Config struct {
	ApiKey string `pulumi:"apiKey,optional" provider:"secret"`
	client *client.Client
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c, "The Resend provider configuration.")
	a.Describe(&c.ApiKey, "Resend API key. Falls back to RESEND_API_KEY when unset.")
}

func (c *Config) Configure(ctx context.Context) error {
	c.ApiKey = strings.TrimSpace(c.ApiKey)
	if c.ApiKey == "" {
		c.ApiKey = strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	}
	if c.ApiKey == "" {
		return errors.New("resend apiKey is required; set resend:apiKey or RESEND_API_KEY")
	}

	c.client = client.NewClient(c.ApiKey)
	return nil
}

func (c *Config) Client() *client.Client {
	if c.client == nil && c.ApiKey != "" {
		c.client = client.NewClient(c.ApiKey)
	}
	return c.client
}

func ClientFromContext(ctx context.Context) (*client.Client, error) {
	cfg := infer.GetConfig[Config](ctx)
	client := cfg.Client()
	if client == nil {
		return nil, errors.New("resend client is not configured")
	}
	return client, nil
}
