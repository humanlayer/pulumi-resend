package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Valid webhook event types documented in the Resend API.
var validWebhookEvents = map[string]bool{
	"email.sent":             true,
	"email.delivered":        true,
	"email.bounced":          true,
	"email.complained":       true,
	"email.opened":           true,
	"email.clicked":          true,
	"email.failed":           true,
	"email.delivery_delayed": true,
	"email.scheduled":        true,
	"email.received":         true,
	"email.suppressed":       true,
	"domain.created":         true,
	"domain.updated":         true,
	"domain.deleted":         true,
	"contact.created":        true,
	"contact.updated":        true,
	"contact.deleted":        true,
}

type Webhook struct {
	getClient ClientGetter
}

type WebhookArgs struct {
	Endpoint string   `pulumi:"endpoint"`
	Events   []string `pulumi:"events"`
}

type WebhookState struct {
	WebhookArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
}

type webhookRequest struct {
	Endpoint string   `json:"endpoint"`
	Events   []string `json:"events"`
}

type webhookResponse struct {
	Id        string   `json:"id"`
	Endpoint  string   `json:"endpoint"`
	Events    []string `json:"events"`
	CreatedAt string   `json:"created_at"`
}

func NewWebhook(getClient ClientGetter) *Webhook {
	return &Webhook{getClient: getClient}
}

func (w *Webhook) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Webhook")
	annotator.Describe(w, "A Resend webhook endpoint that receives event notifications.")
}

func (args *WebhookArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Endpoint, "The HTTPS URL that Resend will send webhook events to.")
	annotator.Describe(&args.Events, "List of event types to subscribe to. Valid events include email.sent, email.delivered, email.bounced, email.complained, email.opened, email.clicked, email.failed, email.delivery_delayed, email.scheduled, email.received, email.suppressed, domain.created, domain.updated, domain.deleted, contact.created, contact.updated, and contact.deleted.")
}

func (state *WebhookState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the webhook was created.")
}

