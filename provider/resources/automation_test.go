package resources

import (
	"context"
	"testing"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestAutomationStateFromResponse(t *testing.T) {
	t.Parallel()

	disabled := "disabled"
	inputs := AutomationArgs{
		Name: "Welcome Sequence",
		Steps: []AutomationStep{
			{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "user.created"}},
			{Key: "send_welcome", Type: "send_email", Config: map[string]interface{}{"subject": "Welcome!"}},
		},
		Connections: []AutomationConnection{
			{From: "trigger", To: "send_welcome"},
		},
		Status: &disabled,
	}

	response := automationResponse{
		Id:   "auto_abc123",
		Name: "Welcome Sequence",
		Steps: []automationStepResponse{
			{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "user.created"}},
			{Key: "send_welcome", Type: "send_email", Config: map[string]interface{}{"subject": "Welcome!"}},
		},
		Connections: []automationConnectionResponse{
			{From: "trigger", To: "send_welcome"},
		},
		Status:    "disabled",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}

	state := automationStateFromResponse(inputs, response)

	if state.Id != "auto_abc123" {
		t.Errorf("Id = %q, want auto_abc123", state.Id)
	}
	if state.Name != "Welcome Sequence" {
		t.Errorf("Name = %q, want Welcome Sequence", state.Name)
	}
	if len(state.Steps) != 2 {
		t.Errorf("Steps count = %d, want 2", len(state.Steps))
	}
	if len(state.Connections) != 1 {
		t.Errorf("Connections count = %d, want 1", len(state.Connections))
	}
	if state.Status == nil || *state.Status != "disabled" {
		t.Errorf("Status = %v, want disabled", strPtrVal(state.Status))
	}
	if state.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", state.CreatedAt)
	}
	if state.UpdatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("UpdatedAt = %q, want 2024-01-01T00:00:00Z", state.UpdatedAt)
	}
}

func TestAutomationStateFromResponseOverridesInputs(t *testing.T) {
	t.Parallel()

	inputs := AutomationArgs{
		Name: "Old Name",
		Steps: []AutomationStep{
			{Key: "old_trigger", Type: "trigger", Config: map[string]interface{}{}},
		},
		Connections: []AutomationConnection{},
	}

	response := automationResponse{
		Id:   "auto_override",
		Name: "API Name",
		Steps: []automationStepResponse{
			{Key: "new_trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
			{Key: "send", Type: "send_email", Config: map[string]interface{}{}},
		},
		Connections: []automationConnectionResponse{
			{From: "new_trigger", To: "send"},
		},
		Status:    "enabled",
		CreatedAt: "2024-06-01T00:00:00Z",
		UpdatedAt: "2024-06-01T00:00:00Z",
	}

	state := automationStateFromResponse(inputs, response)

	if state.Name != "API Name" {
		t.Errorf("Name = %q, want API Name (from response)", state.Name)
	}
	if len(state.Steps) != 2 {
		t.Errorf("Steps count = %d, want 2 (from response)", len(state.Steps))
	}
	if len(state.Connections) != 1 {
		t.Errorf("Connections count = %d, want 1 (from response)", len(state.Connections))
	}
	if state.Status == nil || *state.Status != "enabled" {
		t.Errorf("Status = %v, want enabled (from response)", strPtrVal(state.Status))
	}
}

func TestAutomationNormalizeInputs(t *testing.T) {
	t.Parallel()

	disabled := "disabled"
	inputs := AutomationArgs{}
	state := AutomationState{
		AutomationArgs: AutomationArgs{
			Name: "state-automation",
			Steps: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
			},
			Connections: []AutomationConnection{},
			Status:      &disabled,
		},
	}
	response := automationResponse{
		Id:   "auto_resp",
		Name: "resp-automation",
	}

	result := normalizeAutomationInputs(inputs, state, response)

	if result.Name != "state-automation" {
		t.Errorf("Name = %q, want state-automation (from state)", result.Name)
	}
	if len(result.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1 (from state)", len(result.Steps))
	}
	if result.Status == nil || *result.Status != "disabled" {
		t.Errorf("Status = %v, want disabled (from state)", strPtrVal(result.Status))
	}
}

