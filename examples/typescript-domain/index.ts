import * as pulumi from "@pulumi/pulumi";
import * as resend from "@pulumi/resend";

const config = new pulumi.Config();
const domainName = config.require("domainName");
const region = config.get("region") ?? "us-east-1";
const verify = config.getBoolean("verify") ?? false;

const domain = new resend.Domain("domain", {
    name: domainName,
    region,
});

const verification = verify
    ? new resend.DomainVerification("verification", {
        domainId: domain.id,
    })
    : undefined;

export const domainId = domain.id;
export const status = domain.status;
export const records = domain.records;
export const verificationStatus = verification?.status;
