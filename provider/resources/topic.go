package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	topicSubscriptionOptIn  = "opt_in"
	topicSubscriptionOptOut = "opt_out"
	topicVisibilityPublic   = "public"
	topicVisibilityPrivate  = "private"
)

type Topic struct {
	getClient ClientGetter
}

type TopicArgs struct {
	Name                string  `pulumi:"name"`
	DefaultSubscription string  `pulumi:"defaultSubscription"`
	Description         *string `pulumi:"description,optional"`
	Visibility          *string `pulumi:"visibility,optional"`
}

type TopicState struct {
	TopicArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
}

type topicCreateRequest struct {
	Name                string  `json:"name"`
	DefaultSubscription string  `json:"default_subscription"`
	Description         *string `json:"description,omitempty"`
	Visibility          *string `json:"visibility,omitempty"`
}

type topicUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
}

type topicResponse struct {
	Id                  string  `json:"id"`
	Name                string  `json:"name"`
	DefaultSubscription string  `json:"default_subscription"`
	Description         *string `json:"description"`
	Visibility          *string `json:"visibility"`
	CreatedAt           string  `json:"created_at"`
}

func NewTopic(getClient ClientGetter) *Topic {
	return &Topic{getClient: getClient}
}

func (t *Topic) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Topic")
	annotator.Describe(t, "A Resend topic for managing subscription preferences. Topics allow contacts to control which types of emails they receive.")
}

func (args *TopicArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The name of the topic. Maximum 50 characters.")
	annotator.Describe(&args.DefaultSubscription, "The default subscription behavior for new contacts: opt_in (contacts must opt in) or opt_out (contacts are subscribed by default).")
	annotator.Describe(&args.Description, "An optional description of the topic. Maximum 200 characters.")
	annotator.Describe(&args.Visibility, "Topic visibility: public (visible in preference center) or private (hidden from preference center).")
}

func (state *TopicState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the topic was created.")
}

