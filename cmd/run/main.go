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
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/stroppy"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
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
	var test hatchet.Workflows_StroppyTestSuite_Input
	err = protoyaml.Unmarshal(fileContent, &test)
	if err != nil {
		log.Fatalf("Failed to unmarshal file: %v", err)
	}
	err = test.Validate()
	if err != nil {
		log.Fatalf("Failed to validate %s input: %v", stroppy.TestSuiteWorkflowName, err)
	}
	token := os.Getenv("HATCHET_CLIENT_TOKEN")
	if token == "" {
		log.Fatalf("HATCHET_CLIENT_TOKEN is not set")
	}
	logger.NewFromEnv()
	c, err := hatchetLib.NewClient(v0Client.WithLogger(logger.Zerolog()))
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}
	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()
	result, err := c.Run(
		interruptCtx,
		stroppy.TestSuiteWorkflowName,
		&test,
	)
	if err != nil {
		log.Fatalf("Failed to run workflow: %v", err)
	}
	var output *hatchet.Workflows_StroppyTestSuite_Output
	if err := result.TaskOutput(stroppy.TestSuiteTaskName).Into(output); err != nil {
		log.Fatalf("Failed to get %s output: %v", stroppy.TestSuiteTaskName, err)
	}
	resultYaml, err := protoyaml.MarshalPretty(output)
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}
	fmt.Println(string(resultYaml))
}
