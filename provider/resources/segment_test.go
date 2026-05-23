package resources

import (
	"context"
	"testing"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestSegmentStateFromResponse(t *testing.T) {
	t.Parallel()

	filter := SegmentFilter{"field": "email", "operator": "contains", "value": "@example.com"}
	inputs := SegmentArgs{
		Name:   "Active Users",
		Filter: &filter,
	}

	response := segmentResponse{
		Id:        "seg_abc123",
		Name:      "Active Users",
		Filter:    map[string]interface{}{"field": "email", "operator": "contains", "value": "@example.com"},
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	state := segmentStateFromResponse(inputs, response)

	if state.Id != "seg_abc123" {
		t.Errorf("Id = %q, want seg_abc123", state.Id)
	}
	if state.Name != "Active Users" {
		t.Errorf("Name = %q, want Active Users", state.Name)
	}
	if state.Filter == nil {
		t.Fatal("Filter = nil, want non-nil")
	}
	if state.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want 2024-01-01T00:00:00Z", state.CreatedAt)
	}
}

func TestSegmentStateFromResponseMinimal(t *testing.T) {
	t.Parallel()

	inputs := SegmentArgs{
		Name: "Simple Segment",
	}

	response := segmentResponse{
		Id:        "seg_minimal",
		Name:      "Simple Segment",
		CreatedAt: "2024-02-01T00:00:00Z",
	}

	state := segmentStateFromResponse(inputs, response)

	if state.Id != "seg_minimal" {
		t.Errorf("Id = %q, want seg_minimal", state.Id)
	}
	if state.Name != "Simple Segment" {
		t.Errorf("Name = %q, want Simple Segment", state.Name)
	}
	// Filter should remain nil when response filter is empty
	if state.Filter != nil && len(*state.Filter) > 0 {
		t.Errorf("Filter = %v, want nil or empty", state.Filter)
	}
}

func TestSegmentStateFromResponseOverridesInputs(t *testing.T) {
	t.Parallel()

	inputs := SegmentArgs{
		Name: "Input Name",
	}

	response := segmentResponse{
		Id:        "seg_override",
		Name:      "API Name",
		Filter:    map[string]interface{}{"from": "api"},
		CreatedAt: "2024-03-01T00:00:00Z",
	}

	state := segmentStateFromResponse(inputs, response)

	// Response name should override input name
	if state.Name != "API Name" {
		t.Errorf("Name = %q, want API Name (from response)", state.Name)
	}
	// Response filter should be set
	if state.Filter == nil {
		t.Fatal("Filter = nil, want non-nil (from response)")
	}
}

func TestSegmentNormalizeInputs(t *testing.T) {
	t.Parallel()

	// Empty inputs should be populated from state
	inputs := SegmentArgs{}
	filter := SegmentFilter{"key": "val"}
	state := SegmentState{
		SegmentArgs: SegmentArgs{
			Name:   "state-segment",
			Filter: &filter,
		},
	}
	response := segmentResponse{
		Id:   "seg_resp",
		Name: "resp-segment",
	}

	result := normalizeSegmentInputs(inputs, state, response)

	if result.Name != "state-segment" {
		t.Errorf("Name = %q, want state-segment (from state)", result.Name)
	}
	if result.Filter == nil {
		t.Fatal("Filter = nil, want non-nil (from state)")
	}
}

func TestSegmentNormalizeInputsFallbackToResponse(t *testing.T) {
	t.Parallel()

	inputs := SegmentArgs{}
	state := SegmentState{}
	response := segmentResponse{
		Id:   "seg_resp",
		Name: "resp-segment",
	}

	result := normalizeSegmentInputs(inputs, state, response)

	if result.Name != "resp-segment" {
		t.Errorf("Name = %q, want resp-segment (from response)", result.Name)
	}
}

func TestSegmentDiffNoChanges(t *testing.T) {
	t.Parallel()

	segment := &Segment{}
	filter := SegmentFilter{"field": "email"}
	resp, err := segment.Diff(context.Background(), infer.DiffRequest[SegmentArgs, SegmentState]{
		State: SegmentState{
			SegmentArgs: SegmentArgs{
				Name:   "Active Users",
				Filter: &filter,
			},
		},
		Inputs: SegmentArgs{
			Name:   "Active Users",
			Filter: &filter,
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

func TestSegmentDiffNameRequiresReplace(t *testing.T) {
	t.Parallel()

	segment := &Segment{}
	resp, err := segment.Diff(context.Background(), infer.DiffRequest[SegmentArgs, SegmentState]{
		State: SegmentState{
			SegmentArgs: SegmentArgs{
				Name: "Old Name",
			},
		},
		Inputs: SegmentArgs{
			Name: "New Name",
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if !resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = false, want true (no update API)")
	}
	if diff, ok := resp.DetailedDiff["name"]; !ok {
		t.Error("missing diff entry for name")
	} else if diff.Kind != p.UpdateReplace {
		t.Errorf("name diff kind = %v, want UpdateReplace", diff.Kind)
	}
}

func TestSegmentDiffFilterRequiresReplace(t *testing.T) {
	t.Parallel()

	segment := &Segment{}
	oldFilter := SegmentFilter{"field": "email", "operator": "contains"}
	newFilter := SegmentFilter{"field": "name", "operator": "equals"}

	resp, err := segment.Diff(context.Background(), infer.DiffRequest[SegmentArgs, SegmentState]{
		State: SegmentState{
			SegmentArgs: SegmentArgs{
				Name:   "Users",
				Filter: &oldFilter,
			},
		},
		Inputs: SegmentArgs{
			Name:   "Users",
			Filter: &newFilter,
		},
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !resp.HasChanges {
		t.Error("HasChanges = false, want true")
	}
	if !resp.DeleteBeforeReplace {
		t.Error("DeleteBeforeReplace = false, want true (no update API)")
	}
	if _, ok := resp.DetailedDiff["filter"]; !ok {
		t.Error("missing diff entry for filter")
	}
}

func TestSegmentCreateDryRun(t *testing.T) {
	t.Parallel()

	segment := NewSegment(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called during dry run")
		return nil, nil
	})

	inputs := SegmentArgs{
		Name: "Test Segment",
	}

	resp, err := segment.Create(context.Background(), infer.CreateRequest[SegmentArgs]{
		DryRun: true,
		Inputs: inputs,
	})

	if err != nil {
		t.Fatalf("Create dry run returned error: %v", err)
	}

	if resp.Output.Name != "Test Segment" {
		t.Errorf("Name = %q, want Test Segment", resp.Output.Name)
	}
}

func TestSegmentDeleteEmpty(t *testing.T) {
	t.Parallel()

	segment := NewSegment(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	_, err := segment.Delete(context.Background(), infer.DeleteRequest[SegmentState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Delete with empty ID returned error: %v", err)
	}
}

func TestSegmentReadEmpty(t *testing.T) {
	t.Parallel()

	segment := NewSegment(func(ctx context.Context) (*client.Client, error) {
		t.Fatal("client should not be called when ID is empty")
		return nil, nil
	})

	resp, err := segment.Read(context.Background(), infer.ReadRequest[SegmentArgs, SegmentState]{
		ID: "",
	})

	if err != nil {
		t.Fatalf("Read with empty ID returned error: %v", err)
	}

	if resp.ID != "" {
		t.Errorf("ID = %q, want empty", resp.ID)
	}
}

func TestSegmentAnnotations(t *testing.T) {
	t.Parallel()

	segment := &Segment{}
	args := &SegmentArgs{}
	state := &SegmentState{}

	// These should not panic
	segment.Annotate(noopAnnotator{})
	args.Annotate(noopAnnotator{})
	state.Annotate(noopAnnotator{})
}

func TestSegmentFilterDiff(t *testing.T) {
	t.Parallel()

	t.Run("both nil - no diff", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		changed := addSegmentFilterDiff(diff, "filter", nil, nil)
		if changed {
			t.Error("addSegmentFilterDiff returned true for both nil")
		}
	})

	t.Run("both empty - no diff", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		oldFilter := SegmentFilter{}
		newFilter := SegmentFilter{}
		changed := addSegmentFilterDiff(diff, "filter", &oldFilter, &newFilter)
		if changed {
			t.Error("addSegmentFilterDiff returned true for both empty")
		}
	})

	t.Run("equal filters - no diff", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		f := SegmentFilter{"key": "value"}
		changed := addSegmentFilterDiff(diff, "filter", &f, &f)
		if changed {
			t.Error("addSegmentFilterDiff returned true for equal filters")
		}
	})

	t.Run("add filter", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		newFilter := SegmentFilter{"key": "value"}
		changed := addSegmentFilterDiff(diff, "filter", nil, &newFilter)
		if !changed {
			t.Error("addSegmentFilterDiff returned false when adding filter")
		}
	})

	t.Run("remove filter", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		oldFilter := SegmentFilter{"key": "value"}
		changed := addSegmentFilterDiff(diff, "filter", &oldFilter, nil)
		if !changed {
			t.Error("addSegmentFilterDiff returned false when removing filter")
		}
	})

	t.Run("change filter", func(t *testing.T) {
		diff := make(map[string]p.PropertyDiff)
		oldFilter := SegmentFilter{"key": "old"}
		newFilter := SegmentFilter{"key": "new"}
		changed := addSegmentFilterDiff(diff, "filter", &oldFilter, &newFilter)
		if !changed {
			t.Error("addSegmentFilterDiff returned false for changed filter")
		}
	})
}

func TestSegmentFilterDiffKind(t *testing.T) {
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
			result := segmentFilterDiffKind(tt.oldSet, tt.newSet)
			if result != tt.expected {
				t.Errorf("segmentFilterDiffKind(%v, %v) = %v, want %v", tt.oldSet, tt.newSet, result, tt.expected)
			}
		})
	}
}
