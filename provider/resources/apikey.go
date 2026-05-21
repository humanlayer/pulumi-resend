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

const defaultAPIKeyPermission = "full_access"

type ClientGetter func(context.Context) (*client.Client, error)

type ApiKey struct {
	getClient ClientGetter
}

type ApiKeyArgs struct {
	Name       string  `pulumi:"name"`
	Permission *string `pulumi:"permission,optional"`
	DomainId   *string `pulumi:"domainId,optional"`
}

type ApiKeyState struct {
	ApiKeyArgs
	Id         string
	Token      string  `pulumi:"token" provider:"secret"`
	CreatedAt  string  `pulumi:"createdAt"`
	LastUsedAt *string `pulumi:"lastUsedAt,optional"`
}

type apiKeyCreateRequest struct {
	Name       string  `json:"name"`
	Permission *string `json:"permission,omitempty"`
	DomainId   *string `json:"domain_id,omitempty"`
}

type apiKeyCreateResponse struct {
	Id         string  `json:"id"`
	Token      string  `json:"token"`
	Name       string  `json:"name"`
	Permission *string `json:"permission"`
	DomainId   *string `json:"domain_id"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

type apiKeyListItem struct {
	Id         string  `json:"id"`
	Name       string  `json:"name"`
	Permission *string `json:"permission"`
	DomainId   *string `json:"domain_id"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

func NewApiKey(getClient ClientGetter) *ApiKey {
	return &ApiKey{getClient: getClient}
}

func (a *ApiKey) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "ApiKey")
	annotator.Describe(a, "A Resend API key. API key changes are replaced because Resend does not support updates.")
}

func (args *ApiKeyArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "Display name for the API key.")
	annotator.Describe(&args.Permission, "API key permission: full_access or sending_access.")
	annotator.Describe(&args.DomainId, "Optional domain ID that restricts this API key to a single domain.")
	annotator.SetDefault(&args.Permission, defaultAPIKeyPermission)
}

func (state *ApiKeyState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.Token, "Secret API key token returned only when the key is created.")
	annotator.Describe(&state.CreatedAt, "Timestamp when the API key was created.")
	annotator.Describe(&state.LastUsedAt, "Timestamp when the API key was last used, if any.")
}

func (*ApiKey) WireDependencies(f infer.FieldSelector, args *ApiKeyArgs, state *ApiKeyState) {
	f.OutputField(&state.Token).AlwaysSecret()
}

