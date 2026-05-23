package functions

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/kylemistele/pulumi-resend/provider/client"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	maxBatchSize          = 100
	maxRecipientsPerEmail = 50
)

// SendBatchEmail is a Pulumi function that sends up to 100 emails in a single API call via the Resend API.
type SendBatchEmail struct {
	getClient ClientGetter
}

// BatchEmailRequest represents a single email in a batch send request.
type BatchEmailRequest struct {
	From        string            `pulumi:"from"`
	To          []string          `pulumi:"to"`
	Subject     string            `pulumi:"subject"`
	Html        *string           `pulumi:"html,optional"`
	Text        *string           `pulumi:"text,optional"`
	Template    *string           `pulumi:"template,optional"`
	Cc          []string          `pulumi:"cc,optional"`
	Bcc         []string          `pulumi:"bcc,optional"`
	ReplyTo     []string          `pulumi:"replyTo,optional"`
	Headers     map[string]string `pulumi:"headers,optional"`
	Attachments []Attachment      `pulumi:"attachments,optional"`
	Tags        []Tag             `pulumi:"tags,optional"`
	ScheduledAt *string           `pulumi:"scheduledAt,optional"`
	TopicId     *string           `pulumi:"topicId,optional"`
}

// SendBatchEmailArgs contains the input parameters for sending a batch of emails.
type SendBatchEmailArgs struct {
	Emails []BatchEmailRequest `pulumi:"emails"`
}

// BatchEmailResult represents the result of a single email in a batch send.
type BatchEmailResult struct {
	Id string `pulumi:"id"`
}

// SendBatchEmailResult contains the output from sending a batch of emails.
type SendBatchEmailResult struct {
	Data []BatchEmailResult `pulumi:"data"`
}

// batchEmailAPIRequest is the JSON request body for a single email in POST /emails/batch.
type batchEmailAPIRequest struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Subject     string            `json:"subject"`
	Html        *string           `json:"html,omitempty"`
	Text        *string           `json:"text,omitempty"`
	Template    *string           `json:"template,omitempty"`
	Cc          []string          `json:"cc,omitempty"`
	Bcc         []string          `json:"bcc,omitempty"`
	ReplyTo     []string          `json:"reply_to,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	Tags        []Tag             `json:"tags,omitempty"`
	ScheduledAt *string           `json:"scheduled_at,omitempty"`
	TopicId     *string           `json:"topic_id,omitempty"`
}

// sendBatchEmailResponse is the JSON response body from POST /emails/batch.
type sendBatchEmailResponse struct {
	Data []struct {
		Id string `json:"id"`
	} `json:"data"`
}

// NewSendBatchEmail creates a new SendBatchEmail function with the given client getter.
func NewSendBatchEmail(getClient ClientGetter) *SendBatchEmail {
	return &SendBatchEmail{getClient: getClient}
}

func (f *SendBatchEmail) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "sendBatchEmail")
	annotator.Describe(f, "Send a batch of up to 100 emails in a single API call using the Resend API. Returns an array of email IDs.")
}

func (args *SendBatchEmailArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Emails, "List of emails to send (maximum 100).")
}

func (req *BatchEmailRequest) Annotate(annotator infer.Annotator) {
	annotator.Describe(&req.From, "The sender email address. Must be a verified domain.")
	annotator.Describe(&req.To, "List of recipient email addresses (maximum 50).")
	annotator.Describe(&req.Subject, "The email subject line.")
	annotator.Describe(&req.Html, "The HTML body of the email.")
	annotator.Describe(&req.Text, "The plain text body of the email.")
	annotator.Describe(&req.Template, "The template ID to use for the email body.")
	annotator.Describe(&req.Cc, "List of CC recipient email addresses.")
	annotator.Describe(&req.Bcc, "List of BCC recipient email addresses.")
	annotator.Describe(&req.ReplyTo, "List of reply-to email addresses.")
	annotator.Describe(&req.Headers, "Custom email headers as key-value pairs.")
	annotator.Describe(&req.Attachments, "List of file attachments.")
	annotator.Describe(&req.Tags, "List of tags for categorizing the email.")
	annotator.Describe(&req.ScheduledAt, "ISO 8601 timestamp to schedule the email for future delivery.")
	annotator.Describe(&req.TopicId, "Topic ID for managing unsubscribes.")
}

func (result *SendBatchEmailResult) Annotate(annotator infer.Annotator) {
	annotator.Describe(&result.Data, "Array of results, each containing the ID of a sent email.")
}

func (result *BatchEmailResult) Annotate(annotator infer.Annotator) {
	annotator.Describe(&result.Id, "The unique identifier for the sent email.")
}

// Invoke sends a batch of emails via the Resend API and returns the message IDs.
func (f *SendBatchEmail) Invoke(ctx context.Context, req infer.FunctionRequest[SendBatchEmailArgs]) (infer.FunctionResponse[SendBatchEmailResult], error) {
	resendClient, err := f.client(ctx)
	if err != nil {
		return infer.FunctionResponse[SendBatchEmailResult]{}, err
	}

	args := req.Input

	// Validate batch size
	if len(args.Emails) == 0 {
		return infer.FunctionResponse[SendBatchEmailResult]{}, errors.New("send batch email: emails list must not be empty")
	}
	if len(args.Emails) > maxBatchSize {
		return infer.FunctionResponse[SendBatchEmailResult]{}, fmt.Errorf("send batch email: maximum %d emails per batch, got %d", maxBatchSize, len(args.Emails))
	}

	// Validate per-email recipient count and build API request
	apiReqs := make([]batchEmailAPIRequest, len(args.Emails))
	for i, email := range args.Emails {
		totalRecipients := len(email.To) + len(email.Cc) + len(email.Bcc)
		if totalRecipients > maxRecipientsPerEmail {
			return infer.FunctionResponse[SendBatchEmailResult]{}, fmt.Errorf(
				"send batch email: email[%d] has %d total recipients (to + cc + bcc), maximum is %d",
				i, totalRecipients, maxRecipientsPerEmail,
			)
		}

		apiReqs[i] = batchEmailAPIRequest{
			From:        email.From,
			To:          email.To,
			Subject:     email.Subject,
			Html:        email.Html,
			Text:        email.Text,
			Template:    email.Template,
			Cc:          email.Cc,
			Bcc:        email.Bcc,
			ReplyTo:     email.ReplyTo,
			Headers:     email.Headers,
			Attachments: email.Attachments,
			Tags:        email.Tags,
			ScheduledAt: email.ScheduledAt,
			TopicId:     email.TopicId,
		}
	}

	var resp sendBatchEmailResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/emails/batch", apiReqs, &resp); err != nil {
		return infer.FunctionResponse[SendBatchEmailResult]{}, fmt.Errorf("send batch email: %w", err)
	}

	if len(resp.Data) == 0 {
		return infer.FunctionResponse[SendBatchEmailResult]{}, errors.New("send batch email: response did not include any email IDs")
	}

	results := make([]BatchEmailResult, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = BatchEmailResult{Id: d.Id}
	}

	return infer.FunctionResponse[SendBatchEmailResult]{
		Output: SendBatchEmailResult{Data: results},
	}, nil
}

func (f *SendBatchEmail) client(ctx context.Context) (*client.Client, error) {
	if f == nil || f.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return f.getClient(ctx)
}
