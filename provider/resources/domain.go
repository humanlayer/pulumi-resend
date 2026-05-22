package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	domainCapabilitySending   = "sending"
	domainCapabilityReceiving = "receiving"
	domainCapabilityEnabled   = "enabled"
	domainCapabilityDisabled  = "disabled"
)

type Domain struct {
	getClient ClientGetter
}

type DomainArgs struct {
	Name              string   `pulumi:"name"`
	Region            *string  `pulumi:"region,optional"`
	CustomReturnPath  *string  `pulumi:"customReturnPath,optional"`
	OpenTracking      *bool    `pulumi:"openTracking,optional"`
	ClickTracking     *bool    `pulumi:"clickTracking,optional"`
	Tls               *string  `pulumi:"tls,optional"`
	Capabilities      []string `pulumi:"capabilities,optional"`
	TrackingSubdomain *string  `pulumi:"trackingSubdomain,optional"`
}

type DomainState struct {
	DomainArgs
	Id        string
	Status    string      `pulumi:"status"`
	Records   []DnsRecord `pulumi:"records"`
	CreatedAt string      `pulumi:"createdAt"`
}

type domainCapabilities struct {
	Sending   *string `json:"sending,omitempty"`
	Receiving *string `json:"receiving,omitempty"`
}

type domainCreateRequest struct {
	Name              string              `json:"name"`
	Region            *string             `json:"region,omitempty"`
	CustomReturnPath  *string             `json:"custom_return_path,omitempty"`
	OpenTracking      *bool               `json:"open_tracking,omitempty"`
	ClickTracking     *bool               `json:"click_tracking,omitempty"`
	Tls               *string             `json:"tls,omitempty"`
	Capabilities      *domainCapabilities `json:"capabilities,omitempty"`
	TrackingSubdomain *string             `json:"tracking_subdomain,omitempty"`
}

type domainUpdateRequest struct {
	OpenTracking      *bool               `json:"open_tracking,omitempty"`
	ClickTracking     *bool               `json:"click_tracking,omitempty"`
	Tls               *string             `json:"tls,omitempty"`
	Capabilities      *domainCapabilities `json:"capabilities,omitempty"`
	TrackingSubdomain *string             `json:"tracking_subdomain,omitempty"`
}

type domainResponse struct {
	Id                string              `json:"id"`
	Name              string              `json:"name"`
	Status            string              `json:"status"`
	CreatedAt         string              `json:"created_at"`
	Region            *string             `json:"region"`
	CustomReturnPath  *string             `json:"custom_return_path"`
	OpenTracking      *bool               `json:"open_tracking"`
	ClickTracking     *bool               `json:"click_tracking"`
	Tls               *string             `json:"tls"`
	Capabilities      *domainCapabilities `json:"capabilities"`
	Records           []DnsRecord         `json:"records"`
	TrackingSubdomain *string             `json:"tracking_subdomain"`
}

func NewDomain(getClient ClientGetter) *Domain {
	return &Domain{getClient: getClient}
}

func (d *Domain) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Domain")
	annotator.Describe(d, "A Resend sending domain with DNS records for verification and routing.")
}

func (args *DomainArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The domain name to manage in Resend.")
	annotator.Describe(&args.Region, "The Resend sending region for the domain, such as us-east-1, eu-west-1, sa-east-1, or ap-northeast-1.")
	annotator.Describe(&args.CustomReturnPath, "Optional custom return-path subdomain for bounce handling.")
	annotator.Describe(&args.OpenTracking, "Whether Resend should track email opens for this domain.")
	annotator.Describe(&args.ClickTracking, "Whether Resend should track link clicks for this domain.")
	annotator.Describe(&args.Tls, "TLS mode for email delivery: opportunistic or enforced.")
	annotator.Describe(&args.Capabilities, "Enabled domain capabilities. Supported values are sending and receiving.")
	annotator.Describe(&args.TrackingSubdomain, "Optional subdomain to use for click and open tracking.")
}

func (state *DomainState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.Status, "The current Resend verification status for the domain.")
	annotator.Describe(&state.Records, "DNS records Resend requires for domain verification and operation.")
	annotator.Describe(&state.CreatedAt, "Timestamp when the domain was created in Resend.")
}

