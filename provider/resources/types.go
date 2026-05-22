package resources

import "github.com/pulumi/pulumi-go-provider/infer"

type DnsRecord struct {
	Record   string `pulumi:"record" json:"record"`
	Name     string `pulumi:"name" json:"name"`
	Type     string `pulumi:"type" json:"type"`
	Ttl      string `pulumi:"ttl" json:"ttl"`
	Status   string `pulumi:"status" json:"status"`
	Value    string `pulumi:"value" json:"value"`
	Priority *int   `pulumi:"priority,optional" json:"priority,omitempty"`
}

func (record *DnsRecord) Annotate(annotator infer.Annotator) {
	annotator.Describe(record, "A DNS record required to verify or operate a Resend domain.")
	annotator.Describe(&record.Record, "The Resend record purpose, such as SPF, DKIM, Receiving, Tracking, or TrackingCAA.")
	annotator.Describe(&record.Name, "The DNS record name.")
	annotator.Describe(&record.Type, "The DNS record type, such as TXT, MX, CNAME, or CAA.")
	annotator.Describe(&record.Ttl, "The DNS record time-to-live value.")
	annotator.Describe(&record.Status, "The verification status of the DNS record.")
	annotator.Describe(&record.Value, "The DNS record value.")
	annotator.Describe(&record.Priority, "The MX priority for records that require one.")
}
