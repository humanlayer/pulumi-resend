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

const (
	automationStatusEnabled  = "enabled"
	automationStatusDisabled = "disabled"

	// Step types
	automationStepTypeTrigger       = "trigger"
	automationStepTypeSendEmail     = "send_email"
	automationStepTypeDelay         = "delay"
	automationStepTypeWaitForEvent  = "wait_for_event"
	automationStepTypeCondition     = "condition"
	automationStepTypeContactUpdate = "contact_update"
	automationStepTypeContactDelete = "contact_delete"
	automationStepTypeAddToSegment  = "add_to_segment"

	// Connection types
	automationConnectionTypeDefault         = "default"
	automationConnectionTypeConditionMet    = "condition_met"
	automationConnectionTypeConditionNotMet = "condition_not_met"
	automationConnectionTypeTimeout         = "timeout"
	automationConnectionTypeEventReceived   = "event_received"

	// Limits
	automationMinSteps = 1
	automationMaxSteps = 150
)

var validStepTypes = []string{
	automationStepTypeTrigger,
	automationStepTypeSendEmail,
	automationStepTypeDelay,
	automationStepTypeWaitForEvent,
	automationStepTypeCondition,
	automationStepTypeContactUpdate,
	automationStepTypeContactDelete,
	automationStepTypeAddToSegment,
}

var validConnectionTypes = []string{
	automationConnectionTypeDefault,
	automationConnectionTypeConditionMet,
	automationConnectionTypeConditionNotMet,
	automationConnectionTypeTimeout,
	automationConnectionTypeEventReceived,
}

type Automation struct {
	getClient ClientGetter
}

// AutomationStep represents a step in an automation workflow
type AutomationStep struct {
	Key    string                 `pulumi:"key"`
	Type   string                 `pulumi:"type"`
	Config map[string]interface{} `pulumi:"config"`
}

// AutomationConnection represents an edge between steps in an automation workflow
type AutomationConnection struct {
	From string  `pulumi:"from"`
	To   string  `pulumi:"to"`
	Type *string `pulumi:"type,optional"`
}

type AutomationArgs struct {
	Name        string                 `pulumi:"name"`
	Steps       []AutomationStep       `pulumi:"steps"`
	Connections []AutomationConnection `pulumi:"connections"`
	Status      *string                `pulumi:"status,optional"`
}

type AutomationState struct {
	AutomationArgs
	Id        string
	CreatedAt string `pulumi:"createdAt"`
	UpdatedAt string `pulumi:"updatedAt"`
}

// API request/response types
type automationStepRequest struct {
	Key    string                 `json:"key"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

type automationConnectionRequest struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Type *string `json:"type,omitempty"`
}

type automationCreateRequest struct {
	Name        string                        `json:"name"`
	Steps       []automationStepRequest       `json:"steps"`
	Connections []automationConnectionRequest `json:"connections"`
	Status      *string                       `json:"status,omitempty"`
}

type automationUpdateRequest struct {
	Name        *string                       `json:"name,omitempty"`
	Steps       []automationStepRequest       `json:"steps,omitempty"`
	Connections []automationConnectionRequest `json:"connections,omitempty"`
	Status      *string                       `json:"status,omitempty"`
}

type automationStepResponse struct {
	Key    string                 `json:"key"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

type automationConnectionResponse struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Type *string `json:"type"`
}

type automationResponse struct {
	Id          string                         `json:"id"`
	Name        string                         `json:"name"`
	Steps       []automationStepResponse       `json:"steps"`
	Connections []automationConnectionResponse `json:"connections"`
	Status      string                         `json:"status"`
	CreatedAt   string                         `json:"created_at"`
	UpdatedAt   string                         `json:"updated_at"`
}

func NewAutomation(getClient ClientGetter) *Automation {
	return &Automation{getClient: getClient}
}

