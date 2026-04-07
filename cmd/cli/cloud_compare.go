package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func cloudCompareCmd() *cobra.Command {
	var runA, runB, format string
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare metrics between two runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

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
			fmt.Printf("Compare: %s vs %s\n\n", result.RunA, result.RunB)
			fmt.Printf("%-35s %12s %12s %10s %8s\n", "METRIC", "RUN A", "RUN B", "DIFF %", "VERDICT")
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
		},
	}
	cmd.Flags().StringVar(&runA, "run-a", "", "first run ID (baseline)")
	cmd.Flags().StringVar(&runB, "run-b", "", "second run ID")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, json, junit")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "custom threshold percentage (0 = server default)")
	cmd.MarkFlagRequired("run-a")
	cmd.MarkFlagRequired("run-b")
	return cmd
}
