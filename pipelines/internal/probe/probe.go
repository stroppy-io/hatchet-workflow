// Package probe holds the two service pipelines that touch a tenant's cloud
// without creating anything: stroppy-provider-verify (are the credentials
// good and sufficient?) and stroppy-quotas (what are the limits and usage?).
// Both are one activity on the run worker; no agents, no machines.
package probe

import (
	"context"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/wire"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Pipeline ids.
const (
	VerifyPipelineID = "stroppy-provider-verify"
	QuotasPipelineID = "stroppy-quotas"

	nameVerify   = "stroppy.provider.verify"
	nameQuotas   = "stroppy.provider.quotas"
	probeTimeout = 3 * time.Minute
)

// Verify is the body of stroppy-provider-verify.
func Verify(ctx pipeline.Context, p spec.ProviderVerify) (spec.ProviderVerifyResult, error) {
	if ctx.Recording() {
		ctx.RecordActivity(nameVerify, verifyActivity)
		return spec.ProviderVerifyResult{}, nil
	}
	var out spec.ProviderVerifyResult
	err := workflow.ExecuteActivity(runQueue(ctx), nameVerify, p).Get(ctx, &out)
	return out, err
}

// Quotas is the body of stroppy-quotas.
func Quotas(ctx pipeline.Context, p spec.Quotas) (spec.QuotasResult, error) {
	if ctx.Recording() {
		ctx.RecordActivity(nameQuotas, quotasActivity)
		return spec.QuotasResult{}, nil
	}
	var out spec.QuotasResult
	err := workflow.ExecuteActivity(runQueue(ctx), nameQuotas, p).Get(ctx, &out)
	return out, err
}

func runQueue(ctx pipeline.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           wire.RunQueue(ctx.RunId()),
		StartToCloseTimeout: probeTimeout,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 2 * time.Second, BackoffCoefficient: 2, MaximumAttempts: 3},
	})
}

func verifyActivity(ctx context.Context, p spec.ProviderVerify) (spec.ProviderVerifyResult, error) {
	client, err := cloud.For(p.Provider)
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	creds, err := cloud.Credentials(ctx, p.CredentialsSecret)
	if err != nil {
		// A missing secret is a verification OUTCOME (the profile is broken),
		// not an activity failure to retry.
		return spec.ProviderVerifyResult{OK: false, Error: err.Error()}, nil //nolint:nilerr // reported in the result
	}
	return client.Verify(ctx, p.Settings, creds, p.DryRun)
}

func quotasActivity(ctx context.Context, p spec.Quotas) (spec.QuotasResult, error) {
	client, err := cloud.For(p.Provider)
	if err != nil {
		return spec.QuotasResult{}, err
	}
	creds, err := cloud.Credentials(ctx, p.CredentialsSecret)
	if err != nil {
		return spec.QuotasResult{}, err
	}
	return client.Quotas(ctx, p.Settings, creds, p.Location)
}
