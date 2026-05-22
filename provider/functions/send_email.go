package functions

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/kylemistele/pulumi-resend/provider/client"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ClientGetter retrieves a configured Resend API client from context.
type ClientGetter func(context.Context) (*client.Client, error)

// SendEmail is a Pulumi function that sends an email via the Resend API.
type SendEmail struct {
	getClient ClientGetter
}

// SendEmailArgs contains the input parameters for sending an email.
type SendEmailArgs struct {
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

// SendEmailResult contains the output from sending an email.
type SendEmailResult struct {
	EmailId string `pulumi:"emailId"`
}

// sendEmailRequest is the JSON request body for POST /emails.
type sendEmailRequest struct {
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

// sendEmailResponse is the JSON response body from POST /emails.
type sendEmailResponse struct {
	Id string `json:"id"`
}

// NewSendEmail creates a new SendEmail function with the given client getter.
func NewSendEmail(getClient ClientGetter) *SendEmail {
	return &SendEmail{getClient: getClient}
}

func (f *SendEmail) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "sendEmail")
	annotator.Describe(f, "Send an email using the Resend API. This is a stateless function that sends an email and returns the message ID.")
}

func (args *SendEmailArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.From, "The sender email address. Must be a verified domain.")
	annotator.Describe(&args.To, "List of recipient email addresses (maximum 50).")
	annotator.Describe(&args.Subject, "The email subject line.")
	annotator.Describe(&args.Html, "The HTML body of the email.")
	annotator.Describe(&args.Text, "The plain text body of the email.")
	annotator.Describe(&args.Template, "The template ID to use for the email body.")
	annotator.Describe(&args.Cc, "List of CC recipient email addresses.")
	annotator.Describe(&args.Bcc, "List of BCC recipient email addresses.")
	annotator.Describe(&args.ReplyTo, "List of reply-to email addresses.")
	annotator.Describe(&args.Headers, "Custom email headers as key-value pairs.")
	annotator.Describe(&args.Attachments, "List of file attachments.")
	annotator.Describe(&args.Tags, "List of tags for categorizing the email.")
	annotator.Describe(&args.ScheduledAt, "ISO 8601 timestamp to schedule the email for future delivery.")
	annotator.Describe(&args.TopicId, "Topic ID for managing unsubscribes.")
}

func (result *SendEmailResult) Annotate(annotator infer.Annotator) {
	annotator.Describe(&result.EmailId, "The unique identifier for the sent email.")
}

// Invoke sends an email via the Resend API and returns the message ID.
func (f *SendEmail) Invoke(ctx context.Context, req infer.FunctionRequest[SendEmailArgs]) (infer.FunctionResponse[SendEmailResult], error) {
	resendClient, err := f.client(ctx)
	if err != nil {
		return infer.FunctionResponse[SendEmailResult]{}, err
	}

	args := req.Input
	apiReq := sendEmailRequest{
		From:        args.From,
		To:          args.To,
		Subject:     args.Subject,
		Html:        args.Html,
		Text:        args.Text,
		Template:    args.Template,
		Cc:          args.Cc,
		Bcc:         args.Bcc,
		ReplyTo:     args.ReplyTo,
		Headers:     args.Headers,
		Attachments: args.Attachments,
		Tags:        args.Tags,
		ScheduledAt: args.ScheduledAt,
		TopicId:     args.TopicId,
	}

	var resp sendEmailResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/emails", apiReq, &resp); err != nil {
		return infer.FunctionResponse[SendEmailResult]{}, fmt.Errorf("send email: %w", err)
	}

	if resp.Id == "" {
		return infer.FunctionResponse[SendEmailResult]{}, errors.New("send email: response did not include id")
	}

	return infer.FunctionResponse[SendEmailResult]{Output: SendEmailResult{EmailId: resp.Id}}, nil
}

func (f *SendEmail) client(ctx context.Context) (*client.Client, error) {
	if f == nil || f.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return f.getClient(ctx)
}