func (w *Webhook) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[WebhookArgs], error) {
	inputs, failures, err := infer.DefaultCheck[WebhookArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[WebhookArgs]{}, err
	}

	if inputs.Endpoint == "" {
		failures = append(failures, p.CheckFailure{
			Property: "endpoint",
			Reason:   "endpoint is required",
		})
	}

	if len(inputs.Events) == 0 {
		failures = append(failures, p.CheckFailure{
			Property: "events",
			Reason:   "at least one event type is required",
		})
	}

	for _, event := range inputs.Events {
		if !validWebhookEvents[event] {
			failures = append(failures, p.CheckFailure{
				Property: "events",
				Reason:   fmt.Sprintf("invalid event type: %q", event),
			})
		}
	}

	return infer.CheckResponse[WebhookArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (w *Webhook) Create(ctx context.Context, req infer.CreateRequest[WebhookArgs]) (infer.CreateResponse[WebhookState], error) {
	if req.DryRun {
		return infer.CreateResponse[WebhookState]{
			Output: WebhookState{WebhookArgs: req.Inputs},
		}, nil
	}

	resendClient, err := w.client(ctx)
	if err != nil {
		return infer.CreateResponse[WebhookState]{}, err
	}

	var created webhookResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/webhooks", webhookRequest{
		Endpoint: req.Inputs.Endpoint,
		Events:   req.Inputs.Events,
	}, &created); err != nil {
		return infer.CreateResponse[WebhookState]{}, fmt.Errorf("create webhook: %w", err)
	}
	if created.Id == "" {
		return infer.CreateResponse[WebhookState]{}, errors.New("create webhook: response did not include id")
	}

	state := webhookStateFromResponse(req.Inputs, created)
	if live, err := w.get(ctx, resendClient, created.Id); err == nil {
		state = webhookStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[WebhookState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (w *Webhook) Read(ctx context.Context, req infer.ReadRequest[WebhookArgs, WebhookState]) (infer.ReadResponse[WebhookArgs, WebhookState], error) {
	if req.ID == "" {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, nil
	}

	resendClient, err := w.client(ctx)
	if err != nil {
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, err
	}

	live, err := w.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[WebhookArgs, WebhookState]{}, nil
		}
		return infer.ReadResponse[WebhookArgs, WebhookState]{}, fmt.Errorf("read webhook %q: %w", req.ID, err)
	}

	inputs := normalizeWebhookInputs(req.Inputs, req.State, live)
	state := webhookStateFromResponse(inputs, live)
	return infer.ReadResponse[WebhookArgs, WebhookState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (w *Webhook) Update(ctx context.Context, req infer.UpdateRequest[WebhookArgs, WebhookState]) (infer.UpdateResponse[WebhookState], error) {
	if req.DryRun {
		return infer.UpdateResponse[WebhookState]{
			Output: previewWebhookUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := w.client(ctx)
	if err != nil {
		return infer.UpdateResponse[WebhookState]{}, err
	}

	path := "/webhooks/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, webhookRequest{
		Endpoint: req.Inputs.Endpoint,
		Events:   req.Inputs.Events,
	}, nil); err != nil {
		return infer.UpdateResponse[WebhookState]{}, fmt.Errorf("update webhook %q: %w", req.ID, err)
	}

	live, err := w.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[WebhookState]{}, fmt.Errorf("read updated webhook %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[WebhookState]{
		Output: webhookStateFromResponse(req.Inputs, live),
	}, nil
}

func (w *Webhook) Delete(ctx context.Context, req infer.DeleteRequest[WebhookState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := w.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/webhooks/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete webhook %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Webhook) Diff(ctx context.Context, req infer.DiffRequest[WebhookArgs, WebhookState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	addWebhookStringPropertyDiff(detailedDiff, "endpoint", req.State.Endpoint, req.Inputs.Endpoint)
	addWebhookEventsPropertyDiff(detailedDiff, "events", req.State.Events, req.Inputs.Events)

	return infer.DiffResponse{
		HasChanges:   len(detailedDiff) > 0,
		DetailedDiff: detailedDiff,
	}, nil
}

func (w *Webhook) client(ctx context.Context) (*client.Client, error) {
	if w == nil || w.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return w.getClient(ctx)
}

func (w *Webhook) get(ctx context.Context, resendClient *client.Client, id string) (webhookResponse, error) {
	var response webhookResponse
	path := "/webhooks/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return webhookResponse{}, err
	}
	return response, nil
}

func webhookStateFromResponse(inputs WebhookArgs, response webhookResponse) WebhookState {
	state := WebhookState{
		WebhookArgs: inputs,
		Id:          response.Id,
		CreatedAt:   response.CreatedAt,
	}
	if response.Endpoint != "" {
		state.Endpoint = response.Endpoint
	}
	if len(response.Events) > 0 {
		state.Events = response.Events
	}
	return state
}

func previewWebhookUpdate(id string, inputs WebhookArgs, oldState WebhookState) WebhookState {
	return WebhookState{
		WebhookArgs: inputs,
		Id:          id,
		CreatedAt:   oldState.CreatedAt,
	}
}

func normalizeWebhookInputs(inputs WebhookArgs, state WebhookState, response webhookResponse) WebhookArgs {
	if inputs.Endpoint == "" {
		inputs.Endpoint = state.Endpoint
	}
	if inputs.Endpoint == "" {
		inputs.Endpoint = response.Endpoint
	}
	if len(inputs.Events) == 0 {
		inputs.Events = state.Events
	}
	if len(inputs.Events) == 0 {
		inputs.Events = response.Events
	}
	return inputs
}

func addWebhookStringPropertyDiff(diff map[string]p.PropertyDiff, name, oldValue, newValue string) bool {
	if oldValue == newValue {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: webhookDiffKind(oldValue != "", newValue != ""), InputDiff: true}
	return true
}

func addWebhookEventsPropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue []string) bool {
	oldNormalized := normalizeWebhookEvents(oldValue)
	newNormalized := normalizeWebhookEvents(newValue)
	if slices.Equal(oldNormalized, newNormalized) {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: webhookDiffKind(len(oldNormalized) > 0, len(newNormalized) > 0), InputDiff: true}
	return true
}

func normalizeWebhookEvents(events []string) []string {
	normalized := make([]string, 0, len(events))
	for _, event := range events {
		if event != "" {
			normalized = append(normalized, event)
		}
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

func webhookDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
