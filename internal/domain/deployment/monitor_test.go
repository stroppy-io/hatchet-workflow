package deployment

import (
	"strings"
	"testing"

	topologyindex "github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

func TestMonitorStepsUseOnlyServerAddressForCollectors(t *testing.T) {
	spec := fakeSpec()
	spec.Labels = map[string]string{
		LabelServerAddr: "https://control.example",
		LabelRunID:      "run-1",
	}
	idx, err := topologyindex.NewIndex(spec)
	if err != nil {
		t.Fatalf("topology index: %v", err)
	}
	state := fakeInfrastructureState(spec)

	steps := MonitorSteps(RenderContext{
		Topology:   idx,
		Component:  spec.GetComponents()[0],
		Node:       spec.GetNodes()[0],
		Machine:    state.GetMachines()[0],
		AgentToken: "agent-token",
	})
	if len(steps) == 0 {
		t.Fatal("monitor steps are missing")
	}

	install := callCmdText(t, stepByID(steps, "300_install_collectors"))
	for _, want := range []string{
		"https://control.example/api/binaries/node_exporter/",
		"https://control.example/api/binaries/vmagent/",
		"https://control.example/api/binaries/vector/",
	} {
		if !strings.Contains(install, want) {
			t.Fatalf("collector install script missing %q:\n%s", want, install)
		}
	}
	assertNotContainsAny(t, install, "github.com", "http://vmauth", "https://vmauth", "http://victoria", "https://victoria")

	vmagent := callCmdText(t, stepByID(steps, "330_start_vmagent"))
	if want := "https://control.example/insert/0/prometheus/api/v1/write"; !strings.Contains(vmagent, want) {
		t.Fatalf("vmagent script missing %q:\n%s", want, vmagent)
	}
	if want := "-remoteWrite.bearerToken='agent-token'"; !strings.Contains(vmagent, want) {
		t.Fatalf("vmagent script missing %q:\n%s", want, vmagent)
	}
	assertNotContainsAny(t, vmagent, "http://vmauth", "https://vmauth", "http://victoria", "https://victoria")

	vector := fileText(t, stepByID(steps, "340_write_vector_config"))
	for _, want := range []string{
		`uri: "https://control.example/insert/jsonline"`,
		`Authorization: "Bearer agent-token"`,
		`AccountID: "0"`,
	} {
		if !strings.Contains(vector, want) {
			t.Fatalf("vector config missing %q:\n%s", want, vector)
		}
	}
	assertNotContainsAny(t, vector, "http://vmauth", "https://vmauth", "http://victoria", "https://victoria")
}

func stepByID(steps []*deploymentpb.AgentStep, id string) *deploymentpb.AgentStep {
	for _, step := range steps {
		if step.GetId() == id {
			return step
		}
	}
	return nil
}

func callCmdText(t *testing.T, step *deploymentpb.AgentStep) string {
	t.Helper()
	if step == nil || step.GetCallCmd() == nil || step.GetCallCmd().GetSpec().GetScript() == nil {
		t.Fatalf("step does not contain a shell command: %#v", step)
	}
	return step.GetCallCmd().GetSpec().GetScript().GetText()
}

func fileText(t *testing.T, step *deploymentpb.AgentStep) string {
	t.Helper()
	if step == nil || step.GetWriteFile() == nil {
		t.Fatalf("step does not contain a file write: %#v", step)
	}
	return step.GetWriteFile().GetText()
}

func assertNotContainsAny(t *testing.T, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Fatalf("text must not contain %q:\n%s", needle, text)
		}
	}
}
