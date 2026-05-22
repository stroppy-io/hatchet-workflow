package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
)

func cloudCompareCmd() *cobra.Command {
	var runA, runB string
	var outputFiles []string
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two runs (RunService.CompareRuns)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			return runCompare(c, runA, runB, threshold, outputFiles)
		},
	}
	cmd.Flags().StringVar(&runA, "run-a", "", "baseline run id (required)")
	cmd.Flags().StringVar(&runB, "run-b", "", "candidate run id (required)")
	cmd.Flags().StringArrayVarP(&outputFiles, "output", "o", nil, "output file (format from extension: .md .json .xml); repeatable")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "percent change above which a metric counts as better/worse")
	_ = cmd.MarkFlagRequired("run-a")
	_ = cmd.MarkFlagRequired("run-b")
	return cmd
}

func cloudBenchCmd() *cobra.Command {
	var baselineConfig, candidateConfig string
	var runA, runB string
	var outputFiles []string
	var threshold float64
	var timeout, interval time.Duration

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Submit two runs (or reuse two run ids), wait, then compare",
		Long: `Submit baseline and candidate runs from TestPreset JSON files, wait for both,
then compare their metrics. Alternatively pass existing run ids to skip launch.

The comparison table is printed; -o writes files in the format from extension:
  .md → markdown   .json → json   .xml → junit XML

Examples:
  stroppy-cloud cloud bench --baseline pg16.json --candidate pg17.json
  stroppy-cloud cloud bench --run-a run-123 --run-b run-456 -o results.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()

			idA, idB := runA, runB

			if baselineConfig != "" || candidateConfig != "" {
				if baselineConfig == "" || candidateConfig == "" {
					return fmt.Errorf("both --baseline and --candidate are required when launching runs")
				}
				var wg sync.WaitGroup
				var errA, errB error
				wg.Add(2)
				go func() {
					defer wg.Done()
					idA, errA = submitRun(c, baselineConfig, "baseline", "")
					if errA == nil {
						fmt.Printf("Baseline submitted: %s\n", idA)
					}
				}()
				go func() {
					defer wg.Done()
					idB, errB = submitRun(c, candidateConfig, "candidate", "")
					if errB == nil {
						fmt.Printf("Candidate submitted: %s\n", idB)
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
				return fmt.Errorf("provide either --baseline/--candidate configs or --run-a/--run-b ids")
			}

			fmt.Println("\nWaiting for both runs...")
			var wg sync.WaitGroup
			var errA, errB error
			wg.Add(2)
			go func() { defer wg.Done(); errA = waitForRun(c, idA, timeout, interval) }()
			go func() { defer wg.Done(); errB = waitForRun(c, idB, timeout, interval) }()
			wg.Wait()
			if errA != nil {
				return fmt.Errorf("baseline run wait failed: %w", errA)
			}
			if errB != nil {
				return fmt.Errorf("candidate run wait failed: %w", errB)
			}

			fmt.Println("Comparing results...")
			return runCompare(c, idA, idB, threshold, outputFiles)
		},
	}
	cmd.Flags().StringVar(&baselineConfig, "baseline", "", "path to baseline TestPreset JSON")
	cmd.Flags().StringVar(&candidateConfig, "candidate", "", "path to candidate TestPreset JSON")
	cmd.Flags().StringVar(&runA, "run-a", "", "existing baseline run id (skip launch)")
	cmd.Flags().StringVar(&runB, "run-b", "", "existing candidate run id (skip launch)")
	cmd.Flags().StringArrayVarP(&outputFiles, "output", "o", nil, "output file (format from extension: .md .json .xml); repeatable")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "percent change above which a metric counts as better/worse")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Minute, "max wait time per run")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "poll interval")
	return cmd
}

// runCompare calls RunService.CompareRuns, prints the table, and writes outputs.
func runCompare(c *cloudClient, runA, runB string, threshold float64, outputFiles []string) error {
	tenantID, err := c.tenantID()
	if err != nil {
		return err
	}
	ctx, cancel := callCtx()
	defer cancel()
	cmp, err := uipb.NewRunServiceClient(c.conn).CompareRuns(ctx, &uipb.CompareRunsRequest{
		TenantId:  tenantID,
		RunA:      &models.TestRunId{Value: runA},
		RunB:      &models.TestRunId{Value: runB},
		Threshold: threshold,
	})
	if err != nil {
		return fmt.Errorf("compare runs: %w", err)
	}

	var table strings.Builder
	renderTable(&table, cmp)
	fmt.Print(table.String())

	for _, outPath := range outputFiles {
		var buf strings.Builder
		switch formatFromExt(outPath) {
		case "json":
			data, merr := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(cmp)
			if merr != nil {
				return fmt.Errorf("marshal json: %w", merr)
			}
			buf.Write(data)
			buf.WriteString("\n")
		case "junit":
			renderJUnit(&buf, cmp)
		case "markdown":
			renderMarkdown(&buf, cmp)
		default:
			renderTable(&buf, cmp)
		}
		if err := os.WriteFile(outPath, []byte(buf.String()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "Saved: %s\n", outPath)
	}
	return nil
}

func formatFromExt(path string) string {
	switch filepath.Ext(path) {
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".xml":
		return "junit"
	default:
		return "table"
	}
}

func verdictString(v metrics.MetricDiff_Verdict) string {
	switch v {
	case metrics.MetricDiff_VERDICT_BETTER:
		return "better"
	case metrics.MetricDiff_VERDICT_WORSE:
		return "worse"
	case metrics.MetricDiff_VERDICT_SAME:
		return "same"
	default:
		return "-"
	}
}

func renderTable(w *strings.Builder, r *metrics.Comparison) {
	fmt.Fprintf(w, "\nCompare: %s vs %s\n\n", r.GetRunA(), r.GetRunB())
	fmt.Fprintf(w, "%-35s %12s %12s %10s %8s\n", "METRIC", "BASELINE", "CANDIDATE", "DIFF %", "VERDICT")
	fmt.Fprintln(w, strings.Repeat("-", 82))
	for _, m := range r.GetMetrics() {
		fmt.Fprintf(w, "%-35s %12.2f %12.2f %+9.1f%% %8s\n",
			m.GetName(), m.GetAvgA(), m.GetAvgB(), m.GetDiffAvgPct(), verdictString(m.GetVerdict()))
	}
	s := r.GetSummary()
	fmt.Fprintf(w, "\nSummary: %d better, %d worse, %d same\n", s.GetBetter(), s.GetWorse(), s.GetSame())
}

func renderMarkdown(w *strings.Builder, r *metrics.Comparison) {
	fmt.Fprintf(w, "## Benchmark: %s vs %s\n\n", r.GetRunA(), r.GetRunB())
	fmt.Fprintln(w, "| Metric | Baseline | Candidate | Diff % | Verdict |")
	fmt.Fprintln(w, "|--------|----------|-----------|--------|---------|")
	for _, m := range r.GetMetrics() {
		verdict := verdictString(m.GetVerdict())
		switch verdict {
		case "better":
			verdict = ":white_check_mark: better"
		case "worse":
			verdict = ":x: worse"
		case "same":
			verdict = ":heavy_minus_sign: same"
		}
		fmt.Fprintf(w, "| %s | %.2f %s | %.2f %s | %+.1f%% | %s |\n",
			m.GetName(), m.GetAvgA(), m.GetUnit(), m.GetAvgB(), m.GetUnit(), m.GetDiffAvgPct(), verdict)
	}
	s := r.GetSummary()
	fmt.Fprintf(w, "\n**Summary:** %d better, %d worse, %d same\n", s.GetBetter(), s.GetWorse(), s.GetSame())
}

func renderJUnit(w *strings.Builder, r *metrics.Comparison) {
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(w, "<testsuite name=\"stroppy-compare\" tests=\"%d\" failures=\"%d\">\n",
		len(r.GetMetrics()), r.GetSummary().GetWorse())
	for _, m := range r.GetMetrics() {
		fmt.Fprintf(w, "  <testcase name=\"%s\" classname=\"stroppy.%s\">\n", m.GetName(), m.GetKey())
		if m.GetVerdict() == metrics.MetricDiff_VERDICT_WORSE {
			fmt.Fprintf(w, "    <failure message=\"%s regressed by %.1f%%\">avg_a=%.2f avg_b=%.2f diff=%.1f%%</failure>\n",
				m.GetName(), m.GetDiffAvgPct(), m.GetAvgA(), m.GetAvgB(), m.GetDiffAvgPct())
		}
		fmt.Fprintln(w, "  </testcase>")
	}
	fmt.Fprintln(w, "</testsuite>")
}