func (d *Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
	if req.DryRun {
		return infer.CreateResponse[DomainState]{
			Output: DomainState{DomainArgs: req.Inputs},
		}, nil
	}

	resendClient, err := d.client(ctx)
	if err != nil {
		return infer.CreateResponse[DomainState]{}, err
	}

	var created domainResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/domains", createDomainRequest(req.Inputs), &created); err != nil {
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("create domain %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[DomainState]{}, errors.New("create domain: response did not include id")
	}

	state := domainStateFromResponse(req.Inputs, created)
	if live, err := d.get(ctx, resendClient, created.Id); err == nil {
		state = domainStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[DomainState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (d *Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
	if req.ID == "" {
		return infer.ReadResponse[DomainArgs, DomainState]{}, nil
	}

	resendClient, err := d.client(ctx)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, err
	}

	live, err := d.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[DomainArgs, DomainState]{}, nil
		}
		return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("read domain %q: %w", req.ID, err)
	}

	inputs := normalizeDomainInputs(req.Inputs, req.State, live)
	state := domainStateFromResponse(inputs, live)
	return infer.ReadResponse[DomainArgs, DomainState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (d *Domain) Update(ctx context.Context, req infer.UpdateRequest[DomainArgs, DomainState]) (infer.UpdateResponse[DomainState], error) {
	if req.DryRun {
		return infer.UpdateResponse[DomainState]{
			Output: previewDomainUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := d.client(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, err
	}

	path := "/domains/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, updateDomainRequest(req.Inputs), nil); err != nil {
		return infer.UpdateResponse[DomainState]{}, fmt.Errorf("update domain %q: %w", req.ID, err)
	}

	live, err := d.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, fmt.Errorf("read updated domain %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[DomainState]{
		Output: domainStateFromResponse(req.Inputs, live),
	}, nil
}

func (d *Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := d.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/domains/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete domain %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Domain) Diff(ctx context.Context, req infer.DiffRequest[DomainArgs, DomainState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	requiresReplace := false

	addStringDiff(detailedDiff, "name", req.State.Name, req.Inputs.Name)
	if _, ok := detailedDiff["name"]; ok {
		requiresReplace = true
	}
	if addOptionalStringPropertyDiff(detailedDiff, "region", req.State.Region, req.Inputs.Region, true) {
		requiresReplace = true
	}
	if addOptionalStringPropertyDiff(detailedDiff, "customReturnPath", req.State.CustomReturnPath, req.Inputs.CustomReturnPath, true) {
		requiresReplace = true
	}

	addOptionalBoolPropertyDiff(detailedDiff, "openTracking", req.State.OpenTracking, req.Inputs.OpenTracking, false)
	addOptionalBoolPropertyDiff(detailedDiff, "clickTracking", req.State.ClickTracking, req.Inputs.ClickTracking, false)
	addOptionalStringPropertyDiff(detailedDiff, "tls", req.State.Tls, req.Inputs.Tls, false)
	addStringSlicePropertyDiff(detailedDiff, "capabilities", req.State.Capabilities, req.Inputs.Capabilities, false)
	addOptionalStringPropertyDiff(detailedDiff, "trackingSubdomain", req.State.TrackingSubdomain, req.Inputs.TrackingSubdomain, false)

	return infer.DiffResponse{
		DeleteBeforeReplace: requiresReplace,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (d *Domain) client(ctx context.Context) (*client.Client, error) {
	if d == nil || d.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return d.getClient(ctx)
}

func (d *Domain) get(ctx context.Context, resendClient *client.Client, id string) (domainResponse, error) {
	var response domainResponse
	path := "/domains/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return domainResponse{}, err
	}
	return response, nil
}

func createDomainRequest(inputs DomainArgs) domainCreateRequest {
	return domainCreateRequest{
		Name:              inputs.Name,
		Region:            inputs.Region,
		CustomReturnPath:  inputs.CustomReturnPath,
		OpenTracking:      inputs.OpenTracking,
		ClickTracking:     inputs.ClickTracking,
		Tls:               inputs.Tls,
		Capabilities:      capabilitiesFromStrings(inputs.Capabilities),
		TrackingSubdomain: inputs.TrackingSubdomain,
	}
}

func updateDomainRequest(inputs DomainArgs) domainUpdateRequest {
	return domainUpdateRequest{
		OpenTracking:      inputs.OpenTracking,
		ClickTracking:     inputs.ClickTracking,
		Tls:               inputs.Tls,
		Capabilities:      capabilitiesFromStrings(inputs.Capabilities),
		TrackingSubdomain: inputs.TrackingSubdomain,
	}
}

func previewDomainUpdate(id string, inputs DomainArgs, oldState DomainState) DomainState {
	state := DomainState{
		DomainArgs: inputs,
		Id:         id,
		Status:     oldState.Status,
		Records:    oldState.Records,
		CreatedAt:  oldState.CreatedAt,
	}
	if state.Name == "" {
		state.Name = oldState.Name
	}
	return state
}

func domainStateFromResponse(inputs DomainArgs, response domainResponse) DomainState {
	state := DomainState{
		DomainArgs: inputs,
		Id:         response.Id,
		Status:     response.Status,
		Records:    response.Records,
		CreatedAt:  response.CreatedAt,
	}
	if response.Name != "" {
		state.Name = response.Name
	}
	if inputs.Region != nil && response.Region != nil {
		state.Region = response.Region
	}
	if inputs.CustomReturnPath != nil && response.CustomReturnPath != nil {
		state.CustomReturnPath = response.CustomReturnPath
	}
	if inputs.OpenTracking != nil && response.OpenTracking != nil {
		state.OpenTracking = response.OpenTracking
	}
	if inputs.ClickTracking != nil && response.ClickTracking != nil {
		state.ClickTracking = response.ClickTracking
	}
	if inputs.Tls != nil && response.Tls != nil {
		state.Tls = response.Tls
	}
	if len(inputs.Capabilities) > 0 {
		if capabilities := stringsFromCapabilities(response.Capabilities); len(capabilities) > 0 {
			state.Capabilities = capabilities
		}
	}
	if inputs.TrackingSubdomain != nil && response.TrackingSubdomain != nil {
		state.TrackingSubdomain = response.TrackingSubdomain
	}
	return state
}

func normalizeDomainInputs(inputs DomainArgs, state DomainState, response domainResponse) DomainArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}
	if inputs.Region == nil {
		inputs.Region = state.Region
	}
	if inputs.CustomReturnPath == nil {
		inputs.CustomReturnPath = state.CustomReturnPath
	}
	if inputs.OpenTracking == nil {
		inputs.OpenTracking = state.OpenTracking
	}
	if inputs.ClickTracking == nil {
		inputs.ClickTracking = state.ClickTracking
	}
	if inputs.Tls == nil {
		inputs.Tls = state.Tls
	}
	if len(inputs.Capabilities) == 0 {
		inputs.Capabilities = state.Capabilities
	}
	if inputs.TrackingSubdomain == nil {
		inputs.TrackingSubdomain = state.TrackingSubdomain
	}
	return inputs
}

func capabilitiesFromStrings(values []string) *domainCapabilities {
	if len(values) == 0 {
		return nil
	}

	sending := domainCapabilityDisabled
	receiving := domainCapabilityDisabled
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case domainCapabilitySending:
			sending = domainCapabilityEnabled
		case domainCapabilityReceiving:
			receiving = domainCapabilityEnabled
		}
	}

	return &domainCapabilities{
		Sending:   &sending,
		Receiving: &receiving,
	}
}

func stringsFromCapabilities(capabilities *domainCapabilities) []string {
	if capabilities == nil {
		return nil
	}

	values := []string{}
	if capabilities.Sending != nil && *capabilities.Sending == domainCapabilityEnabled {
		values = append(values, domainCapabilitySending)
	}
	if capabilities.Receiving != nil && *capabilities.Receiving == domainCapabilityEnabled {
		values = append(values, domainCapabilityReceiving)
	}
	return values
}

func addOptionalStringPropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string, replace bool) bool {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(oldSet, newSet, replace), InputDiff: true}
	return true
}

func addOptionalBoolPropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *bool, replace bool) bool {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(oldSet, newSet, replace), InputDiff: true}
	return true
}

func addStringSlicePropertyDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue []string, replace bool) bool {
	oldNormalized := normalizeStringSlice(oldValue)
	newNormalized := normalizeStringSlice(newValue)
	if slices.Equal(oldNormalized, newNormalized) {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: diffKind(len(oldNormalized) > 0, len(newNormalized) > 0, replace), InputDiff: true}
	return true
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, strings.ToLower(trimmed))
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

func diffKind(oldSet, newSet, replace bool) p.DiffKind {
	if replace {
		return replaceKind(oldSet, newSet)
	}
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
