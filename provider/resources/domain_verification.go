package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kylemistele/pulumi-resend/provider/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	domainVerificationStatusVerified = "verified"
	domainVerificationTimeout        = 5 * time.Minute
	domainVerificationPollInterval   = 10 * time.Second
)

type DomainVerification struct {
	getClient ClientGetter
}

type DomainVerificationArgs struct {
	DomainId string `pulumi:"domainId"`
}

type DomainVerificationState struct {
	DomainVerificationArgs
	Id     string
	Status string `pulumi:"status"`
}

func NewDomainVerification(getClient ClientGetter) *DomainVerification {
	return &DomainVerification{getClient: getClient}
}

func (v *DomainVerification) Annotate(annotator infer.Annotator) {
	annotator.SetToken("index", "DomainVerification")
	annotator.Describe(v, "A Resend domain verification trigger that waits for DNS verification to complete.")
}

func (args *DomainVerificationArgs) Annotate(annotator infer.Annotator) {
	annotator.Describe(&args.DomainId, "The Resend domain ID to verify.")
}

func (state *DomainVerificationState) Annotate(annotator infer.Annotator) {
	annotator.Describe(&state.Status, "The current Resend verification status for the domain.")
}

func (v *DomainVerification) Create(ctx context.Context, req infer.CreateRequest[DomainVerificationArgs]) (infer.CreateResponse[DomainVerificationState], error) {
	if req.DryRun {
		return infer.CreateResponse[DomainVerificationState]{
			ID: req.Inputs.DomainId,
			Output: DomainVerificationState{
				DomainVerificationArgs: req.Inputs,
				Id:                     req.Inputs.DomainId,
			},
		}, nil
	}

	resendClient, err := v.client(ctx)
	if err != nil {
		return infer.CreateResponse[DomainVerificationState]{}, err
	}

	state, err := v.verify(ctx, resendClient, req.Inputs.DomainId)
	if err != nil {
		return infer.CreateResponse[DomainVerificationState]{}, err
	}

	return infer.CreateResponse[DomainVerificationState]{
		ID:     state.Id,
		Output: state,
	}, nil
}

func (v *DomainVerification) Read(ctx context.Context, req infer.ReadRequest[DomainVerificationArgs, DomainVerificationState]) (infer.ReadResponse[DomainVerificationArgs, DomainVerificationState], error) {
	domainID := req.ID
	if domainID == "" {
		domainID = req.State.DomainId
	}
	if domainID == "" {
		domainID = req.Inputs.DomainId
	}
	if domainID == "" {
		return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, nil
	}

	resendClient, err := v.client(ctx)
	if err != nil {
		return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, err
	}

	live, err := getDomainForVerification(ctx, resendClient, domainID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, nil
		}
		return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, fmt.Errorf("read domain verification %q: %w", domainID, err)
	}

	inputs := req.Inputs
	if inputs.DomainId == "" {
		inputs.DomainId = domainID
	}

	return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{
		ID:     domainID,
		Inputs: inputs,
		State:  domainVerificationState(inputs, domainID, live.Status),
	}, nil
}

func (v *DomainVerification) Update(ctx context.Context, req infer.UpdateRequest[DomainVerificationArgs, DomainVerificationState]) (infer.UpdateResponse[DomainVerificationState], error) {
	if req.DryRun {
		return infer.UpdateResponse[DomainVerificationState]{
			Output: DomainVerificationState{
				DomainVerificationArgs: req.Inputs,
				Id:                     req.ID,
				Status:                 req.State.Status,
			},
		}, nil
	}

	resendClient, err := v.client(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainVerificationState]{}, err
	}

	domainID := req.Inputs.DomainId
	if domainID == "" {
		domainID = req.ID
	}
	live, err := getDomainForVerification(ctx, resendClient, domainID)
	if err != nil {
		return infer.UpdateResponse[DomainVerificationState]{}, fmt.Errorf("read domain verification %q: %w", domainID, err)
	}

	return infer.UpdateResponse[DomainVerificationState]{
		Output: domainVerificationState(req.Inputs, domainID, live.Status),
	}, nil
}

