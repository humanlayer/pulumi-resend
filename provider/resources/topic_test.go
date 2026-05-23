package resources

import (
	"context"
	"testing"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestTopicStateFromResponse(t *testing.T) {
	t.Parallel()

	inputs := TopicArgs{
		Name:                "Newsletter",
		DefaultSubscription: "opt_in",
		Description:         strPtr("Weekly updates"),
		Visibility:          strPtr("public"),
	}

	response := topicResponse{
		Id:                  "topic_abc123",
		Name:                "Newsletter",
		DefaultSubscription: "opt_in",
		Description:         strPtr("Weekly updates"),
		Visibility:          strPtr("public"),
		CreatedAt:           "2024-01-01T00:00:00Z",
	}

	state := topicStateFromResponse(inputs, response)

	if state.Id != "topic_abc123" {
		t.Errorf("Id = %q, want topic_abc123", state.Id)
	}
	if state.Name != "Newsletter" {
		t.Errorf("Name = %q, want Newsletter", state.Name)
	}
	if state.DefaultSubscription != "opt_in" {
		t.Errorf("DefaultSubscription = %q, want opt_in", state.DefaultSubscription)
	}
	if state.Description == nil || *state.Description != "Weekly updates" {
		t.Errorf("Description = %v, want Weekly updates", strPtrVal(state.Description))
	}
	if state.Visibility == nil || *state.Visibility != "public" {
		t.Errorf("Visibility = %v, want public", strPtrVal(state.Visibility))
	}
	if state.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", state.CreatedAt)
	}
}

func TestTopicStateFromResponseOverridesInputs(t *testing.T) {
	t.Parallel()

	inputs := TopicArgs{
		Name:                "OldName",
		DefaultSubscription: "opt_in",
	}

	response := topicResponse{
		Id:                  "topic_xyz",
		Name:                "NewName",
		DefaultSubscription: "opt_out",
		Description:         strPtr("From API"),
		Visibility:          strPtr("private"),
		CreatedAt:           "2024-06-01T00:00:00Z",
	}

	state := topicStateFromResponse(inputs, response)

	// Response values should override inputs
	if state.Name != "NewName" {
		t.Errorf("Name = %q, want NewName (from response)", state.Name)
	}
	if state.DefaultSubscription != "opt_out" {
		t.Errorf("DefaultSubscription = %q, want opt_out (from response)", state.DefaultSubscription)
	}
	if state.Description == nil || *state.Description != "From API" {
		t.Errorf("Description = %v, want From API (from response)", strPtrVal(state.Description))
	}
	if state.Visibility == nil || *state.Visibility != "private" {
		t.Errorf("Visibility = %v, want private (from response)", strPtrVal(state.Visibility))
	}
}

func TestTopicNormalizeInputs(t *testing.T) {
	t.Parallel()

	// Empty inputs should be populated from state
	inputs := TopicArgs{}
	state := TopicState{
		TopicArgs: TopicArgs{
			Name:                "state-topic",
			DefaultSubscription: "opt_in",
			Description:         strPtr("from state"),
			Visibility:          strPtr("public"),
		},
	}
	response := topicResponse{
		Id:                  "topic_resp",
		Name:                "resp-topic",
		DefaultSubscription: "opt_out",
	}

	result := normalizeTopicInputs(inputs, state, response)

	if result.Name != "state-topic" {
		t.Errorf("Name = %q, want state-topic (from state)", result.Name)
	}
	if result.DefaultSubscription != "opt_in" {
		t.Errorf("DefaultSubscription = %q, want opt_in (from state)", result.DefaultSubscription)
	}
	if result.Description == nil || *result.Description != "from state" {
		t.Errorf("Description = %v, want from state", strPtrVal(result.Description))
	}
	if result.Visibility == nil || *result.Visibility != "public" {
		t.Errorf("Visibility = %v, want public", strPtrVal(result.Visibility))
	}
}

func TestTopicNormalizeInputsFallbackToResponse(t *testing.T) {
	t.Parallel()

	// When state is also empty, should fall back to response
	inputs := TopicArgs{}
	state := TopicState{}
	response := topicResponse{
		Id:                  "topic_resp",
		Name:                "resp-topic",
		DefaultSubscription: "opt_out",
	}

	result := normalizeTopicInputs(inputs, state, response)

	if result.Name != "resp-topic" {
		t.Errorf("Name = %q, want resp-topic (from response)", result.Name)
	}
	if result.DefaultSubscription != "opt_out" {
		t.Errorf("DefaultSubscription = %q, want opt_out (from response)", result.DefaultSubscription)
	}
}

