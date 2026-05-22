import * as resend from "@pulumi/resend";

const newsletter = new resend.Topic("newsletter", {
    name: "Newsletter",
    defaultSubscription: "opt_in",
    description: "Weekly product updates",
    visibility: "public",
});

export const topicId = newsletter.id;
