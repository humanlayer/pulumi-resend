import * as resend from "@pulumi/resend";

const result = resend.sendEvent({
    event: "test.event",
    email: "kyle@humanlayer.dev",
    payload: {
        plan: "pro",
        trial_days: 14,
    },
});

export const triggeredEvent = result.then(r => r.event);
