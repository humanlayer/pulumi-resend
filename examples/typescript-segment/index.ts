import * as resend from "@pulumi/resend";

const activeUsers = new resend.Segment("activeUsers", {
    name: "Active Users",
    filter: {
        // Filter structure is opaque - varies by use case
    },
});

export const segmentId = activeUsers.id;
