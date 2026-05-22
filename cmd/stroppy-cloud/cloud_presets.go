package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// parsePresetKind maps a --kind string to the proto enum. Empty means
// unspecified (server returns all kinds).
func parsePresetKind(s string) (models.Preset_Kind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return models.Preset_KIND_UNSPECIFIED, nil
	case "workload":
		return models.Preset_KIND_WORKLOAD, nil
	case "database", "db":
		return models.Preset_KIND_DATABASE, nil
	case "test":
		return models.Preset_KIND_TEST, nil
	default:
		return models.Preset_KIND_UNSPECIFIED, fmt.Errorf("unknown preset kind %q (want workload|database|test)", s)
	}
}

func presetKindString(k models.Preset_Kind) string {
	switch k {
	case models.Preset_KIND_WORKLOAD:
		return "workload"
	case models.Preset_KIND_DATABASE:
		return "database"
	case models.Preset_KIND_TEST:
		return "test"
	default:
		return "-"
	}
}

func cloudPresetsCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "presets",
		Short: "List presets (PresetService.ListPresets)",
		Long: `List presets for the current tenant, optionally filtered by kind.

Examples:
  stroppy-cloud cloud presets
  stroppy-cloud cloud presets --kind database`,
		RunE: func(cmd *cobra.Command, args []string) error {
			kindEnum, err := parsePresetKind(kind)
			if err != nil {
				return err
			}
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			tenantID, err := c.tenantID()
			if err != nil {
				return err
			}
			ctx, cancel := callCtx()
			defer cancel()
			req := &uipb.ListPresetRequest{TenantId: tenantID}
			if kindEnum != models.Preset_KIND_UNSPECIFIED {
				req.Kinds = []models.Preset_Kind{kindEnum}
			}
			list, err := uipb.NewPresetServiceClient(c.conn).ListPresets(ctx, req)
			if err != nil {
				return fmt.Errorf("list presets: %w", err)
			}

			presets := list.GetPresets()
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tKIND\tTAGS")
			for _, p := range presets {
				tags := strings.Join(p.GetTags().GetTags(), ",")
				if tags == "" {
					tags = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					p.GetEntity().GetId().GetValue(), presetKindString(p.GetKind()), tags)
			}
			w.Flush()
			fmt.Printf("\n%d presets\n", len(presets))
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "filter by preset kind (workload|database|test)")
	return cmd
}

func cloudProbeCmd() *cobra.Command {
	var script, driverType string
	var poolSize, scaleFactor int

	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe a stroppy script for metadata (not supported by the current server API)",
		Long: `Probe a stroppy script for steps / env vars / SQL structure.

The current ui services expose no probe RPC, so this command degrades to a
clear message rather than calling a non-existent endpoint.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("probe is not supported: the control plane exposes no probe RPC on its ui services (script=%q driver=%q pool-size=%d scale-factor=%d)",
				script, driverType, poolSize, scaleFactor)
		},
	}
	cmd.Flags().StringVar(&script, "script", "", "script name (e.g. tpcc/procs)")
	cmd.Flags().StringVar(&driverType, "driver", "", "driver type (postgres, mysql, picodata)")
	cmd.Flags().IntVar(&poolSize, "pool-size", 0, "pool size for probe")
	cmd.Flags().IntVar(&scaleFactor, "scale-factor", 0, "scale factor for probe")
	return cmd
}
