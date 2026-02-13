package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/core/protoyaml"
	workflowTest "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/test"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

func main() {
	filePath := flag.String("file", "", "path to file")
	flag.Parse()
	if *filePath == "" {
		log.Fatal("flag -file is required")
	}
	fileContent, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	var input workflows.Workflows_StroppyTestSuite_Input
	err = protoyaml.Unmarshal(fileContent, &input)
	if err != nil {
		log.Fatalf("Failed to unmarshal file: %v", err)
	}
	err = input.Validate()
	if err != nil {
		log.Fatalf("Failed to validate %s input: %v", workflowTest.SuiteWorkflowName, err)
	}
	logger.NewFromEnv()
	opts := []v0Client.ClientOpt{
		v0Client.WithLogger(logger.Zerolog()),
		v0Client.WithToken(input.GetSettings().GetHatchetConnection().GetToken()),
	}
	opts = append(opts, v0Client.WithHostPort(
		input.GetSettings().GetHatchetConnection().GetHost(),
		int(input.GetSettings().GetHatchetConnection().GetPort()),
	))
	c, err := hatchetLib.NewClient(opts...)
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}
	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()
	result, err := c.Run(
		interruptCtx,
		workflowTest.SuiteWorkflowName,
		&input,
	)
	if err != nil {
		log.Fatalf("Failed to run workflow: %v", err)
	}
	var output *workflows.Workflows_StroppyTestSuite_Output
	if err := result.TaskOutput(workflowTest.SuiteTaskName).Into(output); err != nil {
		log.Fatalf("Failed to get %s output: %v", workflowTest.SuiteTaskName, err)
	}
	resultYaml, err := protoyaml.MarshalPretty(output)
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}
	fmt.Println(string(resultYaml))
}
