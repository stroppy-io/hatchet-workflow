package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func probeCmd() *cobra.Command {
	var (
		script       string
		driver       string
		poolSize     uint32
		version      string
		includeHuman bool
	)
	c := &cobra.Command{
		Use:   "probe",
		Short: "Probe a stroppy config on the server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.Stroppy.ProbeStroppyConfig(
				context.Background(),
				connect.NewRequest(&stroppypb.ProbeStroppyConfigRequest{
					TenantId:       &iampb.TenantId{Value: tenantID},
					Script:         script,
					DriverType:     driver,
					PoolSize:       poolSize,
					StroppyVersion: version,
					IncludeHuman:   includeHuman,
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	c.Flags().StringVar(&script, "script", "", "workload script id (e.g. tpcc/procs)")
	c.Flags().StringVar(&driver, "driver", "postgres", "driver type (postgres, mysql, picodata)")
	c.Flags().Uint32Var(&poolSize, "pool-size", 8, "connection pool size")
	c.Flags().StringVar(&version, "version", "v4.1.0", "stroppy version tag")
	c.Flags().BoolVar(&includeHuman, "include-human", false, "include human-readable probe output")
	return c
}
