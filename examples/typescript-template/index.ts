import * as resend from "@pulumi/resend";

const template = new resend.Template("welcome", {
    name: "pulumi-welcome-template",
    html: "<h1>Welcome to {{{PRODUCT_NAME}}}</h1><p>Your template is managed by Pulumi.</p>",
    subject: "Welcome to Resend",
    text: "Welcome to {{{PRODUCT_NAME}}}. Thanks for trying Pulumi and Resend.",
    variables: [
        {
            key: "PRODUCT_NAME",
            type: "string",
            fallbackValue: "Resend",
        },
    ],
});

export const templateId = template.id;
export const createdAt = template.createdAt;
export const updatedAt = template.updatedAt;
