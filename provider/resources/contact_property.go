package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	contactPropertyTypeString = "string"
	contactPropertyTypeNumber = "number"
	contactPropertyKeyMaxLen  = 50
)

// contactPropertyKeyPattern matches alphanumeric characters and underscores only
var contactPropertyKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type ContactProperty struct {
	getClient ClientGetter
}

type ContactPropertyArgs struct {
	Key           string  `pulumi:"key"`
	Type          string  `pulumi:"type"`
	FallbackValue *string `pulumi:"fallbackValue,optional"`
}

type ContactPropertyState struct {
	ContactPropertyArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
}

type contactPropertyCreateRequest struct {
	Key           string  `json:"key"`
	Type          string  `json:"type"`
	FallbackValue *string `json:"fallback_value,omitempty"`
}

type contactPropertyUpdateRequest struct {
	FallbackValue *string `json:"fallback_value,omitempty"`
}

type contactPropertyResponse struct {
	Id            string  `json:"id"`
	Key           string  `json:"key"`
	Type          string  `json:"type"`
	FallbackValue *string `json:"fallback_value"`
	CreatedAt     string  `json:"created_at"`
}

func NewContactProperty(getClient ClientGetter) *ContactProperty {
	return &ContactProperty{getClient: getClient}
}

func (cp *ContactProperty) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "ContactProperty")
	annotator.Describe(cp, "A Resend contact property for defining custom fields on contacts. Contact properties allow you to store additional data about your contacts.")
}

func (args *ContactPropertyArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Key, "The key of the contact property. Must be alphanumeric with underscores only, maximum 50 characters. This field is immutable after creation.")
	annotator.Describe(&args.Type, "The type of the contact property: string or number. This field is immutable after creation.")
	annotator.Describe(&args.FallbackValue, "Optional fallback value used when the property is not set on a contact.")
}

func (state *ContactPropertyState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the contact property was created.")
}

