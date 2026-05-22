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

type Template struct {
	getClient ClientGetter
}

type TemplateVariable struct {
	Key           string  `pulumi:"key" json:"key"`
	Type          string  `pulumi:"type" json:"type"`
	FallbackValue *string `pulumi:"fallbackValue,optional" json:"fallback_value,omitempty"`
}

type TemplateArgs struct {
	Name      string             `pulumi:"name"`
	Html      string             `pulumi:"html"`
	Alias     *string            `pulumi:"alias,optional"`
	From      *string            `pulumi:"from,optional"`
	Subject   *string            `pulumi:"subject,optional"`
	ReplyTo   []string           `pulumi:"replyTo,optional"`
	Text      *string            `pulumi:"text,optional"`
	Variables []TemplateVariable `pulumi:"variables,optional"`
}

type TemplateState struct {
	TemplateArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

type templateRequest struct {
	Name      string             `json:"name,omitempty"`
	Html      string             `json:"html,omitempty"`
	Alias     *string            `json:"alias,omitempty"`
	From      *string            `json:"from,omitempty"`
	Subject   *string            `json:"subject,omitempty"`
	ReplyTo   []string           `json:"reply_to,omitempty"`
	Text      *string            `json:"text,omitempty"`
	Variables []TemplateVariable `json:"variables,omitempty"`
}

type templateIDResponse struct {
	Id string `json:"id"`
}

type templateResponse struct {
	Id        string             `json:"id"`
	Name      string             `json:"name"`
	Html      string             `json:"html"`
	Alias     *string            `json:"alias"`
	From      *string            `json:"from"`
	Subject   *string            `json:"subject"`
	ReplyTo   []string           `json:"reply_to"`
	Text      *string            `json:"text"`
	Variables []TemplateVariable `json:"variables"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
}

func NewTemplate(getClient ClientGetter) *Template {
	return &Template{getClient: getClient}
}

func (t *Template) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Template")
	annotator.Describe(t, "A Resend email template with HTML, optional defaults, and template variables.")
}

func (variable *TemplateVariable) Annotate(annotator infer.Annotator) {
	annotator.Describe(variable, "A Resend template variable definition.")
	annotator.Describe(&variable.Key, "The template variable key.")
	annotator.Describe(&variable.Type, "The template variable type, such as string, number, boolean, object, or list.")
	annotator.Describe(&variable.FallbackValue, "Optional fallback value used when no value is supplied.")
}

func (args *TemplateArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The template name.")
	annotator.Describe(&args.Html, "The HTML body for the template.")
	annotator.Describe(&args.Alias, "Optional template alias.")
	annotator.Describe(&args.From, "Optional default sender for emails sent with the template.")
	annotator.Describe(&args.Subject, "Optional default subject for emails sent with the template.")
	annotator.Describe(&args.ReplyTo, "Optional default reply-to addresses for emails sent with the template.")
	annotator.Describe(&args.Text, "Optional plain text body for the template.")
	annotator.Describe(&args.Variables, "Optional template variable definitions.")
}

func (state *TemplateState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the template was created.")
	annotator.Describe(&state.UpdatedAt, "Timestamp when the template was last updated.")
}

func (t *Template) Create(ctx context.Context, req infer.CreateRequest[TemplateArgs]) (infer.CreateResponse[TemplateState], error) {
	if req.DryRun {
		return infer.CreateResponse[TemplateState]{
			Output: TemplateState{TemplateArgs: req.Inputs},
		}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.CreateResponse[TemplateState]{}, err
	}

	var created templateIDResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/templates", templateRequestFromArgs(req.Inputs), &created); err != nil {
		return infer.CreateResponse[TemplateState]{}, fmt.Errorf("create template %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[TemplateState]{}, errors.New("create template: response did not include id")
	}

	state := TemplateState{TemplateArgs: req.Inputs, Id: created.Id}
	if live, err := t.get(ctx, resendClient, created.Id); err == nil {
		state = templateStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[TemplateState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (t *Template) Read(ctx context.Context, req infer.ReadRequest[TemplateArgs, TemplateState]) (infer.ReadResponse[TemplateArgs, TemplateState], error) {
	if req.ID == "" {
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, err
	}

	live, err := t.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[TemplateArgs, TemplateState]{}, nil
		}
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, fmt.Errorf("read template %q: %w", req.ID, err)
	}

	inputs := normalizeTemplateInputs(req.Inputs, req.State, live)
	state := templateStateFromResponse(inputs, live)
	return infer.ReadResponse[TemplateArgs, TemplateState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (t *Template) Update(ctx context.Context, req infer.UpdateRequest[TemplateArgs, TemplateState]) (infer.UpdateResponse[TemplateState], error) {
	if req.DryRun {
		return infer.UpdateResponse[TemplateState]{
			Output: previewTemplateUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.UpdateResponse[TemplateState]{}, err
	}

	path := "/templates/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, templateRequestFromArgs(req.Inputs), nil); err != nil {
		return infer.UpdateResponse[TemplateState]{}, fmt.Errorf("update template %q: %w", req.ID, err)
	}

	live, err := t.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[TemplateState]{}, fmt.Errorf("read updated template %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[TemplateState]{
		Output: templateStateFromResponse(req.Inputs, live),
	}, nil
}

func (t *Template) Delete(ctx context.Context, req infer.DeleteRequest[TemplateState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := t.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/templates/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete template %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Template) Diff(ctx context.Context, req infer.DiffRequest[TemplateArgs, TemplateState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	addStringPropertyDiff(detailedDiff, "name", req.State.Name, req.Inputs.Name, false)
	addStringPropertyDiff(detailedDiff, "html", req.State.Html, req.Inputs.Html, false)
	addTemplateOptionalStringPropertyDiff(detailedDiff, "alias", req.State.Alias, req.Inputs.Alias)
	addTemplateOptionalStringPropertyDiff(detailedDiff, "from", req.State.From, req.Inputs.From)
	addTemplateOptionalStringPropertyDiff(detailedDiff, "subject", req.State.Subject, req.Inputs.Subject)
	addStringSlicePropertyDiff(detailedDiff, "replyTo", req.State.ReplyTo, req.Inputs.ReplyTo, false)
	addTemplateOptionalStringPropertyDiff(detailedDiff, "text", req.State.Text, req.Inputs.Text)
	addTemplateVariablesPropertyDiff(detailedDiff, "variables", req.State.Variables, req.Inputs.Variables)

	return infer.DiffResponse{
		HasChanges:   len(detailedDiff) > 0,
		DetailedDiff: detailedDiff,
	}, nil
}

func (t *Template) client(ctx context.Context) (*client.Client, error) {
	if t == nil || t.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return t.getClient(ctx)
}

func (t *Template) get(ctx context.Context, resendClient *client.Client, id string) (templateResponse, error) {
	var response templateResponse
	path := "/templates/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return templateResponse{}, err
	}
	return response, nil
}

func templateRequestFromArgs(inputs TemplateArgs) templateRequest {
	return templateRequest{
		Name:      inputs.Name,
		Html:      inputs.Html,
		Alias:     inputs.Alias,
		From:      inputs.From,
		Subject:   inputs.Subject,
		ReplyTo:   inputs.ReplyTo,
		Text:      inputs.Text,
		Variables: inputs.Variables,
	}
}

func previewTemplateUpdate(id string, inputs TemplateArgs, oldState TemplateState) TemplateState {
	return TemplateState{
		TemplateArgs: inputs,
		Id:           id,
		CreatedAt:    oldState.CreatedAt,
		UpdatedAt:    oldState.UpdatedAt,
	}
}

func templateStateFromResponse(inputs TemplateArgs, response templateResponse) TemplateState {
	state := TemplateState{
		TemplateArgs: inputs,
		Id:           response.Id,
		CreatedAt:    response.CreatedAt,
		UpdatedAt:    response.UpdatedAt,
	}
	if response.Name != "" {
		state.Name = response.Name
	}
	if response.Html != "" {
		state.Html = response.Html
	}
	state.Alias = templateOptionalString(inputs.Alias, response.Alias)
	state.From = templateOptionalString(inputs.From, response.From)
	state.Subject = templateOptionalString(inputs.Subject, response.Subject)
	if response.ReplyTo != nil && (len(response.ReplyTo) > 0 || len(inputs.ReplyTo) > 0) {
		state.ReplyTo = response.ReplyTo
	}
	state.Text = templateOptionalString(inputs.Text, response.Text)
	if response.Variables != nil {
		state.Variables = response.Variables
	}
	return state
}

func templateOptionalString(input, response *string) *string {
	if response == nil {
		return input
	}
	if input == nil && *response == "" {
		return nil
	}
	return response
}

func normalizeTemplateInputs(inputs TemplateArgs, state TemplateState, response templateResponse) TemplateArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}
	if inputs.Html == "" {
		inputs.Html = state.Html
	}
	if inputs.Html == "" {
		inputs.Html = response.Html
	}
	if inputs.Alias == nil {
		inputs.Alias = state.Alias
	}
	if inputs.From == nil {
		inputs.From = state.From
	}
	if inputs.Subject == nil {
		inputs.Subject = state.Subject
	}
	if len(inputs.ReplyTo) == 0 {
		inputs.ReplyTo = state.ReplyTo
	}
	if inputs.Text == nil {
		inputs.Text = state.Text
	}
	if len(inputs.Variables) == 0 {
		inputs.Variables = state.Variables
	}
	return inputs
}

func addStringPropertyDiff(diff map[string]p.PropertyDiff, name, oldValue, newValue string, replace bool) bool {
	if oldValue == newValue {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(oldValue != "", newValue != "", replace), InputDiff: true}
	return true
}

func addTemplateVariablesPropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue []TemplateVariable) bool {
	if reflect.DeepEqual(oldValue, newValue) {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(len(oldValue) > 0, len(newValue) > 0, false), InputDiff: true}
	return true
}

func addTemplateOptionalStringPropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string) bool {
	oldSet := oldValue != nil && *oldValue != ""
	newSet := newValue != nil && *newValue != ""
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(oldSet, newSet, false), InputDiff: true}
	return true
}
