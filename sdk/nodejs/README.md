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

## Examples

### Complete Email Platform Setup

Set up a full email platform with domain verification, templates, topics, and automation:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

// 1. Domain with tracking enabled
const domain = new resend.Domain("mail", {
    name: "mail.example.com",
    region: "us-east-1",
    openTracking: true,
    clickTracking: true,
});

// 2. Verify the domain (polls until DNS records are confirmed)
const verification = new resend.DomainVerification("verify", {
    domainId: domain.id,
});

// 3. Scoped API key for sending only
const sendingKey = new resend.ApiKey("sender", {
    name: "production-sender",
    permission: "sending_access",
    domainId: domain.id,
});

// 4. Subscription topics
const newsletter = new resend.Topic("newsletter", {
    name: "Newsletter",
    defaultSubscription: "opt_in",
    description: "Weekly product updates and tips",
    visibility: "public",
});

const marketing = new resend.Topic("marketing", {
    name: "Marketing",
    defaultSubscription: "opt_out",
    description: "Promotional offers and announcements",
    visibility: "public",
});

// 5. Contact properties for personalization
const companyName = new resend.ContactProperty("company", {
    key: "company_name",
    type: "string",
    fallbackValue: "there",
});

const plan = new resend.ContactProperty("plan", {
    key: "plan",
    type: "string",
    fallbackValue: "free",
});

// 6. Webhook for delivery tracking
const webhook = new resend.Webhook("delivery-events", {
    endpoint: "https://api.example.com/webhooks/resend",
    events: [
        "email.sent",
        "email.delivered",
        "email.bounced",
        "email.complained",
        "email.opened",
        "email.clicked",
    ],
});

export const domainId = domain.id;
export const dnsRecords = domain.records;
export const apiKeyToken = sendingKey.token;
```

### Welcome Email Automation

Build a multi-step onboarding sequence triggered when users sign up:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

// Define the trigger event
const signupEvent = new resend.Event("signup", {
    name: "user.signup",
    schema: {
        first_name: "string",
        plan: "string",
        trial_days: "number",
    },
});

// Create email templates
const welcomeTemplate = new resend.Template("welcome", {
    name: "Welcome Email",
    subject: "Welcome to Example, {{first_name}}!",
    from: "hello@mail.example.com",
    html: `
        <h1>Welcome, {{first_name}}!</h1>
        <p>Thanks for joining us on the {{plan}} plan.</p>
        <p>You have {{trial_days}} days to explore everything.</p>
    `,
});

const tipsTemplate = new resend.Template("tips", {
    name: "Getting Started Tips",
    subject: "3 tips to get the most out of Example",
    from: "hello@mail.example.com",
    html: "<h1>Pro tips for you</h1><p>Here's how to get started...</p>",
});

const checkInTemplate = new resend.Template("checkin", {
    name: "Check-in Email",
    subject: "How's it going, {{first_name}}?",
    from: "hello@mail.example.com",
    html: "<h1>Hey {{first_name}}</h1><p>Just checking in...</p>",
});

// Build the automation workflow
const onboarding = new resend.Automation("onboarding", {
    name: "New User Onboarding",
    status: "enabled",
    steps: [
        {
            key: "trigger",
            type: "trigger",
            config: { event_name: "user.signup" },
        },
        {
            key: "welcome",
            type: "send_email",
            config: {
                template: { id: welcomeTemplate.id },
            },
        },
        {
            key: "wait_2_days",
            type: "delay",
            config: { duration: "2 days" },
        },
        {
            key: "tips",
            type: "send_email",
            config: {
                template: { id: tipsTemplate.id },
            },
        },
        {
            key: "wait_5_days",
            type: "delay",
            config: { duration: "5 days" },
        },
        {
            key: "checkin",
            type: "send_email",
            config: {
                template: { id: checkInTemplate.id },
            },
        },
    ],
    connections: [
        { from: "trigger", to: "welcome" },
        { from: "welcome", to: "wait_2_days" },
        { from: "wait_2_days", to: "tips" },
        { from: "tips", to: "wait_5_days" },
        { from: "wait_5_days", to: "checkin" },
    ],
});

// Trigger the automation for a new user (invoke)
const triggerSignup = resend.sendEvent({
    event: "user.signup",
    email: "newuser@example.com",
    payload: {
        first_name: "Alice",
        plan: "pro",
        trial_days: 14,
    },
});
```

### Domain Setup with AWS Route53

Use domain DNS records to configure Route53 automatically:

```typescript
import * as resend from "@humanlayer/pulumi-resend";
import * as aws from "@pulumi/aws";

const domain = new resend.Domain("mail", {
    name: "mail.example.com",
    region: "us-east-1",
});

const zone = aws.route53.getZone({ name: "example.com" });

// Create DNS records from the domain's output
domain.records.apply(records => {
    records.forEach((rec, i) => {
        new aws.route53.Record(`dns-${i}`, {
            zoneId: zone.then(z => z.zoneId),
            name: rec.name,
            type: rec.type,
            records: [rec.value],
            ttl: parseInt(rec.ttl) || 300,
        });
    });
});

// Verify after DNS records are created
const verification = new resend.DomainVerification("verify", {
    domainId: domain.id,
});
```

### Conditional Automation with Branching

Create an automation with conditional logic based on user properties:

```typescript
import * as resend from "@humanlayer/pulumi-resend";

const automation = new resend.Automation("trial-followup", {
    name: "Trial Expiry Follow-up",
    status: "enabled",
    steps: [
        {
            key: "trigger",
            type: "trigger",
            config: { event_name: "trial.expiring" },
        },
        {
            key: "check_plan",
            type: "condition",
            config: {
                type: "rule",
                field: "plan",
                operator: "equals",
                value: "enterprise",
            },
        },
        {
            key: "enterprise_email",
            type: "send_email",
            config: {
                template: { id: "tmpl_enterprise_upsell" },
                subject: "Your enterprise trial is ending soon",
            },
        },
        {
            key: "standard_email",
            type: "send_email",
            config: {
                template: { id: "tmpl_standard_upsell" },
                subject: "Upgrade before your trial ends",
            },
        },
    ],
    connections: [
        { from: "trigger", to: "check_plan" },
        { from: "check_plan", to: "enterprise_email", type: "condition_met" },
        { from: "check_plan", to: "standard_email", type: "condition_not_met" },
    ],
});
```

### Importing Existing Resources

Import existing Resend resources into Pulumi state:

```bash
# Import an existing domain
pulumi import resend:index:Domain myDomain d_abc123

# Import an existing API key
pulumi import resend:index:ApiKey myKey ak_xyz789

# Import an existing webhook
pulumi import resend:index:Webhook myWebhook wh_def456

# Import an existing template
pulumi import resend:index:Template myTemplate tmpl_ghi789
```

### YAML Example

```yaml
name: resend-email-infra
runtime: yaml

resources:
  domain:
    type: resend:index:Domain
    properties:
      name: mail.example.com
      region: us-east-1
      openTracking: true

  apiKey:
    type: resend:index:ApiKey
    properties:
      name: my-sending-key
      permission: sending_access

  newsletter:
    type: resend:index:Topic
    properties:
      name: Newsletter
      defaultSubscription: opt_in
      visibility: public

  welcomeTemplate:
    type: resend:index:Template
    properties:
      name: Welcome
      subject: Welcome!
      html: "<h1>Welcome!</h1>"

  webhook:
    type: resend:index:Webhook
    properties:
      endpoint: https://api.example.com/webhooks/resend
      events:
        - email.delivered
        - email.bounced

outputs:
  domainId: ${domain.id}
  dnsRecords: ${domain.records}
  apiKeyToken: ${apiKey.token}
```

## Resources

| Resource | Description | Update | Notes |
|----------|-------------|--------|-------|
| `Domain` | Email sending domain with DNS records | In-place | `name`/`region` immutable |
| `DomainVerification` | Triggers DNS verification for a domain | No-op | Idempotent, delete is no-op |
| `ApiKey` | API key for authentication | Replace | All fields immutable, token is secret |
| `Template` | Reusable email template | In-place | Full CRUD |
| `Webhook` | Webhook endpoint for email events | In-place | Full CRUD |
| `Topic` | Subscription topic for contact preferences | In-place | `defaultSubscription` immutable |
| `Event` | Custom event type for automation triggers | In-place | `name` immutable, `schema` updatable |
| `ContactProperty` | Custom field definition on contacts | In-place | `key`/`type` immutable |
| `Segment` | Contact segment for targeted broadcasts | Replace | No update API |
| `Automation` | Multi-step email automation workflow | In-place | Steps/connections replaced together |

## Functions

| Function | Description | Notes |
|----------|-------------|-------|
| `sendEmail` | Send a single email | Runs on every `pulumi up` |
| `sendBatchEmail` | Send up to 100 emails in one call | Max 50 recipients per email |
| `sendEvent` | Trigger a custom event for automations | Requires `contactId` or `email` |
| `sendBroadcast` | Create and send a broadcast to a segment | Atomic create+send |

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

# Build and test everything
make provider && make schema && make codegen && cd sdk/nodejs && npm install && npm run build
```

## License

MIT
