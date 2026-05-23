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

### Topic

Manage subscription topics that control contact email preferences:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const newsletter = new resend.Topic("newsletter", {
    name: "Newsletter",
    defaultSubscription: "opt_in",
    description: "Weekly product updates",
    visibility: "public",
});

export const topicId = newsletter.id;
```

**Notes:**
- `defaultSubscription` is immutable after creation; changes require replacement
- `name`, `description`, and `visibility` can be updated in place

### Event

Define custom event types that trigger automations:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const userCreated = new resend.Event("userCreated", {
    name: "user.created",
    schema: {
        user_id: "string",
        plan: "string",
        trial_days: "number",
    },
});

export const eventId = userCreated.id;
```

**Notes:**
- Event `name` is immutable; changes require replacement
- `schema` can be updated in place
- Names must not start with `resend:` (reserved prefix)

### ContactProperty

Define custom fields on contacts:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const companyName = new resend.ContactProperty("companyName", {
    key: "company_name",
    type: "string",
    fallbackValue: "Unknown",
});

export const propertyId = companyName.id;
```

**Notes:**
- `key` and `type` are immutable; changes require replacement
- Only `fallbackValue` can be updated in place
- Key must be alphanumeric with underscores, max 50 characters

### Segment

Create contact segments for targeted broadcasts:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const activeUsers = new resend.Segment("activeUsers", {
    name: "Active Users",
});

export const segmentId = activeUsers.id;
```

**Notes:**
- Segments are immutable after creation (no update API)
- Any change to inputs requires delete and recreate

### Automation

Create multi-step email automation workflows:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const template = new resend.Template("welcome", {
    name: "Welcome Email",
    html: "<h1>Welcome!</h1><p>Thanks for signing up.</p>",
});

const automation = new resend.Automation("welcomeSequence", {
    name: "Welcome Sequence",
    status: "disabled",
    steps: [
        {
            key: "trigger",
            type: "trigger",
            config: { event_name: "user.created" },
        },
        {
            key: "send_welcome",
            type: "send_email",
            config: {
                template: { id: template.id },
                subject: "Welcome aboard!",
            },
        },
        {
            key: "wait_3_days",
            type: "delay",
            config: { duration: "3 days" },
        },
    ],
    connections: [
        { from: "trigger", to: "send_welcome" },
        { from: "send_welcome", to: "wait_3_days" },
    ],
});

export const automationId = automation.id;
```

**Step types:** `trigger`, `send_email`, `delay`, `wait_for_event`, `condition`, `contact_update`, `contact_delete`, `add_to_segment`

**Connection types:** `default`, `condition_met`, `condition_not_met`, `timeout`, `event_received`

### Send Batch Email (Function)

Send up to 100 emails in a single API call:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const result = await resend.sendBatchEmail({
    emails: [
        {
            from: "hello@mail.example.com",
            to: ["user1@example.com"],
            subject: "Hello User 1",
            html: "<p>Hello!</p>",
        },
        {
            from: "hello@mail.example.com",
            to: ["user2@example.com"],
            subject: "Hello User 2",
            html: "<p>Hello!</p>",
        },
    ],
});
```

### Send Event (Function)

Trigger custom events for automations:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const result = await resend.sendEvent({
    event: "user.created",
    email: "newuser@example.com",
    payload: {
        plan: "pro",
        trial_days: 14,
    },
});
```

**Notes:**
- Exactly one of `contactId` or `email` must be provided

### Send Broadcast (Function)

Create and send a one-time email campaign to a segment:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const customers = new resend.Segment("customers", {
    name: "All Customers",
});

const result = await resend.sendBroadcast({
    from: "updates@mail.example.com",
    subject: "Important Announcement",
    segmentId: customers.id,
    html: "<p>Hello!</p>",
    previewText: "We have exciting news...",
});
```

**Notes:**
- Creates and sends the broadcast atomically
- Supports scheduled sending via `scheduledAt` (ISO 8601 timestamp)

## Resources

| Resource | Description |
|----------|-------------|
| `Domain` | Email sending domain with DNS records |
| `DomainVerification` | Triggers DNS verification for a domain |
| `ApiKey` | API key for authentication |
| `Template` | Reusable email template |
| `Webhook` | Webhook endpoint for email events |
| `Topic` | Subscription topic for contact email preferences |
| `Event` | Custom event type for automation triggers |
| `ContactProperty` | Custom field definition on contacts |
| `Segment` | Contact segment for targeted broadcasts |
| `Automation` | Multi-step email automation workflow |

## Functions

| Function | Description |
|----------|-------------|
| `sendEmail` | Send a single email (stateless invoke) |
| `sendBatchEmail` | Send up to 100 emails in one call |
| `sendEvent` | Trigger a custom event for automations |
| `sendBroadcast` | Create and send a broadcast to a segment |

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