func (a *ApiKey) Create(ctx context.Context, req infer.CreateRequest[ApiKeyArgs]) (infer.CreateResponse[ApiKeyState], error) {
	if req.DryRun {
		return infer.CreateResponse[ApiKeyState]{
			Output: ApiKeyState{ApiKeyArgs: req.Inputs},
		}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.CreateResponse[ApiKeyState]{}, err
	}

	var created apiKeyCreateResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/api-keys", apiKeyCreateRequest{
		Name:       req.Inputs.Name,
		Permission: req.Inputs.Permission,
		DomainId:   req.Inputs.DomainId,
	}, &created); err != nil {
		return infer.CreateResponse[ApiKeyState]{}, fmt.Errorf("create API key: %w", err)
	}
	if created.Id == "" {
		return infer.CreateResponse[ApiKeyState]{}, errors.New("create API key: response did not include id")
	}

	state := stateFromCreateResponse(req.Inputs, created)
	if live, found, err := a.find(ctx, resendClient, created.Id); err == nil && found {
		state = stateFromListItem(req.Inputs, live, created.Token)
	}

	return infer.CreateResponse[ApiKeyState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (a *ApiKey) Read(ctx context.Context, req infer.ReadRequest[ApiKeyArgs, ApiKeyState]) (infer.ReadResponse[ApiKeyArgs, ApiKeyState], error) {
	if req.ID == "" {
		return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{}, err
	}

	live, found, err := a.find(ctx, resendClient, req.ID)
	if err != nil {
		return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{}, fmt.Errorf("read API key %q: %w", req.ID, err)
	}
	if !found {
		return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{}, nil
	}

	inputs := normalizeInputs(req.Inputs, req.State, live)
	state := stateFromListItem(inputs, live, req.State.Token)
	return infer.ReadResponse[ApiKeyArgs, ApiKeyState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (a *ApiKey) Delete(ctx context.Context, req infer.DeleteRequest[ApiKeyState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/api-keys/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete API key %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*ApiKey) Diff(ctx context.Context, req infer.DiffRequest[ApiKeyArgs, ApiKeyState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	addStringDiff(detailedDiff, "name", req.State.Name, req.Inputs.Name)
	addOptionalStringDiff(detailedDiff, "permission", req.State.Permission, req.Inputs.Permission)
	addOptionalStringDiff(detailedDiff, "domainId", req.State.DomainId, req.Inputs.DomainId)

	return infer.DiffResponse{
		DeleteBeforeReplace: len(detailedDiff) > 0,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (a *ApiKey) client(ctx context.Context) (*client.Client, error) {
	if a == nil || a.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return a.getClient(ctx)
}

func (a *ApiKey) find(ctx context.Context, resendClient *client.Client, id string) (apiKeyListItem, bool, error) {
	keys, err := client.ListAll[apiKeyListItem](ctx, resendClient, "/api-keys")
	if err != nil {
		return apiKeyListItem{}, false, err
	}
	for _, key := range keys {
		if key.Id == id {
			return key, true, nil
		}
	}
	return apiKeyListItem{}, false, nil
}

func stateFromCreateResponse(inputs ApiKeyArgs, created apiKeyCreateResponse) ApiKeyState {
	state := ApiKeyState{
		ApiKeyArgs: inputs,
		Id:         created.Id,
		Token:      created.Token,
		CreatedAt:  created.CreatedAt,
		LastUsedAt: created.LastUsedAt,
	}
	if created.Name != "" {
		state.Name = created.Name
	}
	if state.Permission == nil {
		state.Permission = created.Permission
	}
	if state.DomainId == nil {
		state.DomainId = created.DomainId
	}
	return state
}

func stateFromListItem(inputs ApiKeyArgs, item apiKeyListItem, token string) ApiKeyState {
	state := ApiKeyState{
		ApiKeyArgs: inputs,
		Id:         item.Id,
		Token:      token,
		CreatedAt:  item.CreatedAt,
		LastUsedAt: item.LastUsedAt,
	}
	if item.Name != "" {
		state.Name = item.Name
	}
	if state.Permission == nil {
		state.Permission = item.Permission
	}
	if state.DomainId == nil {
		state.DomainId = item.DomainId
	}
	return state
}

func normalizeInputs(inputs ApiKeyArgs, state ApiKeyState, item apiKeyListItem) ApiKeyArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = item.Name
	}
	if inputs.Permission == nil {
		inputs.Permission = state.Permission
	}
	if inputs.Permission == nil {
		inputs.Permission = item.Permission
	}
	if inputs.DomainId == nil {
		inputs.DomainId = state.DomainId
	}
	if inputs.DomainId == nil {
		inputs.DomainId = item.DomainId
	}
	return inputs
}

func addStringDiff(diff map[string]p.PropertyDiff, name, oldValue, newValue string) {
	if oldValue == newValue {
		return
	}
	diff[name] = p.PropertyDiff{Kind: replaceKind(oldValue != "", newValue != ""), InputDiff: true}
}

func addOptionalStringDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string) {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return
	}
	if !oldSet && !newSet {
		return
	}
	diff[name] = p.PropertyDiff{Kind: replaceKind(oldSet, newSet), InputDiff: true}
}

func replaceKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.AddReplace
	case oldSet && !newSet:
		return p.DeleteReplace
	default:
		return p.UpdateReplace
	}
}