func (a *Automation) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "Automation")
	annotator.Describe(a, "A Resend automation for email automation workflows. Automations allow you to create multi-step email sequences triggered by events.")
}

func (step *AutomationStep) Annotate(annotator infer.Annotator) {
	annotator.Describe(step, "A step in a Resend automation workflow.")
	annotator.Describe(&step.Key, "Unique identifier for this step within the automation. Used to reference the step in connections.")
	annotator.Describe(&step.Type, "The type of step: trigger, send_email, delay, wait_for_event, condition, contact_update, contact_delete, or add_to_segment.")
	annotator.Describe(&step.Config, "Configuration object for the step. Structure depends on the step type.")
}

func (conn *AutomationConnection) Annotate(annotator infer.Annotator) {
	annotator.Describe(conn, "A connection (edge) between steps in a Resend automation workflow.")
	annotator.Describe(&conn.From, "The key of the source step.")
	annotator.Describe(&conn.To, "The key of the destination step.")
	annotator.Describe(&conn.Type, "The connection type: default, condition_met, condition_not_met, timeout, or event_received. Defaults to 'default'.")
}

func (args *AutomationArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.Name, "The name of the automation.")
	annotator.Describe(&args.Steps, "The steps in the automation workflow. Must contain 1-150 steps, including at least one trigger step.")
	annotator.Describe(&args.Connections, "The connections (edges) between steps defining the workflow graph.")
	annotator.Describe(&args.Status, "The automation status: 'enabled' or 'disabled'. Defaults to 'disabled'.")
}

func (state *AutomationState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.CreatedAt, "Timestamp when the automation was created.")
	annotator.Describe(&state.UpdatedAt, "Timestamp when the automation was last updated.")
}

