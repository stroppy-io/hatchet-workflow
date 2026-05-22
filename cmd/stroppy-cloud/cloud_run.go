package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// loadTestPreset reads a JSON file (protojson-marshalled domain.TestPreset) and
// unmarshals it into a *domain.TestPreset.
func loadTestPreset(path string) (*domain.TestPreset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	preset := &domain.TestPreset{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, preset); err != nil {
		return nil, fmt.Errorf("parse config %s as TestPreset: %w", path, err)
	}
	return preset, nil
}

func cloudRunCmd() *cobra.Command {
	var configPath, name, description string
	var wait bool
	var timeout, interval time.Duration

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Submit a test run (RunService.SubmitTestRun) from a TestPreset JSON",
		Long: `Submit a run to the control plane from a JSON config file. The config is a
protojson-marshalled cloud.v1.domain.TestPreset.

With --wait the command polls RunService.GetTestRun until the run reaches a
terminal state (see note: the ui RunService does not yet surface live run
status, so --wait confirms the run is registered and then returns).

Examples:
  stroppy-cloud cloud run -c preset.json
  stroppy-cloud cloud run -c preset.json --wait`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()

			id, err := submitRun(c, configPath, name, description)
			if err != nil {
				return err
			}
			fmt.Printf("Run submitted: %s\n", id)

			if !wait {
				return nil
			}
			return waitForRun(c, id, timeout, interval)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to TestPreset JSON (required)")
	cmd.Flags().StringVar(&name, "name", "", "optional run name")
	cmd.Flags().StringVar(&description, "description", "", "optional run description")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for the run to complete")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "max wait time (with --wait)")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval (with --wait)")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

// submitRun loads the preset, submits it scoped to the client's tenant, and
// returns the new run id.
func submitRun(c *cloudClient, configPath, name, description string) (string, error) {
	preset, err := loadTestPreset(configPath)
	if err != nil {
		return "", err
	}
	tenantID, err := c.tenantID()
	if err != nil {
		return "", err
	}

	req := &uipb.SubmitTestRunRequest{
		TenantId:   tenantID,
		TestPreset: preset,
	}
	if name != "" {
		req.Name = &name
	}
	if description != "" {
		req.Description = &description
	}

	ctx, cancel := callCtx()
	defer cancel()
	run, err := uipb.NewRunServiceClient(c.conn).SubmitTestRun(ctx, req)
	if err != nil {
		return "", fmt.Errorf("submit run: %w", err)
	}
	return run.GetEntity().GetId().GetValue(), nil
}

// getRun fetches a run by id, scoped to the client's tenant.
func getRun(c *cloudClient, runID string) (*models.TestRun, error) {
	tenantID, err := c.tenantID()
	if err != nil {
		return nil, err
	}
	ctx, cancel := callCtx()
	defer cancel()
	return uipb.NewRunServiceClient(c.conn).GetTestRun(ctx, &uipb.GetTestRunRequest{
		TenantId: tenantID,
		Id:       &models.TestRunId{Value: runID},
	})
}
