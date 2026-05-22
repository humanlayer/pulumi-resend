import * as resend from "@pulumi/resend";

const webhook = new resend.Webhook("email-events", {
    endpoint: "https://example.com/webhooks/resend",
    events: [
        "email.sent",
        "email.delivered",
        "email.bounced",
        "email.complained",
    ],
});

export const webhookId = webhook.id;
export const webhookEndpoint = webhook.endpoint;
export const webhookEvents = webhook.events;
export const createdAt = webhook.createdAt;