func (a *Automation) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[AutomationArgs], error) {
	inputs, failures, err := infer.DefaultCheck[AutomationArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[AutomationArgs]{}, err
	}

	// Validate required fields
	if inputs.Name == "" {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "name is required",
		})
	}

	// Validate steps count
	stepCount := len(inputs.Steps)
	if stepCount < automationMinSteps {
		failures = append(failures, p.CheckFailure{
			Property: "steps",
			Reason:   fmt.Sprintf("steps must contain at least %d step", automationMinSteps),
		})
	} else if stepCount > automationMaxSteps {
		failures = append(failures, p.CheckFailure{
			Property: "steps",
			Reason:   fmt.Sprintf("steps must contain at most %d steps", automationMaxSteps),
		})
	}

	// Validate at least one trigger step exists
	hasTrigger := false
	stepKeys := make(map[string]bool)
	for i, step := range inputs.Steps {
		// Check for duplicate keys
		if stepKeys[step.Key] {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("steps[%d].key", i),
				Reason:   fmt.Sprintf("duplicate step key %q; step keys must be unique", step.Key),
			})
		}
		stepKeys[step.Key] = true

		// Validate step key is not empty
		if step.Key == "" {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("steps[%d].key", i),
				Reason:   "step key is required",
			})
		}

		// Validate step type
		if step.Type == "" {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("steps[%d].type", i),
				Reason:   "step type is required",
			})
		} else if !isValidStepType(step.Type) {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("steps[%d].type", i),
				Reason:   fmt.Sprintf("step type must be one of: %v", validStepTypes),
			})
		}

		if step.Type == automationStepTypeTrigger {
			hasTrigger = true
		}
	}

	if stepCount >= automationMinSteps && !hasTrigger {
		failures = append(failures, p.CheckFailure{
			Property: "steps",
			Reason:   "automation must contain at least one trigger step",
		})
	}

	// Validate connections reference valid step keys
	for i, conn := range inputs.Connections {
		if conn.From == "" {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("connections[%d].from", i),
				Reason:   "connection 'from' is required",
			})
		} else if !stepKeys[conn.From] {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("connections[%d].from", i),
				Reason:   fmt.Sprintf("connection 'from' references unknown step key %q", conn.From),
			})
		}

		if conn.To == "" {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("connections[%d].to", i),
				Reason:   "connection 'to' is required",
			})
		} else if !stepKeys[conn.To] {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("connections[%d].to", i),
				Reason:   fmt.Sprintf("connection 'to' references unknown step key %q", conn.To),
			})
		}

		// Validate connection type if provided
		if conn.Type != nil && *conn.Type != "" && !isValidConnectionType(*conn.Type) {
			failures = append(failures, p.CheckFailure{
				Property: fmt.Sprintf("connections[%d].type", i),
				Reason:   fmt.Sprintf("connection type must be one of: %v", validConnectionTypes),
			})
		}
	}

	// Validate status if provided
	if inputs.Status != nil && *inputs.Status != "" {
		if *inputs.Status != automationStatusEnabled && *inputs.Status != automationStatusDisabled {
			failures = append(failures, p.CheckFailure{
				Property: "status",
				Reason:   fmt.Sprintf("status must be %q or %q", automationStatusEnabled, automationStatusDisabled),
			})
		}
	}

	return infer.CheckResponse[AutomationArgs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

func (a *Automation) Create(ctx context.Context, req infer.CreateRequest[AutomationArgs]) (infer.CreateResponse[AutomationState], error) {
	if req.DryRun {
		return infer.CreateResponse[AutomationState]{
			Output: AutomationState{AutomationArgs: req.Inputs},
		}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.CreateResponse[AutomationState]{}, err
	}

	createReq := automationCreateRequest{
		Name:        req.Inputs.Name,
		Steps:       stepsToRequest(req.Inputs.Steps),
		Connections: connectionsToRequest(req.Inputs.Connections),
		Status:      req.Inputs.Status,
	}

	var created automationResponse
	if err := resendClient.Do(ctx, http.MethodPost, "/automations", createReq, &created); err != nil {
		return infer.CreateResponse[AutomationState]{}, fmt.Errorf("create automation %q: %w", req.Inputs.Name, err)
	}
	if created.Id == "" {
		return infer.CreateResponse[AutomationState]{}, errors.New("create automation: response did not include id")
	}

	state := automationStateFromResponse(req.Inputs, created)
	if live, err := a.get(ctx, resendClient, created.Id); err == nil {
		state = automationStateFromResponse(req.Inputs, live)
	}

	return infer.CreateResponse[AutomationState]{
		ID:     created.Id,
		Output: state,
	}, nil
}

func (a *Automation) Read(ctx context.Context, req infer.ReadRequest[AutomationArgs, AutomationState]) (infer.ReadResponse[AutomationArgs, AutomationState], error) {
	if req.ID == "" {
		return infer.ReadResponse[AutomationArgs, AutomationState]{}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.ReadResponse[AutomationArgs, AutomationState]{}, err
	}

	live, err := a.get(ctx, resendClient, req.ID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[AutomationArgs, AutomationState]{}, nil
		}
		return infer.ReadResponse[AutomationArgs, AutomationState]{}, fmt.Errorf("read automation %q: %w", req.ID, err)
	}

	inputs := normalizeAutomationInputs(req.Inputs, req.State, live)
	state := automationStateFromResponse(inputs, live)
	return infer.ReadResponse[AutomationArgs, AutomationState]{
		ID:     live.Id,
		Inputs: inputs,
		State:  state,
	}, nil
}

