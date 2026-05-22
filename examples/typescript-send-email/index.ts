import * as resend from "@pulumi/resend";

// Send an email using the Resend API.
// Note: Using Resend's test addresses that work without a verified domain.
const result = resend.sendEmail({
    from: "onboarding@resend.dev",
    to: ["delivered@resend.dev"],
    subject: "Hello from Pulumi",
    html: "<p>Hello World</p>",
});

export const emailId = result.then(r => r.emailId);
