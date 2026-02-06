package edge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/envs"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
)

func streamLogsWithPrefix(ctx context.Context, r io.Reader, prefix string, log func(string), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			fmt.Println(prefix + line)
			log(prefix + line)
		}
	}
}

const (
	StroppyBinaryName = "stroppy"
	StroppyCommandGen = "gen"
	StroppyCommandRun = "run"

	StroppyWorkdirFlag = "--workdir"
	StroppyPresetFlag  = "--preset"
	TagFlag            = "--tag"

	K6RunIdTagName    = "run_id"
	K6WorkloadTagName = "workload"
)

func RunStroppyTask(c *hatchetLib.Client, identifier *hatchet.EdgeTasks_Identifier) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		TaskIdToString(identifier),
		func(ctx hatchetLib.Context, input *hatchet.EdgeTasks_RunStroppy_Input) (*hatchet.EdgeTasks_RunStroppy_Output, error) {
			runcmd := func(cmd *exec.Cmd) error {
				stdout, _ := cmd.StdoutPipe()
				stderr, _ := cmd.StderrPipe()
				err := cmd.Start()
				if err != nil {
					return err
				}
				var wg sync.WaitGroup
				wg.Add(2)
				go streamLogsWithPrefix(ctx, stdout, "[OUT]: ", ctx.Log, &wg)
				go streamLogsWithPrefix(ctx, stderr, "[ERR]: ", ctx.Log, &wg)
				wg.Wait()
				return cmd.Wait()
			}

			ctx.Log("Running Stroppy Gen")
			workloadName := strings.ToLower(input.GetStroppyCliCall().GetWorkload().String())
			envsCmd := append(os.Environ(), envs.ToSlice(input.GetStroppyCliCall().GetStroppyEnv())...)
			genCmd := exec.Command(
				StroppyBinaryName,
				StroppyCommandGen,
				StroppyWorkdirFlag,
				input.GetStroppyCliCall().GetWorkdir(),
				StroppyPresetFlag,
				workloadName,
			)
			genCmd.Env = envsCmd
			err := runcmd(genCmd)
			if err != nil {
				return nil, fmt.Errorf("failed to run stroppy gen: %w", err)
			}
			ctx.Log("Running Stroppy Run")
			runCmd := exec.Command(
				StroppyBinaryName,
				StroppyCommandRun,
				fmt.Sprintf("%s.ts", workloadName),
				fmt.Sprintf("%s.sql", workloadName),
				TagFlag,
				fmt.Sprintf("%s=%s", K6RunIdTagName, input.GetCommon().GetRunId()),
				TagFlag,
				fmt.Sprintf("%s=%s", K6WorkloadTagName, workloadName),
			)
			runCmd.Dir = filepath.Dir(input.GetStroppyCliCall().GetWorkdir())
			runCmd.Env = envsCmd
			err = runcmd(runCmd)
			if err != nil {
				return nil, fmt.Errorf("failed to run stroppy: %w", err)
			}
			return &hatchet.EdgeTasks_RunStroppy_Output{}, nil
		},
	)
}