func (cp *ContactProperty) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ContactPropertyArgs], error) {
	inputs, failures, err := infer.DefaultCheck[ContactPropertyArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ContactPropertyArgs]{}, err
	}

	// Validate key
	if inputs.Key == "" {
		failures = append(failures, p.CheckFailure{
			Property: "key",
			Reason:   "key is required",
		})
	} else {
		if len(inputs.Key) > contactPropertyKeyMaxLen {
			failures = append(failures, p.CheckFailure{
				Property: "key",
				Reason:   fmt.Sprintf("key must be %d characters or less", contactPropertyKeyMaxLen),
			})
		}
		if !contactPropertyKeyPattern.MatchString(inputs.Key) {
			failures = append(failures, p.CheckFailure{
				Property: "key",
				Reason:   "key must contain only alphanumeric characters and underscores",
			})
		}
	}

	// Validate type
	if inputs.Type == "" {
		failures = append(failures, p.CheckFailure{
			Property: "type",
			Reason:   "type is required",
		})
	} else if inputs.Type != contactPropertyTypeString && inputs.Type != contactPropertyTypeNumber {
		failures = append(failures, p.CheckFailure{
			Property: "type",
			Reason:   fmt.Sprintf("type must be %q or %q", contactPropertyTypeString, contactPropertyTypeNumber),
		})
	}

	return infer.CheckResponse[ContactPropertyArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (cp *ContactProperty) Create(ctx context.Context, req infer.CreateRequest[ContactPropertyArgs]) (infer.CreateResponse[ContactPropertyState], error) {
	if req.DryRun {
		return infer.CreateResponse[ContactPropertyState]{
			Output: ContactPropertyState{ContactPropertyArgs: req.Inputs},
		}, nil
	}

	resendClient, err := cp.client(ctx)
	if err != nil {
		return infer.CreateResponse[ContactPropertyState]{}, err
	}

	createReq := contactPropertyCreateRequest{
		Key:           req.Inputs.Key,
		Type:          req.Inputs.Type,
		FallbackValue: req.Inputs.FallbackValue,
	}

	var created contactPropertyResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/contact-properties", createReq, &created); err != nil {
		return infer.CreateResponse[ContactPropertyState]{}, fmt.Errorf("create contact property %q: %w", req.Inputs.Key, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[ContactPropertyState]{}, errors.New("create contact property: response did not include id")
	}

	state := contactPropertyStateFromResponse(req.Inputs, created)
	if live, err := cp.get(ctx, resendClient, created.Id); err == nil {
		state = contactPropertyStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[ContactPropertyState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (cp *ContactProperty) Read(ctx context.Context, req infer.ReadRequest[ContactPropertyArgs, ContactPropertyState]) (infer.ReadResponse[ContactPropertyArgs, ContactPropertyState], error) {
	if req.ID == "" {
		return infer.ReadResponse[ContactPropertyArgs, ContactPropertyState]{}, nil
	}

	resendClient, err := cp.client(ctx)
	if err != nil {
		return infer.ReadResponse[ContactPropertyArgs, ContactPropertyState]{}, err
	}

	live, err := cp.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[ContactPropertyArgs, ContactPropertyState]{}, nil
		}
		return infer.ReadResponse[ContactPropertyArgs, ContactPropertyState]{}, fmt.Errorf("read contact property %q: %w", req.ID, err)
	}

	inputs := normalizeContactPropertyInputs(req.Inputs, req.State, live)
	state := contactPropertyStateFromResponse(inputs, live)
	return infer.ReadResponse[ContactPropertyArgs, ContactPropertyState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (cp *ContactProperty) Update(ctx context.Context, req infer.UpdateRequest[ContactPropertyArgs, ContactPropertyState]) (infer.UpdateResponse[ContactPropertyState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ContactPropertyState]{
			Output: previewContactPropertyUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := cp.client(ctx)
	if err != nil {
		return infer.UpdateResponse[ContactPropertyState]{}, err
	}

	// Only fallbackValue can be updated (key and type are immutable)
	updateReq := contactPropertyUpdateRequest{
		FallbackValue: req.Inputs.FallbackValue,
	}

	path := "/contact-properties/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, updateReq, nil); err != nil {
		return infer.UpdateResponse[ContactPropertyState]{}, fmt.Errorf("update contact property %q: %w", req.ID, err)
	}

	live, err := cp.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[ContactPropertyState]{}, fmt.Errorf("read updated contact property %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[ContactPropertyState]{
		Output: contactPropertyStateFromResponse(req.Inputs, live),
	}, nil
}

func (cp *ContactProperty) Delete(ctx context.Context, req infer.DeleteRequest[ContactPropertyState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := cp.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/contact-properties/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete contact property %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*ContactProperty) Diff(ctx context.Context, req infer.DiffRequest[ContactPropertyArgs, ContactPropertyState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	requiresReplace := false

	// key is immutable after creation - any change requires replacement
	if req.Inputs.Key != req.State.Key {
		detailedDiff["key"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	// type is immutable after creation - any change requires replacement
	if req.Inputs.Type != req.State.Type {
		detailedDiff["type"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	// fallbackValue can be updated in place
	addContactPropertyOptionalStringDiff(detailedDiff, "fallbackValue", req.State.FallbackValue, req.Inputs.FallbackValue)

	return infer.DiffResponse{
		DeleteBeforeReplace: requiresReplace,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (cp *ContactProperty) client(ctx context.Context) (*client.Client, error) {
	if cp == nil || cp.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return cp.getClient(ctx)
}

func (cp *ContactProperty) get(ctx context.Context, resendClient *client.Client, id string) (contactPropertyResponse, error) {
	var response contactPropertyResponse
	path := "/contact-properties/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return contactPropertyResponse{}, err
	}
	return response, nil
}

func contactPropertyStateFromResponse(inputs ContactPropertyArgs, response contactPropertyResponse) ContactPropertyState {
	state := ContactPropertyState{
		ContactPropertyArgs: inputs,
		Id:                  response.Id,
		CreatedAt:           response.CreatedAt,
	}
	if response.Key != "" {
		state.Key = response.Key
	}
	if response.Type != "" {
		state.Type = response.Type
	}
	if response.FallbackValue != nil {
		state.FallbackValue = response.FallbackValue
	}
	return state
}

func previewContactPropertyUpdate(id string, inputs ContactPropertyArgs, oldState ContactPropertyState) ContactPropertyState {
	return ContactPropertyState{
		ContactPropertyArgs: inputs,
		Id:                  id,
		CreatedAt:           oldState.CreatedAt,
	}
}

func normalizeContactPropertyInputs(inputs ContactPropertyArgs, state ContactPropertyState, response contactPropertyResponse) ContactPropertyArgs {
	if inputs.Key == "" {
		inputs.Key = state.Key
	}
	if inputs.Key == "" {
		inputs.Key = response.Key
	}
	if inputs.Type == "" {
		inputs.Type = state.Type
	}
	if inputs.Type == "" {
		inputs.Type = response.Type
	}
	if inputs.FallbackValue == nil {
		inputs.FallbackValue = state.FallbackValue
	}
	return inputs
}

func addContactPropertyOptionalStringDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string) bool {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: contactPropertyDiffKind(oldSet, newSet), InputDiff: true}
	return true
}

func contactPropertyDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
