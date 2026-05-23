import * as resend from "@pulumi/resend";

// First create a segment (this is a resource)
const customers = new resend.Segment("customers", {
    name: "Broadcast Test Segment",
});

export const segmentId = customers.id;

// Send a broadcast to that segment (this is a function invoke).
// NOTE: Broadcasts require a verified domain owned by your team.
// The built-in onboarding@resend.dev sender cannot be used for broadcasts.
// Uncomment below when you have a verified domain:
//
// const result = resend.sendBroadcastOutput({
//     from: "updates@your-verified-domain.com",
//     subject: "Important Announcement",
//     segmentId: customers.id,
//     html: "<p>Hello! This is a test broadcast.</p>",
//     previewText: "We have exciting news...",
// });
//
// export const broadcastId = result.broadcastId;
// export const broadcastStatus = result.status;
