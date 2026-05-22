package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func cloudWaitCmd() *cobra.Command {
	var runID string
	var timeout, interval time.Duration
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Poll RunService.GetTestRun until the run is observable",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			return waitForRun(c, runID, timeout, interval)
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "run id to wait for (required)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "max wait time")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")
	_ = cmd.MarkFlagRequired("run-id")
	return cmd
}

// waitForRun polls RunService.GetTestRun until the run is observable or the
// timeout elapses.
//
// NOTE: the ui RunService does not yet surface a run's live status — the
// returned models.TestRun has no status field (status lives on the Dag,
// server-side TODO). So this confirms the run is registered and readable, then
// returns; it cannot block on a terminal verdict over the current protos.
func waitForRun(c *cloudClient, runID string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	notedDegradation := false

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s waiting for run %s to become observable", timeout, runID)
		}

		run, err := getRun(c, runID)
		if err != nil {
			fmt.Printf("\r[%s] not yet readable (retrying): %v   ", runID, err)
			time.Sleep(interval)
			continue
		}

		if !notedDegradation {
			fmt.Printf("\nRun %s is registered.\n", runID)
			fmt.Println("NOTE: live run status is not exposed by the ui RunService over the current protos;")
			fmt.Println("      cannot block on a terminal verdict. Use the dashboard/logs to track progress.")
			notedDegradation = true
		}
		_ = run
		return nil
	}
}