func (a *Automation) Update(ctx context.Context, req infer.UpdateRequest[AutomationArgs, AutomationState]) (infer.UpdateResponse[AutomationState], error) {
	if req.DryRun {
		return infer.UpdateResponse[AutomationState]{
			Output: previewAutomationUpdate(req.ID, req.Inputs, req.State),
		}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.UpdateResponse[AutomationState]{}, err
	}

	// Build update request
	updateReq := automationUpdateRequest{}

	// Name can be updated
	if req.Inputs.Name != req.State.Name {
		updateReq.Name = &req.Inputs.Name
	}

	// Status can be updated
	if !optionalStringEqual(req.Inputs.Status, req.State.Status) {
		updateReq.Status = req.Inputs.Status
	}

	// Steps and connections: if either changed, both must be sent together (full replacement)
	stepsChanged := !stepsEqual(req.Inputs.Steps, req.State.Steps)
	connectionsChanged := !connectionsEqual(req.Inputs.Connections, req.State.Connections)

	if stepsChanged || connectionsChanged {
		// Both steps and connections must be sent together for a full graph replacement
		updateReq.Steps = stepsToRequest(req.Inputs.Steps)
		updateReq.Connections = connectionsToRequest(req.Inputs.Connections)
	}

	path := "/automations/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodPatch, path, updateReq, nil); err != nil {
		return infer.UpdateResponse[AutomationState]{}, fmt.Errorf("update automation %q: %w", req.ID, err)
	}

	live, err := a.get(ctx, resendClient, req.ID)
	if err != nil {
		return infer.UpdateResponse[AutomationState]{}, fmt.Errorf("read updated automation %q: %w", req.ID, err)
	}

	return infer.UpdateResponse[AutomationState]{
		Output: automationStateFromResponse(req.Inputs, live),
	}, nil
}

func (a *Automation) Delete(ctx context.Context, req infer.DeleteRequest[AutomationState]) (infer.DeleteResponse, error) {
	if req.ID == "" {
		return infer.DeleteResponse{}, nil
	}

	resendClient, err := a.client(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	path := "/automations/" + url.PathEscape(req.ID)
	if err := resendClient.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("delete automation %q: %w", req.ID, err)
	}

	return infer.DeleteResponse{}, nil
}

func (*Automation) Diff(ctx context.Context, req infer.DiffRequest[AutomationArgs, AutomationState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}

	// name can be updated in place
	addAutomationStringDiff(detailedDiff, "name", req.State.Name, req.Inputs.Name)

	// status can be updated in place
	addAutomationOptionalStringDiff(detailedDiff, "status", req.State.Status, req.Inputs.Status)

	// steps - full replacement semantics but updates in place
	if !stepsEqual(req.Inputs.Steps, req.State.Steps) {
		detailedDiff["steps"] = p.PropertyDiff{Kind: p.Update, InputDiff: true}
	}

	// connections - full replacement semantics but updates in place
	if !connectionsEqual(req.Inputs.Connections, req.State.Connections) {
		detailedDiff["connections"] = p.PropertyDiff{Kind: p.Update, InputDiff: true}
	}

	return infer.DiffResponse{
		HasChanges:   len(detailedDiff) > 0,
		DetailedDiff: detailedDiff,
	}, nil
}

func (a *Automation) client(ctx context.Context) (*client.Client, error) {
	if a == nil || a.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return a.getClient(ctx)
}

func (a *Automation) get(ctx context.Context, resendClient *client.Client, id string) (automationResponse, error) {
	var response automationResponse
	path := "/automations/" + url.PathEscape(id)
	if err := resendClient.Do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return automationResponse{}, err
	}
	return response, nil
}

// Helper functions

func isValidStepType(t string) bool {
	for _, valid := range validStepTypes {
		if t == valid {
			return true
		}
	}
	return false
}

func isValidConnectionType(t string) bool {
	for _, valid := range validConnectionTypes {
		if t == valid {
			return true
		}
	}
	return false
}

func stepsToRequest(steps []AutomationStep) []automationStepRequest {
	result := make([]automationStepRequest, len(steps))
	for i, step := range steps {
		result[i] = automationStepRequest{
			Key:    step.Key,
			Type:   step.Type,
			Config: step.Config,
		}
	}
	return result
}

func connectionsToRequest(connections []AutomationConnection) []automationConnectionRequest {
	result := make([]automationConnectionRequest, len(connections))
	for i, conn := range connections {
		result[i] = automationConnectionRequest{
			From: conn.From,
			To:   conn.To,
			Type: conn.Type,
		}
	}
	return result
}

