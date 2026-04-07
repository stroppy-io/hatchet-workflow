package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func cloudBenchCmd() *cobra.Command {
	var baselineConfig, candidateConfig string
	var baselineDeb, candidateDeb string
	var runA, runB string
	var format string
	var threshold float64
	var timeout, interval time.Duration

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run two configs (or wait for two runs), then compare results",
		Long: `Launch baseline and candidate runs in parallel, wait for both to complete,
then compare their metrics. Alternatively, pass existing run IDs to skip launching.

Examples:
  # Launch two configs and compare
  stroppy-cloud cloud bench --baseline pg16.json --candidate pg17.json

  # With custom .deb packages
  stroppy-cloud cloud bench \
    --baseline run.json --baseline-deb ./pg16-custom.deb \
    --candidate run.json --candidate-deb ./pg17-custom.deb

  # Compare two existing runs
  stroppy-cloud cloud bench --run-a run-123 --run-b run-456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			idA := runA
			idB := runB

			// Launch runs if configs provided.
			if baselineConfig != "" || candidateConfig != "" {
				if baselineConfig == "" || candidateConfig == "" {
					return fmt.Errorf("both --baseline and --candidate are required when launching runs")
				}

				var wg sync.WaitGroup
				var errA, errB error

				wg.Add(2)
				go func() {
					defer wg.Done()
					idA, errA = submitRun(c, baselineConfig, "", baselineDeb)
					if errA == nil {
						fmt.Printf("Baseline started: %s\n", idA)
					}
				}()
				go func() {
					defer wg.Done()
					idB, errB = submitRun(c, candidateConfig, "", candidateDeb)
					if errB == nil {
						fmt.Printf("Candidate started: %s\n", idB)
					}
				}()
				wg.Wait()

				if errA != nil {
					return fmt.Errorf("baseline launch failed: %w", errA)
				}
				if errB != nil {
					return fmt.Errorf("candidate launch failed: %w", errB)
				}
			}

			if idA == "" || idB == "" {
				return fmt.Errorf("provide either --baseline/--candidate configs or --run-a/--run-b IDs")
			}

			// Wait for both runs in parallel.
			fmt.Println("\nWaiting for both runs to complete...")
			var wg sync.WaitGroup
			var errA, errB error

			wg.Add(2)
			go func() {
				defer wg.Done()
				errA = waitForRun(c, idA, timeout, interval)
			}()
			go func() {
				defer wg.Done()
				errB = waitForRun(c, idB, timeout, interval)
			}()
			wg.Wait()

			fmt.Println() // newline after progress output
			if errA != nil {
				return fmt.Errorf("baseline run failed: %w", errA)
			}
			if errB != nil {
				return fmt.Errorf("candidate run failed: %w", errB)
			}

			// Compare.
			fmt.Println("Comparing results...")
			return runCompare(c, idA, idB, format, threshold)
		},
	}

	cmd.Flags().StringVar(&baselineConfig, "baseline", "", "path to baseline run config JSON")
	cmd.Flags().StringVar(&candidateConfig, "candidate", "", "path to candidate run config JSON")
	cmd.Flags().StringVar(&baselineDeb, "baseline-deb", "", "path to .deb for baseline run")
	cmd.Flags().StringVar(&candidateDeb, "candidate-deb", "", "path to .deb for candidate run")
	cmd.Flags().StringVar(&runA, "run-a", "", "existing baseline run ID (skip launch)")
	cmd.Flags().StringVar(&runB, "run-b", "", "existing candidate run ID (skip launch)")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, json, junit")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "custom threshold percentage (0 = server default)")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Minute, "max wait time per run")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")

	return cmd
}

// runCompare fetches and prints the comparison between two runs.
func runCompare(c *cloudHTTPClient, runA, runB, format string, threshold float64) error {
	path := fmt.Sprintf("/api/v1/compare?a=%s&b=%s",
		url.QueryEscape(runA), url.QueryEscape(runB))
	if threshold > 0 {
		path += fmt.Sprintf("&threshold=%.1f", threshold)
	}

	body, status, err := c.doJSON("GET", path, nil)
	if err != nil {
		return fmt.Errorf("compare request: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("server error %d: %s", status, string(body))
	}

	if format == "json" {
		fmt.Println(string(body))
		return nil
	}

	var result struct {
		RunA    string `json:"run_a"`
		RunB    string `json:"run_b"`
		Metrics []struct {
			Key        string  `json:"key"`
			Name       string  `json:"name"`
			Unit       string  `json:"unit"`
			AvgA       float64 `json:"avg_a"`
			AvgB       float64 `json:"avg_b"`
			DiffAvgPct float64 `json:"diff_avg_pct"`
			Verdict    string  `json:"verdict"`
		} `json:"metrics"`
		Summary struct {
			Better int `json:"better"`
			Worse  int `json:"worse"`
			Same   int `json:"same"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if format == "junit" {
		fmt.Println(`<?xml version="1.0" encoding="UTF-8"?>`)
		fmt.Printf("<testsuite name=\"stroppy-compare\" tests=\"%d\" failures=\"%d\">\n",
			len(result.Metrics), result.Summary.Worse)
		for _, m := range result.Metrics {
			fmt.Printf("  <testcase name=\"%s\" classname=\"stroppy.%s\">\n", m.Name, m.Key)
			if m.Verdict == "worse" {
				fmt.Printf("    <failure message=\"%s regressed by %.1f%%\">avg_a=%.2f avg_b=%.2f diff=%.1f%%</failure>\n",
					m.Name, m.DiffAvgPct, m.AvgA, m.AvgB, m.DiffAvgPct)
			}
			fmt.Println("  </testcase>")
		}
		fmt.Println("</testsuite>")
		return nil
	}

	// Table format (default)
	fmt.Printf("\nCompare: %s vs %s\n\n", result.RunA, result.RunB)
	fmt.Printf("%-35s %12s %12s %10s %8s\n", "METRIC", "BASELINE", "CANDIDATE", "DIFF %", "VERDICT")
	fmt.Println(strings.Repeat("-", 82))
	for _, m := range result.Metrics {
		verdict := m.Verdict
		if verdict == "same" {
			verdict = "= same"
		}
		fmt.Printf("%-35s %12.2f %12.2f %+9.1f%% %8s\n",
			m.Name, m.AvgA, m.AvgB, m.DiffAvgPct, verdict)
	}
	fmt.Printf("\nSummary: %d better, %d worse, %d same\n",
		result.Summary.Better, result.Summary.Worse, result.Summary.Same)

	if result.Summary.Worse > 2 {
		return fmt.Errorf("performance regression: %d metrics worse", result.Summary.Worse)
	}
	return nil
}
