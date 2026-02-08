package edge

import (
	"fmt"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/install"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"google.golang.org/protobuf/types/known/emptypb"
)

var ErrUnsupportedSoftware = fmt.Errorf("unsupported software")

func InstallSoftwareTask(
	c *hatchetLib.Client,
	identifier *hatchet.EdgeTasks_Identifier,
) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		TaskIdToString(identifier),
		hatchet_ext.WTask(
			func(ctx hatchetLib.Context, input *hatchet.EdgeTasks_InstallSoftware_Input) (*emptypb.Empty, error) {
				err := input.Validate()
				if err != nil {
					return nil, err
				}
				for _, software := range input.GetSoftware() {
					switch software.GetSoftware().(type) {
					case *hatchet.Software_Stroppy:
						return nil, install.Install(
							install.StroppyInstaller(software.GetStroppy()),
							software.GetSetupStrategy(),
						)
					case *hatchet.Software_Postgres:
						return nil, install.Install(
							install.PostgresInstaller(software.GetPostgres()),
							software.GetSetupStrategy(),
						)
					default:
						return nil, ErrUnsupportedSoftware
					}
				}
				return &emptypb.Empty{}, nil
			}),
	)
}