func stepsFromResponse(steps []automationStepResponse) []AutomationStep {
	result := make([]AutomationStep, len(steps))
	for i, step := range steps {
		result[i] = AutomationStep{
			Key:    step.Key,
			Type:   step.Type,
			Config: step.Config,
		}
	}
	return result
}

func connectionsFromResponse(connections []automationConnectionResponse) []AutomationConnection {
	result := make([]AutomationConnection, len(connections))
	for i, conn := range connections {
		result[i] = AutomationConnection{
			From: conn.From,
			To:   conn.To,
			Type: conn.Type,
		}
	}
	return result
}

func automationStateFromResponse(inputs AutomationArgs, response automationResponse) AutomationState {
	state := AutomationState{
		AutomationArgs: inputs,
		Id:             response.Id,
		CreatedAt:      response.CreatedAt,
		UpdatedAt:      response.UpdatedAt,
	}

	if response.Name != "" {
		state.Name = response.Name
	}

	if len(response.Steps) > 0 {
		state.Steps = stepsFromResponse(response.Steps)
	}

	if len(response.Connections) > 0 {
		state.Connections = connectionsFromResponse(response.Connections)
	}

	if response.Status != "" {
		state.Status = &response.Status
	}

	return state
}

func previewAutomationUpdate(id string, inputs AutomationArgs, oldState AutomationState) AutomationState {
	return AutomationState{
		AutomationArgs: inputs,
		Id:             id,
		CreatedAt:      oldState.CreatedAt,
		UpdatedAt:      oldState.UpdatedAt,
	}
}

func normalizeAutomationInputs(inputs AutomationArgs, state AutomationState, response automationResponse) AutomationArgs {
	if inputs.Name == "" {
		inputs.Name = state.Name
	}
	if inputs.Name == "" {
		inputs.Name = response.Name
	}

	if len(inputs.Steps) == 0 {
		inputs.Steps = state.Steps
	}
	if len(inputs.Steps) == 0 {
		inputs.Steps = stepsFromResponse(response.Steps)
	}

	if len(inputs.Connections) == 0 {
		inputs.Connections = state.Connections
	}
	if len(inputs.Connections) == 0 {
		inputs.Connections = connectionsFromResponse(response.Connections)
	}

	if inputs.Status == nil {
		inputs.Status = state.Status
	}
	if inputs.Status == nil && response.Status != "" {
		inputs.Status = &response.Status
	}

	return inputs
}

func stepsEqual(a, b []AutomationStep) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].Type != b[i].Type {
			return false
		}
		if !reflect.DeepEqual(a[i].Config, b[i].Config) {
			return false
		}
	}
	return true
}

func connectionsEqual(a, b []AutomationConnection) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].From != b[i].From || a[i].To != b[i].To {
			return false
		}
		if !optionalStringEqual(a[i].Type, b[i].Type) {
			return false
		}
	}
	return true
}

func addAutomationStringDiff(diff map[string]p.PropertyDiff, name, oldValue, newValue string) bool {
	if oldValue == newValue {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: automationDiffKind(oldValue != "", newValue != ""), InputDiff: true}
	return true
}

func addAutomationOptionalStringDiff(diff map[string]p.PropertyDiff, name string, oldValue, newValue *string) bool {
	oldSet := oldValue != nil
	newSet := newValue != nil
	if oldSet && newSet && *oldValue == *newValue {
		return false
	}
	if !oldSet && !newSet {
		return false
	}
	diff[name] = p.PropertyDiff{Kind: automationDiffKind(oldSet, newSet), InputDiff: true}
	return true
}

func automationDiffKind(oldSet, newSet bool) p.DiffKind {
	switch {
	case !oldSet && newSet:
		return p.Add
	case oldSet && !newSet:
		return p.Delete
	default:
		return p.Update
	}
}
