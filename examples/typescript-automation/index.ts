import * as resend from "@pulumi/resend";

// Create a simple automation workflow
const automation = new resend.Automation("welcomeSequence", {
    name: "Welcome Sequence",
    status: "enabled", // Now enabled
    steps: [
        {
            key: "trigger",
            type: "trigger",
            config: { event_name: "user.created" },
        },
        {
            key: "wait_1_day",
            type: "delay",
            config: { duration: "1 day" },
        },
        {
            key: "wait_2_days",
            type: "delay",
            config: { duration: "2 days" },
        },
    ],
    connections: [
        { from: "trigger", to: "wait_1_day" },
        { from: "wait_1_day", to: "wait_2_days" },
    ],
});

export const automationId = automation.id;
export const automationStatus = automation.status;
