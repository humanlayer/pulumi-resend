import * as resend from "@pulumi/resend";

const companyName = new resend.ContactProperty("companyName", {
    key: "company_name",
    type: "string",
    fallbackValue: "Unknown",
});

const score = new resend.ContactProperty("score", {
    key: "score",
    type: "number",
});

export const propertyId = companyName.id;