func (t *Topic) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[TopicArgs], error) {
	inputs, failures, err := infer.DefaultCheck[TopicArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[TopicArgs]{}, err
	}

	// Validate required fields
	if inputs.Name == "" {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "name is required",
		})
	} else if len(inputs.Name) > 50 {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "name must be 50 characters or less",
		})
	}

	if inputs.DefaultSubscription == "" {
		failures = append(failures, p.CheckFailure{
			Property: "defaultSubscription",
			Reason:   "defaultSubscription is required",
		})
	} else if inputs.DefaultSubscription != topicSubscriptionOptIn && inputs.DefaultSubscription != topicSubscriptionOptOut {
		failures = append(failures, p.CheckFailure{
			Property: "defaultSubscription",
			Reason:   fmt.Sprintf("defaultSubscription must be %q or %q", topicSubscriptionOptIn, topicSubscriptionOptOut),
		})
	}

	// Validate optional fields
	if inputs.Description != nil && len(*inputs.Description) > 200 {
		failures = append(failures, p.CheckFailure{
			Property: "description",
			Reason:   "description must be 200 characters or less",
		})
	}

	if inputs.Visibility != nil && *inputs.Visibility != topicVisibilityPublic && *inputs.Visibility != topicVisibilityPrivate {
		failures = append(failures, p.CheckFailure{
			Property: "visibility",
			Reason:   fmt.Sprintf("visibility must be %q or %q", topicVisibilityPublic, topicVisibilityPrivate),
		})
	}

	return infer.CheckResponse[TopicArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (t *Topic) Create(ctx context.Context, req infer.CreateRequest[TopicArgs]) (infer.CreateResponse[TopicState], error) {
	if req.DryRun {
		return infer.CreateResponse[TopicState]{
			Output: TopicState{TopicArgs: req.Inputs},
		}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.CreateResponse[TopicState]{}, err
	}

	var created topicResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/topics", topicCreateRequest{
		Name:                req.Inputs.Name,
		DefaultSubscription: req.Inputs.DefaultSubscription,
		Description:         req.Inputs.Description,
		Visibility:          req.Inputs.Visibility,
	}, &created); err != nil {
		return infer.CreateResponse[TopicState]{}, fmt.Errorf("create topic %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[TopicState]{}, errors.New("create topic: response did not include id")
	}

	state := topicStateFromResponse(req.Inputs, created)
	if live, err := t.get(ctx, resendClient, created.Id); err == nil {
		state = topicStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[TopicState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (t *Topic) Read(ctx context.Context, req infer.ReadRequest[TopicArgs, TopicState]) (infer.ReadResponse[TopicArgs, TopicState], error) {
	if req.ID == "" {
		return infer.ReadResponse[TopicArgs, TopicState]{}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.ReadResponse[TopicArgs, TopicState]{}, err
	}

	live, err := t.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[TopicArgs, TopicState]{}, nil
		}
		return infer.ReadResponse[TopicArgs, TopicState]{}, fmt.Errorf("read topic %q: %w", req.ID, err)
	}

	inputs := normalizeTopicInputs(req.Inputs, req.State, live)
	state := topicStateFromResponse(inputs, live)
	return infer.ReadResponse[TopicArgs, TopicState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (t *Topic) Update(ctx context.Context, req infer.UpdateRequest[TopicArgs, TopicState]) (infer.UpdateResponse[TopicState], error) {
	if req.DryRun {
		return infer.UpdateResponse[TopicState]{
			Output: previewTopicUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.UpdateResponse[TopicState]{}, err
	}

	// Build update request with only changed fields
	updateReq := topicUpdateRequest{}

	// Name can be updated
	if req.Inputs.Name != req.State.Name {
		updateReq.Name = &req.Inputs.Name
	}

	// Description can be updated
	if !optionalStringEqual(req.Inputs.Description, req.State.Description) {
		updateReq.Description = req.Inputs.Description
	}

	// Visibility can be updated
	if !optionalStringEqual(req.Inputs.Visibility, req.State.Visibility) {
		updateReq.Visibility = req.Inputs.Visibility
	}

	path := "/topics/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, updateReq, nil); err != nil {
		return infer.UpdateResponse[TopicState]{}, fmt.Errorf("update topic %q: %w", req.ID, err)
	}

	live, err := t.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[TopicState]{}, fmt.Errorf("read updated topic %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[TopicState]{
		Output: topicStateFromResponse(req.Inputs, live),
	}, nil
}

func (t *Topic) Delete(ctx context.Context, req infer.DeleteRequest[TopicState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/topics/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete topic %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Topic) Diff(ctx context.Context, req infer.DiffRequest[TopicArgs, TopicState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	requiresReplace := false

	// defaultSubscription is immutable after creation - any change requires replacement
	if req.Inputs.DefaultSubscription != req.State.DefaultSubscription {
		detailedDiff["defaultSubscription"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	// name, description, and visibility can be updated in place
	addTopicStringDiff(detailedDiff, "name", req.State.Name, req.Inputs.Name)
	addTopicOptionalStringDiff(detailedDiff, "description", req.State.Description, req.Inputs.Description)
	addTopicOptionalStringDiff(detailedDiff, "visibility", req.State.Visibility, req.Inputs.Visibility)

	return infer.DiffResponse{
		DeleteBeforeReplace: requiresReplace,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (t *Topic) client(ctx context.Context) (*client.Client, error) {
	if t == nil || t.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return t.getClient(ctx)
}

func (t *Topic) get(ctx context.Context, resendClient *client.Client, id string) (topicResponse, error) {
	var response topicResponse
	path := "/topics/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return topicResponse{}, err
	}
	return response, nil
}

func topicStateFromResponse(inputs TopicArgs, response topicResponse) TopicState {
	state := TopicState{
		TopicArgs: inputs,
		Id:        response.Id,
		CreatedAt: response.CreatedAt,
	}
	if response.Name != "" {
		state.Name = response.Name
	}
	if response.DefaultSubscription != "" {
		state.DefaultSubscription = response.DefaultSubscription
	}
	if response.Description != nil {
		state.Description = response.Description
	}
	if response.Visibility != nil {
		state.Visibility = response.Visibility
	}
	return state
}

func previewTopicUpdate(id string, inputs TopicArgs, oldState TopicState) TopicState {
	return TopicState{
		TopicArgs: inputs,
		Id:        id,
		CreatedAt: oldState.CreatedAt,
	}
}

func normalizeTopicInputs(inputs TopicArgs, state TopicState, response topicResponse) TopicArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}
	if inputs.DefaultSubscription == "" {
		inputs.DefaultSubscription = state.DefaultSubscription
	}
	if inputs.DefaultSubscription == "" {
		inputs.DefaultSubscription = response.DefaultSubscription
	}
	if inputs.Description == nil {
		inputs.Description = state.Description
	}
	if inputs.Visibility == nil {
		inputs.Visibility = state.Visibility
	}
	return inputs
}

func optionalStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func addTopicStringDiff(diff map[string]p.PropertyDiff, name, oldValue, newValue string) bool {
	if oldValue == newValue {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: topicDiffKind(oldValue != "", newValue != ""), InputDiff: true}
	return true
}

func addTopicOptionalStringDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string) bool {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: topicDiffKind(oldSet, newSet), InputDiff: true}
	return true
}

func topicDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
