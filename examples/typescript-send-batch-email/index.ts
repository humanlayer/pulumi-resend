import * as resend from "@pulumi/resend";

const result = resend.sendBatchEmail({
    emails: [
        {
            from: "onboarding@resend.dev",
            to: ["kyle@humanlayer.dev"],
            subject: "Batch Email 1 from Pulumi",
            html: "<p>Hello from batch email 1!</p>",
        },
        {
            from: "onboarding@resend.dev",
            to: ["kyle@humanlayer.dev"],
            subject: "Batch Email 2 from Pulumi",
            html: "<p>Hello from batch email 2!</p>",
        },
    ],
});

export const emailIds = result.then(r => r.data.map(d => d.id));
