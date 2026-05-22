import * as resend from "@pulumi/resend";

const userCreated = new resend.Event("userCreated", {
    name: "user.created",
    schema: {
        user_id: "string",
        plan: "string",
        trial_days: "number",
    },
});

export const eventId = userCreated.id;