func TestAutomationNormalizeInputsFallbackToResponse(t *testing.T) {
	t.Parallel()

	inputs := AutomationArgs{}
	state := AutomationState{}
	response := automationResponse{
		Id:   "auto_resp",
		Name: "resp-automation",
		Steps: []automationStepResponse{
			{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
		},
		Connections: []automationConnectionResponse{
			{From: "trigger", To: "send"},
		},
		Status: "enabled",
	}

	result := normalizeAutomationInputs(inputs, state, response)

	if result.Name != "resp-automation" {
		t.Errorf("Name = %q, want resp-automation (from response)", result.Name)
	}
	if len(result.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1 (from response)", len(result.Steps))
	}
	if len(result.Connections) != 1 {
		t.Errorf("Connections count = %d, want 1 (from response)", len(result.Connections))
	}
	if result.Status == nil || *result.Status != "enabled" {
		t.Errorf("Status = %v, want enabled (from response)", strPtrVal(result.Status))
	}
}

func TestAutomationPreviewUpdate(t *testing.T) {
	t.Parallel()

	inputs := AutomationArgs{
		Name: "Updated Automation",
		Steps: []AutomationStep{
			{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
		},
		Connections: []AutomationConnection{},
	}

	oldState := AutomationState{
		AutomationArgs: AutomationArgs{Name: "Old Name"},
		Id:             "auto_prev",
		CreatedAt:      "2024-01-01T00:00:00Z",
		UpdatedAt:      "2024-01-01T12:00:00Z",
	}

	result := previewAutomationUpdate("auto_prev", inputs, oldState)

	if result.Id != "auto_prev" {
		t.Errorf("Id = %q, want auto_prev", result.Id)
	}
	if result.Name != "Updated Automation" {
		t.Errorf("Name = %q, want Updated Automation", result.Name)
	}
	if result.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z (preserved)", result.CreatedAt)
	}
	if result.UpdatedAt != "2024-01-01T12:00:00Z" {
		t.Errorf("UpdatedAt = %q, want 2024-01-01T12:00:00Z (preserved)", result.UpdatedAt)
	}
}

func TestAutomationDiffNoChanges(t *testing.T) {
	t.Parallel()

	disabled := "disabled"
	automation := &Automation{}
	steps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
		{Key: "send", Type: "send_email", Config: map[string]interface{}{"subject": "Hi"}},
	}
	conns := []AutomationConnection{
		{From: "trigger", To: "send"},
	}

	resp, err := automation.Diff(context.Background(), infer.DiffRequest[AutomationArgs, AutomationState]{
		State: AutomationState{
			AutomationArgs: AutomationArgs{
				Name:        "Automation",
				Steps:       steps,
				Connections: conns,
				Status:      &disabled,
			},
		},
		Inputs: AutomationArgs{
			Name:        "Automation",
			Steps:       steps,
			Connections: conns,
			Status:      &disabled,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if resp.HasChanges {
		t.Error("HasChanges = true, want false")
	}
}

func TestAutomationDiffNameUpdateInPlace(t *testing.T) {
	t.Parallel()

	automation := &Automation{}
	steps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
	}
	conns := []AutomationConnection{}

	resp, err := automation.Diff(context.Background(), infer.DiffRequest[AutomationArgs, AutomationState]{
		State: AutomationState{
			AutomationArgs: AutomationArgs{
				Name:        "Old Name",
				Steps:       steps,
				Connections: conns,
			},
		},
		Inputs: AutomationArgs{
			Name:        "New Name",
			Steps:       steps,
			Connections: conns,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	// Name update is in place, no replacement needed
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (name is updatable)")
	}
	if _, ok := resp.DetailedDiff["name"]; !ok {
		t.Error("missing diff entry for name")
	}
}

func TestAutomationDiffStatusUpdateInPlace(t *testing.T) {
	t.Parallel()

	automation := &Automation{}
	disabled := "disabled"
	enabled := "enabled"
	steps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
	}
	conns := []AutomationConnection{}

	resp, err := automation.Diff(context.Background(), infer.DiffRequest[AutomationArgs, AutomationState]{
		State: AutomationState{
			AutomationArgs: AutomationArgs{
				Name:        "Automation",
				Steps:       steps,
				Connections: conns,
				Status:      &disabled,
			},
		},
		Inputs: AutomationArgs{
			Name:        "Automation",
			Steps:       steps,
			Connections: conns,
			Status:      &enabled,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (status is updatable)")
	}
	if _, ok := resp.DetailedDiff["status"]; !ok {
		t.Error("missing diff entry for status")
	}
}

func TestAutomationDiffStepsUpdateInPlace(t *testing.T) {
	t.Parallel()

	automation := &Automation{}
	oldSteps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
	}
	newSteps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
		{Key: "send", Type: "send_email", Config: map[string]interface{}{"subject": "Hi"}},
	}
	conns := []AutomationConnection{}

	resp, err := automation.Diff(context.Background(), infer.DiffRequest[AutomationArgs, AutomationState]{
		State: AutomationState{
			AutomationArgs: AutomationArgs{
				Name:        "Automation",
				Steps:       oldSteps,
				Connections: conns,
			},
		},
		Inputs: AutomationArgs{
			Name:        "Automation",
			Steps:       newSteps,
			Connections: conns,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	// Steps update is in place (full graph replacement but not delete+recreate)
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (steps update in place)")
	}
	if _, ok := resp.DetailedDiff["steps"]; !ok {
		t.Error("missing diff entry for steps")
	}
}

func TestAutomationDiffConnectionsUpdateInPlace(t *testing.T) {
	t.Parallel()

	automation := &Automation{}
	steps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
		{Key: "send", Type: "send_email", Config: map[string]interface{}{}},
		{Key: "delay", Type: "delay", Config: map[string]interface{}{"duration": "1 day"}},
	}
	oldConns := []AutomationConnection{
		{From: "trigger", To: "send"},
	}
	newConns := []AutomationConnection{
		{From: "trigger", To: "send"},
		{From: "send", To: "delay"},
	}

	resp, err := automation.Diff(context.Background(), infer.DiffRequest[AutomationArgs, AutomationState]{
		State: AutomationState{
			AutomationArgs: AutomationArgs{
				Name:        "Automation",
				Steps:       steps,
				Connections: oldConns,
			},
		},
		Inputs: AutomationArgs{
			Name:        "Automation",
			Steps:       steps,
			Connections: newConns,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if _, ok := resp.DetailedDiff["connections"]; !ok {
		t.Error("missing diff entry for connections")
	}
}

func TestAutomationCreateDryRun(t *testing.T) {
	t.Parallel()

	automation := NewAutomation(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called during dry run")
		return nil, nil
	})

	disabled := "disabled"
	inputs := AutomationArgs{
		Name: "Test Automation",
		Steps: []AutomationStep{
			{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "test"}},
		},
		Connections: []AutomationConnection{},
		Status:      &disabled,
	}

	resp, err := automation.Create(context.Background(), infer.CreateRequest[AutomationArgs]{
		DryRun: true,
		Inputs: inputs,
	})

	if err != nil {
		t.Fatalf("Create dry run returned error: %v", err)
	}

	if resp.Output.Name != "Test Automation" {
		t.Errorf("Name = %q, want Test Automation", resp.Output.Name)
	}
	if len(resp.Output.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1", len(resp.Output.Steps))
	}
	if resp.Output.Status == nil || *resp.Output.Status != "disabled" {
		t.Errorf("Status = %v, want disabled", strPtrVal(resp.Output.Status))
	}
}

func TestAutomationUpdateDryRun(t *testing.T) {
	t.Parallel()

	automation := NewAutomation(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called during dry run")
		return nil, nil
	})

	enabled := "enabled"
	resp, err := automation.Update(context.Background(), infer.UpdateRequest[AutomationArgs, AutomationState]{
		DryRun: true,
		ID:     "auto_123",
		Inputs: AutomationArgs{
			Name: "Updated Automation",
			Steps: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
			},
			Connections: []AutomationConnection{},
			Status:      &enabled,
		},
		State: AutomationState{
			AutomationArgs: AutomationArgs{Name: "Old Name"},
			Id:             "auto_123",
			CreatedAt:      "2024-01-01T00:00:00Z",
			UpdatedAt:      "2024-01-01T12:00:00Z",
		},
	})

	if err != nil {
		t.Fatalf("Update dry run returned error: %v", err)
	}

	if resp.Output.Name != "Updated Automation" {
		t.Errorf("Name = %q, want Updated Automation", resp.Output.Name)
	}
	if resp.Output.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", resp.Output.CreatedAt)
	}
}

func TestAutomationDeleteEmpty(t *testing.T) {
	t.Parallel()

	automation := NewAutomation(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	_, err := automation.Delete(context.Background(), infer.DeleteRequest[AutomationState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Delete with empty ID returned error: %v", err)
	}
}

func TestAutomationReadEmpty(t *testing.T) {
	t.Parallel()

	automation := NewAutomation(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	resp, err := automation.Read(context.Background(), infer.ReadRequest[AutomationArgs, AutomationState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Read with empty ID returned error: %v", err)
	}

	if resp.ID != "" {
		t.Errorf("ID = %q, want empty", resp.ID)
	}
}

func TestAutomationAnnotations(t *testing.T) {
	t.Parallel()

	automation := &Automation{}
	args := &AutomationArgs{}
	state := &AutomationState{}
	step := &AutomationStep{}
	conn := &AutomationConnection{}

	// These should not panic
	automation.Annotate(noopAnnotator{})
	args.Annotate(noopAnnotator{})
	state.Annotate(noopAnnotator{})
	step.Annotate(noopAnnotator{})
	conn.Annotate(noopAnnotator{})
}

func TestStepsToRequest(t *testing.T) {
	t.Parallel()

	steps := []AutomationStep{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
		{Key: "send", Type: "send_email", Config: map[string]interface{}{"subject": "Welcome"}},
	}

	result := stepsToRequest(steps)

	if len(result) != 2 {
		t.Fatalf("stepsToRequest returned %d items, want 2", len(result))
	}
	if result[0].Key != "trigger" {
		t.Errorf("result[0].Key = %q, want trigger", result[0].Key)
	}
	if result[0].Type != "trigger" {
		t.Errorf("result[0].Type = %q, want trigger", result[0].Type)
	}
	if result[1].Key != "send" {
		t.Errorf("result[1].Key = %q, want send", result[1].Key)
	}
}

func TestConnectionsToRequest(t *testing.T) {
	t.Parallel()

	connType := "default"
	connections := []AutomationConnection{
		{From: "trigger", To: "send", Type: &connType},
		{From: "send", To: "delay"},
	}

	result := connectionsToRequest(connections)

	if len(result) != 2 {
		t.Fatalf("connectionsToRequest returned %d items, want 2", len(result))
	}
	if result[0].From != "trigger" {
		t.Errorf("result[0].From = %q, want trigger", result[0].From)
	}
	if result[0].To != "send" {
		t.Errorf("result[0].To = %q, want send", result[0].To)
	}
	if result[0].Type == nil || *result[0].Type != "default" {
		t.Errorf("result[0].Type = %v, want default", strPtrVal(result[0].Type))
	}
	if result[1].Type != nil {
		t.Errorf("result[1].Type = %v, want nil", strPtrVal(result[1].Type))
	}
}

func TestStepsFromResponse(t *testing.T) {
	t.Parallel()

	response := []automationStepResponse{
		{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
	}

	result := stepsFromResponse(response)

	if len(result) != 1 {
		t.Fatalf("stepsFromResponse returned %d items, want 1", len(result))
	}
	if result[0].Key != "trigger" {
		t.Errorf("result[0].Key = %q, want trigger", result[0].Key)
	}
	if result[0].Type != "trigger" {
		t.Errorf("result[0].Type = %q, want trigger", result[0].Type)
	}
}

func TestConnectionsFromResponse(t *testing.T) {
	t.Parallel()

	connType := "condition_met"
	response := []automationConnectionResponse{
		{From: "trigger", To: "send", Type: &connType},
	}

	result := connectionsFromResponse(response)

	if len(result) != 1 {
		t.Fatalf("connectionsFromResponse returned %d items, want 1", len(result))
	}
	if result[0].From != "trigger" {
		t.Errorf("result[0].From = %q, want trigger", result[0].From)
	}
	if result[0].Type == nil || *result[0].Type != "condition_met" {
		t.Errorf("result[0].Type = %v, want condition_met", strPtrVal(result[0].Type))
	}
}

func TestStepsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        []AutomationStep
		b        []AutomationStep
		expected bool
	}{
		{
			name:     "both empty",
			a:        []AutomationStep{},
			b:        []AutomationStep{},
			expected: true,
		},
		{
			name: "equal",
			a: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
			},
			b: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "signup"}},
			},
			expected: true,
		},
		{
			name: "different length",
			a: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{}},
			},
			b:        []AutomationStep{},
			expected: false,
		},
		{
			name: "different key",
			a: []AutomationStep{
				{Key: "trigger1", Type: "trigger", Config: map[string]interface{}{}},
			},
			b: []AutomationStep{
				{Key: "trigger2", Type: "trigger", Config: map[string]interface{}{}},
			},
			expected: false,
		},
		{
			name: "different type",
			a: []AutomationStep{
				{Key: "step", Type: "trigger", Config: map[string]interface{}{}},
			},
			b: []AutomationStep{
				{Key: "step", Type: "delay", Config: map[string]interface{}{}},
			},
			expected: false,
		},
		{
			name: "different config",
			a: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "a"}},
			},
			b: []AutomationStep{
				{Key: "trigger", Type: "trigger", Config: map[string]interface{}{"event_name": "b"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stepsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("stepsEqual = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConnectionsEqual(t *testing.T) {
	t.Parallel()

	defaultType := "default"
	conditionType := "condition_met"

	tests := []struct {
		name     string
		a        []AutomationConnection
		b        []AutomationConnection
		expected bool
	}{
		{
			name:     "both empty",
			a:        []AutomationConnection{},
			b:        []AutomationConnection{},
			expected: true,
		},
		{
			name: "equal without type",
			a:    []AutomationConnection{{From: "a", To: "b"}},
			b:    []AutomationConnection{{From: "a", To: "b"}},
			expected: true,
		},
		{
			name: "equal with type",
			a:    []AutomationConnection{{From: "a", To: "b", Type: &defaultType}},
			b:    []AutomationConnection{{From: "a", To: "b", Type: &defaultType}},
			expected: true,
		},
		{
			name: "different from",
			a:    []AutomationConnection{{From: "a", To: "b"}},
			b:    []AutomationConnection{{From: "c", To: "b"}},
			expected: false,
		},
		{
			name: "different to",
			a:    []AutomationConnection{{From: "a", To: "b"}},
			b:    []AutomationConnection{{From: "a", To: "c"}},
			expected: false,
		},
		{
			name: "different type",
			a:    []AutomationConnection{{From: "a", To: "b", Type: &defaultType}},
			b:    []AutomationConnection{{From: "a", To: "b", Type: &conditionType}},
			expected: false,
		},
		{
			name: "different length",
			a:    []AutomationConnection{{From: "a", To: "b"}},
			b:    []AutomationConnection{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := connectionsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("connectionsEqual = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidStepType(t *testing.T) {
	t.Parallel()

	validTypes := []string{"trigger", "send_email", "delay", "wait_for_event", "condition", "contact_update", "contact_delete", "add_to_segment"}
	for _, typ := range validTypes {
		if !isValidStepType(typ) {
			t.Errorf("isValidStepType(%q) = false, want true", typ)
		}
	}

	invalidTypes := []string{"invalid", "TRIGGER", "send-email", ""}
	for _, typ := range invalidTypes {
		if isValidStepType(typ) {
			t.Errorf("isValidStepType(%q) = true, want false", typ)
		}
	}
}

func TestIsValidConnectionType(t *testing.T) {
	t.Parallel()

	validTypes := []string{"default", "condition_met", "condition_not_met", "timeout", "event_received"}
	for _, typ := range validTypes {
		if !isValidConnectionType(typ) {
			t.Errorf("isValidConnectionType(%q) = false, want true", typ)
		}
	}

	invalidTypes := []string{"invalid", "DEFAULT", "", "custom"}
	for _, typ := range invalidTypes {
		if isValidConnectionType(typ) {
			t.Errorf("isValidConnectionType(%q) = true, want false", typ)
		}
	}
}

func TestAutomationDiffKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		oldSet   bool
		newSet   bool
		expected p.DiffKind
	}{
		{name: "add", oldSet: false, newSet: true, expected: p.Add},
		{name: "delete", oldSet: true, newSet: false, expected: p.Delete},
		{name: "update", oldSet: true, newSet: true, expected: p.Update},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := automationDiffKind(tt.oldSet, tt.newSet)
			if result != tt.expected {
				t.Errorf("automationDiffKind(%v, %v) = %v, want %v", tt.oldSet, tt.newSet, result, tt.expected)
			}
		})
	}
}