func (*DomainVerification) Delete(ctx context.Context, req infer.DeleteRequest[DomainVerificationState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func (*DomainVerification) Diff(ctx context.Context, req infer.DiffRequest[DomainVerificationArgs, DomainVerificationState]) (infer.DiffResponse, error) {
	detailedDiff := map[string]p.PropertyDiff{}
	addStringDiff(detailedDiff, "domainId", req.State.DomainId, req.Inputs.DomainId)

	return infer.DiffResponse{
		DeleteBeforeReplace: len(detailedDiff) > 0,
		HasChanges:          len(detailedDiff) > 0,
		DetailedDiff:        detailedDiff,
	}, nil
}

func (v *DomainVerification) client(ctx context.Context) (*client.Client, error) {
	if v == nil || v.getClient == nil {
		return nil, errors.New("resend client getter is not configured")
	}
	return v.getClient(ctx)
}

func (v *DomainVerification) verify(ctx context.Context, resendClient *client.Client, domainID string) (DomainVerificationState, error) {
	domainID = strings.TrimSpace(domainID)
	if domainID == "" {
		return DomainVerificationState{}, errors.New("domain verification requires domainId")
	}

	live, err := getDomainForVerification(ctx, resendClient, domainID)
	if err != nil {
		return DomainVerificationState{}, fmt.Errorf("read domain before verification %q: %w", domainID, err)
	}
	inputs := DomainVerificationArgs{DomainId: domainID}
	if isDomainVerified(live.Status) {
		return domainVerificationState(inputs, domainID, live.Status), nil
	}

	path := domainVerificationPath(domainID)
	if err := resendClient.Do(ctx, http.MethodPost, path, nil, nil); err != nil {
		return DomainVerificationState{}, fmt.Errorf("verify domain %q: %w", domainID, err)
	}

	return pollDomainVerification(ctx, resendClient, inputs, domainID)
}

func pollDomainVerification(ctx context.Context, resendClient *client.Client, inputs DomainVerificationArgs, domainID string) (DomainVerificationState, error) {
	ctx, cancel := context.WithTimeout(ctx, domainVerificationTimeout)
	defer cancel()

	for {
		live, err := getDomainForVerification(ctx, resendClient, domainID)
		if err != nil {
			return DomainVerificationState{}, fmt.Errorf("poll domain verification %q: %w", domainID, err)
		}

		state := domainVerificationState(inputs, domainID, live.Status)
		if isDomainVerified(live.Status) {
			return state, nil
		}
		if isDomainVerificationFailed(live.Status) {
			return DomainVerificationState{}, fmt.Errorf("domain %q verification failed with status %q", domainID, live.Status)
		}

		if err := waitForDomainVerificationPoll(ctx); err != nil {
			return DomainVerificationState{}, fmt.Errorf("domain %q verification timed out with status %q: %w", domainID, live.Status, err)
		}
	}
}

func waitForDomainVerificationPoll(ctx context.Context) error {
	timer := time.NewTimer(domainVerificationPollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func getDomainForVerification(ctx context.Context, resendClient *client.Client, domainID string) (domainResponse, error) {
	var response domainResponse
	if err := resendClient.Do(ctx, http.MethodGet, domainVerificationGetPath(domainID), nil, &response); err != nil {
		return domainResponse{}, err
	}
	return response, nil
}

func domainVerificationGetPath(domainID string) string {
	return "/domains/" + url.PathEscape(domainID)
}

func domainVerificationPath(domainID string) string {
	return domainVerificationGetPath(domainID) + "/verify"
}

func domainVerificationState(inputs DomainVerificationArgs, domainID, status string) DomainVerificationState {
	if inputs.DomainId == "" {
		inputs.DomainId = domainID
	}
	return DomainVerificationState{
		DomainVerificationArgs: inputs,
		Id:                     domainID,
		Status:                 status,
	}
}

func isDomainVerified(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), domainVerificationStatusVerified)
}

func isDomainVerificationFailed(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "failed" || normalized == "partially_failed"
}