func TestTopicPreviewUpdate(t *testing.T) {
	t.Parallel()

	inputs := TopicArgs{
		Name:                "Updated Topic",
		DefaultSubscription: "opt_in",
		Description:         strPtr("Updated description"),
	}

	oldState := TopicState{
		TopicArgs: TopicArgs{
			Name:                "Old Topic",
			DefaultSubscription: "opt_in",
		},
		Id:        "topic_prev",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	result := previewTopicUpdate("topic_prev", inputs, oldState)

	if result.Id != "topic_prev" {
		t.Errorf("Id = %q, want topic_prev", result.Id)
	}
	if result.Name != "Updated Topic" {
		t.Errorf("Name = %q, want Updated Topic", result.Name)
	}
	if result.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z (preserved from old state)", result.CreatedAt)
	}
}

func TestTopicDiffNoChanges(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	resp, err := topic.Diff(context.Background(), infer.DiffRequest[TopicArgs, TopicState]{
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Newsletter",
				DefaultSubscription: "opt_in",
				Description:         strPtr("Weekly updates"),
				Visibility:          strPtr("public"),
			},
		},
		Inputs: TopicArgs{
			Name:                "Newsletter",
			DefaultSubscription: "opt_in",
			Description:         strPtr("Weekly updates"),
			Visibility:          strPtr("public"),
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if resp.HasChanges {
		t.Error("HasChanges = true, want false")
	}
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false")
	}
}

func TestTopicDiffDefaultSubscriptionRequiresReplace(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	resp, err := topic.Diff(context.Background(), infer.DiffRequest[TopicArgs, TopicState]{
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Newsletter",
				DefaultSubscription: "opt_in",
			},
		},
		Inputs: TopicArgs{
			Name:                "Newsletter",
			DefaultSubscription: "opt_out",
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if !resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = false, want true (defaultSubscription is immutable)")
	}
	if diff, ok := resp.DetailedDiff["defaultSubscription"]; !ok {
		t.Error("missing diff entry for defaultSubscription")
	} else if diff.Kind != p.UpdateReplace {
		t.Errorf("defaultSubscription diff kind = %v, want UpdateReplace", diff.Kind)
	}
}

func TestTopicDiffNameUpdateInPlace(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	resp, err := topic.Diff(context.Background(), infer.DiffRequest[TopicArgs, TopicState]{
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Old Name",
				DefaultSubscription: "opt_in",
			},
		},
		Inputs: TopicArgs{
			Name:                "New Name",
			DefaultSubscription: "opt_in",
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (name is updatable in place)")
	}
	if _, ok := resp.DetailedDiff["name"]; !ok {
		t.Error("missing diff entry for name")
	}
}

func TestTopicDiffDescriptionUpdateInPlace(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	resp, err := topic.Diff(context.Background(), infer.DiffRequest[TopicArgs, TopicState]{
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Newsletter",
				DefaultSubscription: "opt_in",
				Description:         strPtr("Old description"),
			},
		},
		Inputs: TopicArgs{
			Name:                "Newsletter",
			DefaultSubscription: "opt_in",
			Description:         strPtr("New description"),
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (description is updatable in place)")
	}
	if _, ok := resp.DetailedDiff["description"]; !ok {
		t.Error("missing diff entry for description")
	}
}

func TestTopicDiffVisibilityUpdateInPlace(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	resp, err := topic.Diff(context.Background(), infer.DiffRequest[TopicArgs, TopicState]{
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Newsletter",
				DefaultSubscription: "opt_in",
				Visibility:          strPtr("public"),
			},
		},
		Inputs: TopicArgs{
			Name:                "Newsletter",
			DefaultSubscription: "opt_in",
			Visibility:          strPtr("private"),
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = true, want false (visibility is updatable in place)")
	}
	if _, ok := resp.DetailedDiff["visibility"]; !ok {
		t.Error("missing diff entry for visibility")
	}
}

func TestTopicCreateDryRun(t *testing.T) {
	t.Parallel()

	topic := NewTopic(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called during dry run")
		return nil, nil
	})

	inputs := TopicArgs{
		Name:                "Newsletter",
		DefaultSubscription: "opt_in",
		Description:         strPtr("Weekly updates"),
	}

	resp, err := topic.Create(context.Background(), infer.CreateRequest[TopicArgs]{
		DryRun: true,
		Inputs: inputs,
	})

	if err != nil {
		t.Fatalf("Create dry run returned error: %v", err)
	}

	if resp.Output.Name != "Newsletter" {
		t.Errorf("Name = %q, want Newsletter", resp.Output.Name)
	}
	if resp.Output.DefaultSubscription != "opt_in" {
		t.Errorf("DefaultSubscription = %q, want opt_in", resp.Output.DefaultSubscription)
	}
	if resp.Output.Description == nil || *resp.Output.Description != "Weekly updates" {
		t.Errorf("Description = %v, want Weekly updates", strPtrVal(resp.Output.Description))
	}
}

