package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

func cloudWaitCmd() *cobra.Command {
	var runID string
	var timeout, interval time.Duration
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a run to complete",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			deadline := time.Now().Add(timeout)
			path := fmt.Sprintf("/api/v1/run/%s/status", url.PathEscape(runID))

			for {
				if time.Now().After(deadline) {
					return fmt.Errorf("timeout after %s", timeout)
				}

				data, status, err := c.doJSON("GET", path, nil)
				if err != nil {
					log.Printf("status check failed: %v (retrying)", err)
					time.Sleep(interval)
					continue
				}
				if status != 200 {
					log.Printf("status check HTTP %d (retrying)", status)
					time.Sleep(interval)
					continue
				}

				var snap struct {
					Nodes []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
						Error  string `json:"error,omitempty"`
					} `json:"nodes"`
				}
				if err := json.Unmarshal(data, &snap); err != nil {
					log.Printf("parse error: %v (retrying)", err)
					time.Sleep(interval)
					continue
				}

				pending, done, failed := 0, 0, 0
				for _, n := range snap.Nodes {
					switch n.Status {
					case "done":
						done++
					case "failed":
						failed++
					default:
						pending++
					}
				}

				total := len(snap.Nodes)
				fmt.Printf("\r[%d/%d] done=%d failed=%d pending=%d", done+failed, total, done, failed, pending)

				if failed > 0 {
					fmt.Println()
					for _, n := range snap.Nodes {
						if n.Status == "failed" {
							fmt.Printf("  FAILED: %s: %s\n", n.ID, n.Error)
						}
					}
					return fmt.Errorf("run failed: %d nodes failed", failed)
				}

				if pending == 0 && total > 0 {
					fmt.Printf("\nRun %s completed successfully (%d nodes)\n", runID, total)
					return nil
				}

				time.Sleep(interval)
			}
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "run ID to wait for")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "max wait time")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")
	cmd.MarkFlagRequired("run-id")
	return cmd
}
