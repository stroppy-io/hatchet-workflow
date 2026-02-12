package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/cmdutils"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/core/protoyaml"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/test"
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
		log.Fatalf("Failed to validate %s input: %v", test.TestSuiteWorkflowName, err)
	}
	logger.NewFromEnv()
	var host string
	var port string
	var urlServer string
	switch test.GetHatchetServer().GetServer().(type) {
	case *hatchet.HatchetServer_HostPort_:
		host = test.GetHatchetServer().GetHostPort().GetHost()
		port = test.GetHatchetServer().GetHostPort().GetPort()
	case *hatchet.HatchetServer_Url:
		urlServer = test.GetHatchetServer().GetUrl()
	}
	opts := []v0Client.ClientOpt{
		v0Client.WithLogger(logger.Zerolog()),
		v0Client.WithToken(test.GetHatchetServer().GetToken()),
	}
	if host != "" && port != "" {
		opts = append(opts, v0Client.WithHostPort(host, lo.Must(strconv.Atoi(port))))
	}
	if urlServer != "" {
		urls, err := url.Parse(urlServer)
		if err != nil {
			log.Fatalf("Failed to parse Hatchet server URL: %v", err)
		}
		opts = append(opts, v0Client.WithHostPort(urls.Hostname(), lo.Must(strconv.Atoi(urls.Port()))))
	}
	c, err := hatchetLib.NewClient(opts...)
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}
	interruptCtx, cancel := cmdutils.NewInterruptContext()
	defer cancel()
	result, err := c.Run(
		interruptCtx,
		test.TestSuiteWorkflowName,
		&test,
	)
	if err != nil {
		log.Fatalf("Failed to run workflow: %v", err)
	}
	var output *hatchet.Workflows_StroppyTestSuite_Output
	if err := result.TaskOutput(test.TestSuiteTaskName).Into(output); err != nil {
		log.Fatalf("Failed to get %s output: %v", test.TestSuiteTaskName, err)
	}
	resultYaml, err := protoyaml.MarshalPretty(output)
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}
	fmt.Println(string(resultYaml))
}