func TestTopicUpdateDryRun(t *testing.T) {
	t.Parallel()

	topic := NewTopic(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called during dry run")
		return nil, nil
	})

	resp, err := topic.Update(context.Background(), infer.UpdateRequest[TopicArgs, TopicState]{
		DryRun: true,
		ID:     "topic_123",
		Inputs: TopicArgs{
			Name:                "Updated Name",
			DefaultSubscription: "opt_in",
		},
		State: TopicState{
			TopicArgs: TopicArgs{
				Name:                "Old Name",
				DefaultSubscription: "opt_in",
			},
			Id:        "topic_123",
			CreatedAt: "2024-01-01T00:00:00Z",
		},
	})

	if err != nil {
		t.Fatalf("Update dry run returned error: %v", err)
	}

	if resp.Output.Name != "Updated Name" {
		t.Errorf("Name = %q, want Updated Name", resp.Output.Name)
	}
	if resp.Output.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", resp.Output.CreatedAt)
	}
}

func TestTopicDeleteEmpty(t *testing.T) {
	t.Parallel()

	topic := NewTopic(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	_, err := topic.Delete(context.Background(), infer.DeleteRequest[TopicState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Delete with empty ID returned error: %v", err)
	}
}

func TestTopicReadEmpty(t *testing.T) {
	t.Parallel()

	topic := NewTopic(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	resp, err := topic.Read(context.Background(), infer.ReadRequest[TopicArgs, TopicState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Read with empty ID returned error: %v", err)
	}

	if resp.ID != "" {
		t.Errorf("ID = %q, want empty", resp.ID)
	}
}

func TestTopicAnnotations(t *testing.T) {
	t.Parallel()

	topic := &Topic{}
	args := &TopicArgs{}
	state := &TopicState{}

	// These should not panic
	topic.Annotate(noopAnnotator{})
	args.Annotate(noopAnnotator{})
	state.Annotate(noopAnnotator{})
}

func TestTopicOptionalStringEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        *string
		b        *string
		expected bool
	}{
		{name: "both nil", a: nil, b: nil, expected: true},
		{name: "a nil", a: nil, b: strPtr("val"), expected: false},
		{name: "b nil", a: strPtr("val"), b: nil, expected: false},
		{name: "equal", a: strPtr("val"), b: strPtr("val"), expected: true},
		{name: "not equal", a: strPtr("a"), b: strPtr("b"), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optionalStringEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("optionalStringEqual(%v, %v) = %v, want %v",
					strPtrVal(tt.a), strPtrVal(tt.b), result, tt.expected)
			}
		})
	}
}

func TestTopicDiffHelpers(t *testing.T) {
	t.Parallel()

	t.Run("addTopicStringDiff no change", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addTopicStringDiff(diff, "test", "same", "same")
		if changed {
			t.Error("addTopicStringDiff returned true for identical values")
		}
		if len(diff) != 0 {
			t.Errorf("expected empty diff, got %d entries", len(diff))
		}
	})

	t.Run("addTopicStringDiff with change", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addTopicStringDiff(diff, "name", "old", "new")
		if !changed {
			t.Error("addTopicStringDiff returned false for different values")
		}
		if len(diff) != 1 {
			t.Errorf("expected 1 diff entry, got %d", len(diff))
		}
	})

	t.Run("addTopicOptionalStringDiff no change", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addTopicOptionalStringDiff(diff, "desc", strPtr("same"), strPtr("same"))
		if changed {
			t.Error("addTopicOptionalStringDiff returned true for identical values")
		}
	})

	t.Run("addTopicOptionalStringDiff with change", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addTopicOptionalStringDiff(diff, "desc", strPtr("old"), strPtr("new"))
		if !changed {
			t.Error("addTopicOptionalStringDiff returned false for different values")
		}
		if len(diff) != 1 {
			t.Errorf("expected 1 diff entry, got %d", len(diff))
		}
	})

	t.Run("addTopicOptionalStringDiff both nil", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addTopicOptionalStringDiff(diff, "desc", nil, nil)
		if changed {
			t.Error("addTopicOptionalStringDiff returned true for both nil")
		}
	})
}

func TestTopicDiffKind(t *testing.T) {
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
			result := topicDiffKind(tt.oldSet, tt.newSet)
			if result != tt.expected {
				t.Errorf("topicDiffKind(%v, %v) = %v, want %v", tt.oldSet, tt.newSet, result, tt.expected)
			}
		})
	}
}
