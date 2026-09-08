package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// AWS uses the AWS SDK v2 with static access keys.
//
// doc: https://docs.aws.amazon.com/servicequotas/latest/userguide/intro.html
// doc: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-resource-limits.html
type AWS struct{}

type awsSettings struct {
	Region           string `json:"region"`
	AvailabilityZone string `json:"availability_zone"`
}

type awsCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
}

// awsQuotaCodes are the EC2/VPC quotas a run consumes.
//
// doc: https://docs.aws.amazon.com/general/latest/gr/ec2-service.html#limits_ec2
var awsQuotaCodes = []struct{ service, code, name, unit string }{
	{"ec2", "L-1216C47A", "ec2.standard.vcpus", "vcpus"},       // Running On-Demand Standard instances
	{"ec2", "L-34B43A08", "ec2.spot.standard.vcpus", "vcpus"},  // All Standard Spot Instance Requests
	{"ebs", "L-D18FCD1D", "ebs.gp3.storage", "tib"},            // Storage for gp3 volumes
	{"ebs", "L-7A658B76", "ebs.gp2.storage", "tib"},            // Storage for gp2 volumes
	{"ebs", "L-FD252861", "ebs.io2.storage", "tib"},            // Storage for io2 volumes
	{"vpc", "L-F678F1CE", "vpc.vpcs", "vpcs"},                  // VPCs per Region
	{"vpc", "L-E79EC296", "vpc.security_groups", "groups"},     // Security groups per VPC
	{"vpc", "L-A4707A72", "vpc.internet_gateways", "gateways"}, // Internet gateways per Region
	{"ec2", "L-0263D0A3", "ec2.elastic_ips", "addresses"},      // EC2-VPC Elastic IPs
}

func (AWS) config(ctx context.Context, settings json.RawMessage, creds string) (aws.Config, awsSettings, error) {
	var st awsSettings
	if err := json.Unmarshal(settings, &st); err != nil {
		return aws.Config{}, st, fmt.Errorf("aws settings: %w", err)
	}
	if st.Region == "" {
		return aws.Config{}, st, fmt.Errorf("aws settings: region is required")
	}
	var c awsCredentials
	if err := json.Unmarshal([]byte(creds), &c); err != nil {
		return aws.Config{}, st, fmt.Errorf("aws credentials: %w", err)
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(st.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, c.SessionToken)),
	)
	if err != nil {
		return aws.Config{}, st, err
	}
	return cfg, st, nil
}

// Verify authenticates (STS GetCallerIdentity) and dry-runs the EC2 calls a
// run makes: DescribeVpcs, RunInstances (DryRun) — the API answers
// DryRunOperation when the right exists and UnauthorizedOperation when not,
// without creating anything.
func (a AWS) Verify(ctx context.Context, settings json.RawMessage, creds string, dryRun bool) (spec.ProviderVerifyResult, error) {
	cfg, st, err := a.config(ctx, settings, creds)
	if err != nil {
		return spec.ProviderVerifyResult{OK: false, Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	res := spec.ProviderVerifyResult{Scope: "region:" + st.Region}
	ident, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	res.Permissions = append(res.Permissions, permission("sts:GetCallerIdentity", err))
	if err != nil {
		res.Error = "identity: " + err.Error()
		return res, nil
	}
	res.AccountID = aws.ToString(ident.Account)
	client := ec2.NewFromConfig(cfg)
	_, err = client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{MaxResults: aws.Int32(5)})
	res.Permissions = append(res.Permissions, permission("ec2:DescribeVpcs", err))
	if !dryRun {
		// A dry-run RunInstances checks the launch right without launching.
		_, err = client.RunInstances(ctx, &ec2.RunInstancesInput{
			DryRun: aws.Bool(true), MinCount: aws.Int32(1), MaxCount: aws.Int32(1),
			InstanceType: ec2types.InstanceTypeT3Micro, ImageId: aws.String("ami-00000000000000000"),
		})
		res.Permissions = append(res.Permissions, permission("ec2:RunInstances", dryRunOutcome(err)))
	}
	var denied []string
	for _, p := range res.Permissions {
		if !p.Granted {
			denied = append(denied, p.Name)
		}
	}
	res.OK = len(denied) == 0
	if !res.OK {
		res.Error = "missing rights: " + strings.Join(denied, ", ")
	}
	return res, nil
}

// dryRunOutcome maps the EC2 dry-run answer: DryRunOperation means allowed
// (nil), UnauthorizedOperation stays an error. An invalid AMI id error also
// means the right exists — the auth check happens before parameter checks.
func dryRunOutcome(err error) error {
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "DryRunOperation", "InvalidAMIID.NotFound", "InvalidAMIID.Malformed":
			return nil
		}
	}
	return err
}

// Quotas reads the applied EC2/EBS/VPC quotas and the running vCPU usage.
func (a AWS) Quotas(ctx context.Context, settings json.RawMessage, creds, location string) (spec.QuotasResult, error) {
	cfg, st, err := a.config(ctx, settings, creds)
	if err != nil {
		return spec.QuotasResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	sq := servicequotas.NewFromConfig(cfg)
	out := spec.QuotasResult{ObservedAt: time.Now().UTC()}
	zone := location
	if zone == "" {
		zone = st.Region
	}
	usage := map[string]float64{}
	if v, err := a.runningVCPUs(ctx, ec2.NewFromConfig(cfg)); err == nil {
		usage["ec2.standard.vcpus"] = v
	}
	for _, q := range awsQuotaCodes {
		resp, err := sq.GetServiceQuota(ctx, &servicequotas.GetServiceQuotaInput{ServiceCode: aws.String(q.service), QuotaCode: aws.String(q.code)})
		if err != nil {
			// A quota the account never touched has no applied value; fall
			// back to the AWS default, and skip on any other failure.
			def, derr := sq.GetAWSDefaultServiceQuota(ctx, &servicequotas.GetAWSDefaultServiceQuotaInput{ServiceCode: aws.String(q.service), QuotaCode: aws.String(q.code)})
			if derr != nil {
				continue
			}
			resp = &servicequotas.GetServiceQuotaOutput{Quota: def.Quota}
		}
		out.Quotas = append(out.Quotas, spec.Quota{
			Name:  q.name,
			Limit: aws.ToFloat64(resp.Quota.Value),
			Used:  usage[q.name],
			Unit:  q.unit,
			Zone:  zone,
		})
	}
	return out, nil
}

// runningVCPUs sums the vCPUs of running on-demand instances in the region.
func (AWS) runningVCPUs(ctx context.Context, client *ec2.Client) (float64, error) {
	var total float64
	p := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{{Name: aws.String("instance-state-name"), Values: []string{"running", "pending"}}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		for _, r := range page.Reservations {
			for _, in := range r.Instances {
				if in.InstanceLifecycle == ec2types.InstanceLifecycleTypeSpot {
					continue
				}
				if in.CpuOptions != nil {
					total += float64(aws.ToInt32(in.CpuOptions.CoreCount) * aws.ToInt32(in.CpuOptions.ThreadsPerCore))
				}
			}
		}
	}
	return total, nil
}
