package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SegmentFilter is an opaque filter object - the API doesn't fully document the schema
type SegmentFilter map[string]interface{}

type Segment struct {
	getClient ClientGetter
}

type SegmentArgs struct {
	Name   string         `pulumi:"name"`
	Filter *SegmentFilter `pulumi:"filter,optional"`
}

type SegmentState struct {
	SegmentArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
}

type segmentCreateRequest struct {
	Name   string                 `json:"name"`
	Filter map[string]interface{} `json:"filter,omitempty"`
}

type segmentResponse struct {
	Id        string                 `json:"id"`
	Name      string                 `json:"name"`
	Filter    map[string]interface{} `json:"filter"`
	CreatedAt string                 `json:"created_at"`
}

func NewSegment(getClient ClientGetter) *Segment {
	return &Segment{getClient: getClient}
}

func (s *Segment) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Segment")
	annotator.Describe(s, "A Resend segment for contact segmentation. Segments allow you to group contacts based on filter criteria for targeted broadcasts. Note: Segments are immutable after creation - any changes require replacement.")
}

func (args *SegmentArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The name of the segment. This field is immutable after creation.")
	annotator.Describe(&args.Filter, "Optional filter criteria for the segment. This is an opaque object as the API doesn't fully document the schema. This field is immutable after creation.")
}

func (state *SegmentState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the segment was created.")
}

func (s *Segment) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[SegmentArgs], error) {
	inputs, failures, err := infer.DefaultCheck[SegmentArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[SegmentArgs]{}, err
	}

	// Validate required fields
	if inputs.Name == "" {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "name is required",
		})
	}

	return infer.CheckResponse[SegmentArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (s *Segment) Create(ctx context.Context, req infer.CreateRequest[SegmentArgs]) (infer.CreateResponse[SegmentState], error) {
	if req.DryRun {
		return infer.CreateResponse[SegmentState]{
			Output: SegmentState{SegmentArgs: req.Inputs},
		}, nil
	}

	resendClient, err := s.client(ctx)
	if err != nil {
		return infer.CreateResponse[SegmentState]{}, err
	}

	createReq := segmentCreateRequest{
		Name: req.Inputs.Name,
	}
	if req.Inputs.Filter != nil {
		createReq.Filter = *req.Inputs.Filter
	}

	var created segmentResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/segments", createReq, &created); err != nil {
		return infer.CreateResponse[SegmentState]{}, fmt.Errorf("create segment %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[SegmentState]{}, errors.New("create segment: response did not include id")
	}

	state := segmentStateFromResponse(req.Inputs, created)
	if live, err := s.get(ctx, resendClient, created.Id); err == nil {
		state = segmentStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[SegmentState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (s *Segment) Read(ctx context.Context, req infer.ReadRequest[SegmentArgs, SegmentState]) (infer.ReadResponse[SegmentArgs, SegmentState], error) {
	if req.ID == "" {
		return infer.ReadResponse[SegmentArgs, SegmentState]{}, nil
	}

	resendClient, err := s.client(ctx)
	if err != nil {
		return infer.ReadResponse[SegmentArgs, SegmentState]{}, err
	}

	live, err := s.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[SegmentArgs, SegmentState]{}, nil
		}
		return infer.ReadResponse[SegmentArgs, SegmentState]{}, fmt.Errorf("read segment %q: %w", req.ID, err)
	}

	inputs := normalizeSegmentInputs(req.Inputs, req.State, live)
	state := segmentStateFromResponse(inputs, live)
	return infer.ReadResponse[SegmentArgs, SegmentState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

// Note: No Update method - segments are immutable after creation (no PATCH endpoint)

func (s *Segment) Delete(ctx context.Context, req infer.DeleteRequest[SegmentState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := s.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/segments/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete segment %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Segment) Diff(ctx context.Context, req infer.DiffRequest[SegmentArgs, SegmentState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	requiresReplace := false

	// name is immutable - any change requires replacement (no update API)
	if req.Inputs.Name != req.State.Name {
		detailedDiff["name"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	// filter is immutable - any change requires replacement (no update API)
	if addSegmentFilterDiff(detailedDiff, "filter", req.State.Filter, req.Inputs.Filter) {
		// Mark as requiring replacement since there's no PATCH endpoint
		detailedDiff["filter"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
		requiresReplace = true
	}

	return infer.DiffResponse{
		DeleteBeforeReplace: requiresReplace,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (s *Segment) client(ctx context.Context) (*client.Client, error) {
	if s == nil || s.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return s.getClient(ctx)
}

func (s *Segment) get(ctx context.Context, resendClient *client.Client, id string) (segmentResponse, error) {
	var response segmentResponse
	path := "/segments/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return segmentResponse{}, err
	}
	return response, nil
}

func segmentStateFromResponse(inputs SegmentArgs, response segmentResponse) SegmentState {
	state := SegmentState{
		SegmentArgs: inputs,
		Id:          response.Id,
		CreatedAt:   response.CreatedAt,
	}
	if response.Name != "" {
		state.Name = response.Name
	}
	if len(response.Filter) > 0 {
		filter := SegmentFilter(response.Filter)
		state.Filter = &filter
	}
	return state
}

func normalizeSegmentInputs(inputs SegmentArgs, state SegmentState, response segmentResponse) SegmentArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}
	if inputs.Filter == nil {
		inputs.Filter = state.Filter
	}
	return inputs
}

func addSegmentFilterDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *SegmentFilter) bool {
	oldSet := oldValue != nil && len(*oldValue) > 0
	newSet := newValue != nil && len(*newValue) > 0

	if !oldSet && !newSet {
		return false
	}

	if oldSet && newSet && reflect.DeepEqual(*oldValue, *newValue) {
		return false
	}

	diff[name] = p.PropertyDiff{Kind: segmentFilterDiffKind(oldSet, newSet), InputDiff: true}
	return true
}

func segmentFilterDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
