# Pulumi Resend Provider

A native Pulumi provider for managing [Resend](https://resend.com) email infrastructure as code.

## Installation

### TypeScript/JavaScript

```bash
npm install @humanlayer/pulumi-resend
```

### Provider Binary

The provider binary is automatically downloaded from GitHub releases when you run `pulumi up`.

## Configuration

Set your Resend API key via environment variable or Pulumi config:

```bash
export RESEND_API_KEY=re_xxxxx
# or
pulumi config set resend:apiKey re_xxxxx --secret
```

## Usage

### Domain

Create and manage email sending domains:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const domain = new resend.Domain("my-domain", {
    name: "mail.example.com",
    region: "us-east-1",
});

export const domainId = domain.id;
export const dnsRecords = domain.records;
```

### Domain Verification

Trigger DNS verification for a domain:

```typescript
const verification = new resend.DomainVerification("verify", {
    domainId: domain.id,
});

export const verificationStatus = verification.status;
```

### API Key

Create API keys for sending emails:

```typescript
const apiKey = new resend.ApiKey("sending-key", {
    name: "production-sender",
    permission: "sending_access",
});

export const keyId = apiKey.id;
export const token = apiKey.token; // marked as secret
```

### Template

Create reusable email templates:

```typescript
const template = new resend.Template("welcome", {
    name: "welcome-email",
    subject: "Welcome to our service!",
    html: "<h1>Welcome {{name}}!</h1><p>Thanks for signing up.</p>",
});

export const templateId = template.id;
```

### Webhook

Set up webhooks for email events:

```typescript
const webhook = new resend.Webhook("events", {
    endpoint: "https://api.example.com/webhooks/resend",
    events: ["email.sent", "email.delivered", "email.bounced"],
});

export const webhookId = webhook.id;
```

### Send Email (Function)

Send emails directly via Pulumi:

```typescript
const result = resend.sendEmail({
    from: "hello@mail.example.com",
    to: ["user@example.com"],
    subject: "Hello from Pulumi!",
    html: "<p>This email was sent via infrastructure as code.</p>",
});

export const emailId = result.then(r => r.emailId);
```

> **Note**: `sendEmail` is a function (invoke), not a resource. It executes on every `pulumi up` and doesn't maintain state.

## Resources

| Resource | Description |
|----------|-------------|
| `Domain` | Email sending domain with DNS records |
| `DomainVerification` | Triggers DNS verification for a domain |
| `ApiKey` | API key for authentication |
| `Template` | Reusable email template |
| `Webhook` | Webhook endpoint for email events |

## Functions

| Function | Description |
|----------|-------------|
| `sendEmail` | Send an email (stateless invoke) |

## Development

```bash
# Build the provider
make provider

# Generate schema
make schema

# Generate TypeScript SDK
make codegen

# Run tests
go test ./...
```

## License

MIT
