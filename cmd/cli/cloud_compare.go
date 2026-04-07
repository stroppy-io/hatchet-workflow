package main

import (
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
			return runCompare(c, runA, runB, format, threshold)
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
