package functions

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/kylemistele/pulumi-resend/provider/client"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SendEvent is a Pulumi function that triggers a custom event for a contact via the Resend API.
type SendEvent struct {
	getClient ClientGetter
}

// SendEventArgs contains the input parameters for sending an event.
type SendEventArgs struct {
	Event     string                 `pulumi:"event"`
	ContactId *string                `pulumi:"contactId,optional"`
	Email     *string                `pulumi:"email,optional"`
	Payload   map[string]interface{} `pulumi:"payload,optional"`
}

// SendEventResult contains the output from sending an event.
type SendEventResult struct {
	Event string `pulumi:"event"`
}

// sendEventRequest is the JSON request body for POST /events/send.
type sendEventRequest struct {
	Event     string                 `json:"event"`
	ContactId *string                `json:"contact_id,omitempty"`
	Email     *string                `json:"email,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// sendEventResponse is the JSON response body from POST /events/send.
type sendEventResponse struct {
	Object string `json:"object"`
	Event  string `json:"event"`
}

// NewSendEvent creates a new SendEvent function with the given client getter.
func NewSendEvent(getClient ClientGetter) *SendEvent {
	return &SendEvent{getClient: getClient}
}

func (f *SendEvent) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "sendEvent")
	annotator.Describe(f, "Trigger a custom event for a contact via the Resend API. Requires exactly one of contactId or email to identify the contact.")
}

func (args *SendEventArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Event, "The name of the event to trigger (e.g., 'user.created').")
	annotator.Describe(&args.ContactId, "The ID of the contact to trigger the event for. Exactly one of contactId or email must be provided.")
	annotator.Describe(&args.Email, "The email address of the contact to trigger the event for. Exactly one of contactId or email must be provided.")
	annotator.Describe(&args.Payload, "Optional key-value pairs of event data.")
}

func (result *SendEventResult) Annotate(annotator infer.Annotator) {
	annotator.Describe(&result.Event, "The name of the triggered event.")
}

// Invoke triggers a custom event for a contact via the Resend API.
func (f *SendEvent) Invoke(ctx context.Context, req infer.FunctionRequest[SendEventArgs]) (infer.FunctionResponse[SendEventResult], error) {
	resendClient, err := f.client(ctx)
	if err != nil {
		return infer.FunctionResponse[SendEventResult]{}, err
	}

	args := req.Input

	// Validate: exactly one of contactId or email must be provided
	hasContactId := args.ContactId != nil && *args.ContactId != ""
	hasEmail := args.Email != nil && *args.Email != ""
	if !hasContactId && !hasEmail {
		return infer.FunctionResponse[SendEventResult]{}, errors.New("send event: exactly one of contactId or email must be provided")
	}
	if hasContactId && hasEmail {
		return infer.FunctionResponse[SendEventResult]{}, errors.New("send event: only one of contactId or email may be provided, not both")
	}

	apiReq := sendEventRequest{
		Event:     args.Event,
		ContactId: args.ContactId,
		Email:     args.Email,
		Payload:   args.Payload,
	}

	var resp sendEventResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/events/send", apiReq, &resp); err != nil {
		return infer.FunctionResponse[SendEventResult]{}, fmt.Errorf("send event: %w", err)
	}

	eventName := resp.Event
	if eventName == "" {
		eventName = args.Event
	}

	return infer.FunctionResponse[SendEventResult]{
		Output: SendEventResult{Event: eventName},
	}, nil
}

func (f *SendEvent) client(ctx context.Context) (*client.Client, error) {
	if f == nil || f.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return f.getClient(ctx)
}
