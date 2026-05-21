import * as resend from "@pulumi/resend";

const key = new resend.ApiKey("sendingKey", {
    name: "kyle pulumi testing key",
    permission: "sending_access",
});

export const keyId = key.id;
export const token = key.token;
