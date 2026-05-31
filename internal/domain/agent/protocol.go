package agent

// Command is one primitive agent operation the recipe produces. The run recipe
// builds sequences of these; the Temporal RunWorkflow translates each into an
// AgentCommandService activity (run_cmd -> CallCmd, write_file -> WriteFile,
// start_daemon -> systemd-run CallCmd) and dispatches it to the target's agent
// worker. This is a pure data model — no transport.
type Command struct {
	ID     string `json:"id"`
	Action Action `json:"action"`
	// Label is a human/semantic tag (e.g. "install_postgres") used for log
	// correlation / stage naming in the workflow.
	Label  string `json:"label,omitempty"`
	Config any    `json:"config"`
}

// Action identifies which primitive a Command carries.
type Action string

const (
	// ActionRunCmd runs a bash script. See RunCmdConfig.
	ActionRunCmd Action = "run_cmd"
	// ActionWriteFile writes a file. See WriteFileConfig.
	ActionWriteFile Action = "write_file"
	// ActionStartDaemon launches a long-running tracked process. See StartDaemonConfig.
	ActionStartDaemon Action = "start_daemon"
)

// RunCmdConfig is the payload for ActionRunCmd.
type RunCmdConfig struct {
	// Script is an opaque bash script.
	Script string `json:"script"`
	// Exclusive hints that the command must serialize against other exclusive
	// commands on the same machine (apt/dpkg lock). The workflow runs a target's
	// commands sequentially, so this is informational now.
	Exclusive bool `json:"exclusive,omitempty"`
}

// WriteFileConfig is the payload for ActionWriteFile.
type WriteFileConfig struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	Mode         uint32 `json:"mode,omitempty"`
	Append       bool   `json:"append,omitempty"`
	Owner        string `json:"owner,omitempty"`
	MkdirParents *bool  `json:"mkdir_parents,omitempty"`
}

// StartDaemonConfig is the payload for ActionStartDaemon. Translated to a
// systemd-run command by the workflow so the process survives the activity.
type StartDaemonConfig struct {
	Name string            `json:"name"`
	Bin  string            `json:"bin"`
	Args []string          `json:"args,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}
