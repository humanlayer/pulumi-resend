package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	eventSchemaTypeString  = "string"
	eventSchemaTypeNumber  = "number"
	eventSchemaTypeBoolean = "boolean"
	eventSchemaTypeDate    = "date"
	eventReservedPrefix    = "resend:"
)

type Event struct {
	getClient ClientGetter
}

// EventSchema maps field names to types (string, number, boolean, date)
type EventSchema map[string]string

type EventArgs struct {
	Name   string       `pulumi:"name"`
	Schema *EventSchema `pulumi:"schema,optional"`
}

type EventState struct {
	EventArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

type eventCreateRequest struct {
	Name   string            `json:"name"`
	Schema map[string]string `json:"schema,omitempty"`
}

type eventUpdateRequest struct {
	Schema map[string]string `json:"schema,omitempty"`
}

type eventResponse struct {
	Id        string            `json:"id"`
	Name      string            `json:"name"`
	Schema    map[string]string `json:"schema"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

func NewEvent(getClient ClientGetter) *Event {
	return &Event{getClient: getClient}
}

func (e *Event) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Event")
	annotator.Describe(e, "A Resend event for defining custom event types that trigger automations. Events allow you to send behavioral data about contacts to trigger automated email sequences.")
}

func (args *EventArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The name of the event. Must not start with 'resend:' (reserved prefix). This field is immutable after creation.")
	annotator.Describe(&args.Schema, "Optional schema defining the event payload structure. Maps field names to types: string, number, boolean, or date.")
}

func (state *EventState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the event was created.")
	annotator.Describe(&state.UpdatedAt, "Timestamp when the event was last updated.")
}

func (e *Event) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[EventArgs], error) {
	inputs, failures, err := infer.DefaultCheck[EventArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[EventArgs]{}, err
	}

	// Validate required fields
	if inputs.Name == "" {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "name is required",
		})
	} else if strings.HasPrefix(strings.ToLower(inputs.Name), eventReservedPrefix) {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   fmt.Sprintf("name must not start with %q (reserved prefix)", eventReservedPrefix),
		})
	}

	// Validate schema field types if provided
	if inputs.Schema != nil {
		for fieldName, fieldType := range *inputs.Schema {
			if fieldType != eventSchemaTypeString &&
				fieldType != eventSchemaTypeNumber &&
				fieldType != eventSchemaTypeBoolean &&
				fieldType != eventSchemaTypeDate {
				failures = append(failures, p.CheckFailure{
					Property: fmt.Sprintf("schema.%s", fieldName),
					Reason:   fmt.Sprintf("schema field type must be one of: %s, %s, %s, %s", eventSchemaTypeString, eventSchemaTypeNumber, eventSchemaTypeBoolean, eventSchemaTypeDate),
				})
			}
		}
	}

	return infer.CheckResponse[EventArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (e *Event) Create(ctx context.Context, req infer.CreateRequest[EventArgs]) (infer.CreateResponse[EventState], error) {
	if req.DryRun {
		return infer.CreateResponse[EventState]{
			Output: EventState{EventArgs: req.Inputs},
		}, nil
	}

	resendClient, err := e.client(ctx)
	if err != nil {
		return infer.CreateResponse[EventState]{}, err
	}

	createReq := eventCreateRequest{
		Name: req.Inputs.Name,
	}
	if req.Inputs.Schema != nil {
		createReq.Schema = *req.Inputs.Schema
	}

	var created eventResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/events", createReq, &created); err != nil {
		return infer.CreateResponse[EventState]{}, fmt.Errorf("create event %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[EventState]{}, errors.New("create event: response did not include id")
	}

	state := eventStateFromResponse(req.Inputs, created)
	if live, err := e.get(ctx, resendClient, created.Id); err == nil {
		state = eventStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[EventState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (e *Event) Read(ctx context.Context, req infer.ReadRequest[EventArgs, EventState]) (infer.ReadResponse[EventArgs, EventState], error) {
	if req.ID == "" {
		return infer.ReadResponse[EventArgs, EventState]{}, nil
	}

	resendClient, err := e.client(ctx)
	if err != nil {
		return infer.ReadResponse[EventArgs, EventState]{}, err
	}

	live, err := e.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[EventArgs, EventState]{}, nil
		}
		return infer.ReadResponse[EventArgs, EventState]{}, fmt.Errorf("read event %q: %w", req.ID, err)
	}

	inputs := normalizeEventInputs(req.Inputs, req.State, live)
	state := eventStateFromResponse(inputs, live)
	return infer.ReadResponse[EventArgs, EventState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (e *Event) Update(ctx context.Context, req infer.UpdateRequest[EventArgs, EventState]) (infer.UpdateResponse[EventState], error) {
	if req.DryRun {
		return infer.UpdateResponse[EventState]{
			Output: previewEventUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := e.client(ctx)
	if err != nil {
		return infer.UpdateResponse[EventState]{}, err
	}

	// Only schema can be updated (name is immutable)
	updateReq := eventUpdateRequest{}
	if req.Inputs.Schema != nil {
		updateReq.Schema = *req.Inputs.Schema
	}

	path := "/events/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, updateReq, nil); err != nil {
		return infer.UpdateResponse[EventState]{}, fmt.Errorf("update event %q: %w", req.ID, err)
	}

	live, err := e.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[EventState]{}, fmt.Errorf("read updated event %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[EventState]{
		Output: eventStateFromResponse(req.Inputs, live),
	}, nil
}

func (e *Event) Delete(ctx context.Context, req infer.DeleteRequest[EventState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := e.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/events/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete event %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Event) Diff(ctx context.Context, req infer.DiffRequest[EventArgs, EventState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	requiresReplace := false

	// name is immutable after creation - any change requires replacement
	if req.Inputs.Name != req.State.Name {
		detailedDiff["name"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	// schema can be updated in place
	addEventSchemaDiff(detailedDiff, "schema", req.State.Schema, req.Inputs.Schema)

	return infer.DiffResponse{
		DeleteBeforeReplace: requiresReplace,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (e *Event) client(ctx context.Context) (*client.Client, error) {
	if e == nil || e.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return e.getClient(ctx)
}

func (e *Event) get(ctx context.Context, resendClient *client.Client, id string) (eventResponse, error) {
	var response eventResponse
	path := "/events/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return eventResponse{}, err
	}
	return response, nil
}

func eventStateFromResponse(inputs EventArgs, response eventResponse) EventState {
	state := EventState{
		EventArgs: inputs,
		Id:        response.Id,
		CreatedAt: response.CreatedAt,
		UpdatedAt: response.UpdatedAt,
	}
	if response.Name != "" {
		state.Name = response.Name
	}
	if len(response.Schema) > 0 {
		schema := EventSchema(response.Schema)
		state.Schema = &schema
	}
	return state
}

func previewEventUpdate(id string, inputs EventArgs, oldState EventState) EventState {
	return EventState{
		EventArgs: inputs,
		Id:        id,
		CreatedAt: oldState.CreatedAt,
		UpdatedAt: oldState.UpdatedAt,
	}
}

func normalizeEventInputs(inputs EventArgs, state EventState, response eventResponse) EventArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}
	if inputs.Schema == nil {
		inputs.Schema = state.Schema
	}
	return inputs
}

func addEventSchemaDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *EventSchema) bool {
	oldSet := oldValue != nil && len(*oldValue) > 0
	newSet := newValue != nil && len(*newValue) > 0

	if !oldSet && !newSet {
		return false
	}

	if oldSet && newSet && reflect.DeepEqual(*oldValue, *newValue) {
		return false
	}

	diff[name] = p.PropertyDiff{Kind: eventSchemaDiffKind(oldSet, newSet), InputDiff: true}
	return true
}

func eventSchemaDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
