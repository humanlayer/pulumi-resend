package functions

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/kylemistele/pulumi-resend/provider/client"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SendBroadcast is a Pulumi function that creates and sends a broadcast email
// to a segment via the Resend API in a single atomic operation.
type SendBroadcast struct {
	getClient ClientGetter
}

// SendBroadcastArgs contains the input parameters for sending a broadcast.
type SendBroadcastArgs struct {
	From        string   `pulumi:"from"`
	Subject     string   `pulumi:"subject"`
	SegmentId   string   `pulumi:"segmentId"`
	Name        *string  `pulumi:"name,optional"`
	ReplyTo     []string `pulumi:"replyTo,optional"`
	PreviewText *string  `pulumi:"previewText,optional"`
	Html        *string  `pulumi:"html,optional"`
	Text        *string  `pulumi:"text,optional"`
	TopicId     *string  `pulumi:"topicId,optional"`
	ScheduledAt *string  `pulumi:"scheduledAt,optional"`
}

// SendBroadcastResult contains the output from sending a broadcast.
type SendBroadcastResult struct {
	BroadcastId string `pulumi:"broadcastId"`
	Status      string `pulumi:"status"`
}

// sendBroadcastRequest is the JSON request body for POST /broadcasts.
type sendBroadcastRequest struct {
	From        string   `json:"from"`
	Subject     string   `json:"subject"`
	SegmentId   string   `json:"segment_id"`
	Send        bool     `json:"send"`
	Name        *string  `json:"name,omitempty"`
	ReplyTo     []string `json:"reply_to,omitempty"`
	PreviewText *string  `json:"preview_text,omitempty"`
	Html        *string  `json:"html,omitempty"`
	Text        *string  `json:"text,omitempty"`
	TopicId     *string  `json:"topic_id,omitempty"`
	ScheduledAt *string  `json:"scheduled_at,omitempty"`
}

// sendBroadcastResponse is the JSON response body from POST /broadcasts.
type sendBroadcastResponse struct {
	Id string `json:"id"`
}

// NewSendBroadcast creates a new SendBroadcast function with the given client getter.
func NewSendBroadcast(getClient ClientGetter) *SendBroadcast {
	return &SendBroadcast{getClient: getClient}
}

func (f *SendBroadcast) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "sendBroadcast")
	annotator.Describe(f, "Create and send a broadcast email to a segment via the Resend API. This combines broadcast creation and sending into a single atomic operation using the send: true flag.")
}

func (args *SendBroadcastArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.From, "The sender email address. Must be a verified domain.")
	annotator.Describe(&args.Subject, "The email subject line for the broadcast.")
	annotator.Describe(&args.SegmentId, "The ID of the segment to send the broadcast to.")
	annotator.Describe(&args.Name, "An optional name for the broadcast (for internal reference).")
	annotator.Describe(&args.ReplyTo, "List of reply-to email addresses.")
	annotator.Describe(&args.PreviewText, "Preview text shown in email clients before opening the email.")
	annotator.Describe(&args.Html, "The HTML body of the broadcast email.")
	annotator.Describe(&args.Text, "The plain text body of the broadcast email.")
	annotator.Describe(&args.TopicId, "Topic ID for managing unsubscribes.")
	annotator.Describe(&args.ScheduledAt, "ISO 8601 timestamp to schedule the broadcast for future delivery. Omit for immediate send.")
}

func (result *SendBroadcastResult) Annotate(annotator infer.Annotator) {
	annotator.Describe(&result.BroadcastId, "The unique identifier for the broadcast.")
	annotator.Describe(&result.Status, "The status of the broadcast (e.g., 'queued' or 'sent').")
}

// Invoke creates and sends a broadcast via the Resend API in a single atomic operation.
func (f *SendBroadcast) Invoke(ctx context.Context, req infer.FunctionRequest[SendBroadcastArgs]) (infer.FunctionResponse[SendBroadcastResult], error) {
	resendClient, err := f.client(ctx)
	if err != nil {
		return infer.FunctionResponse[SendBroadcastResult]{}, err
	}

	args := req.Input

	apiReq := sendBroadcastRequest{
		From:        args.From,
		Subject:     args.Subject,
		SegmentId:   args.SegmentId,
		Send:        true,
		Name:        args.Name,
		ReplyTo:     args.ReplyTo,
		PreviewText: args.PreviewText,
		Html:        args.Html,
		Text:        args.Text,
		TopicId:     args.TopicId,
		ScheduledAt: args.ScheduledAt,
	}

	var resp sendBroadcastResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/broadcasts", apiReq, &resp); err != nil {
		return infer.FunctionResponse[SendBroadcastResult]{}, fmt.Errorf("send broadcast: %w", err)
	}

	if resp.Id == "" {
		return infer.FunctionResponse[SendBroadcastResult]{}, errors.New("send broadcast: response did not include id")
	}

	// Determine status based on whether a scheduled time was provided
	status := "queued"
	if args.ScheduledAt != nil && *args.ScheduledAt != "" {
		status = "scheduled"
	}

	return infer.FunctionResponse[SendBroadcastResult]{
		Output: SendBroadcastResult{
			BroadcastId: resp.Id,
			Status:      status,
		},
	}, nil
}

func (f *SendBroadcast) client(ctx context.Context) (*client.Client, error) {
	if f == nil || f.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return f.getClient(ctx)
}
