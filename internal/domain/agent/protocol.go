package agent

// Command is sent from server to agent to execute a single primitive step.
//
// The agent knows NOTHING about databases, exporters, or any specific software.
// All domain logic (which packages, which versions, which config bodies, which
// scripts) lives on the server in internal/domain/run/. The agent only knows how
// to: run a shell script, write a file, start a tracked background process, and
// shut down. Everything else is composed on the server from these primitives.
type Command struct {
	ID     string `json:"id"`
	Action Action `json:"action"`
	// Label is a human/semantic tag (e.g. "install_postgres", "config_mysql")
	// used purely for log correlation in the UI. The agent does not branch on it.
	Label  string `json:"label,omitempty"`
	Config any    `json:"config"` // action-specific payload
}

// Action identifies which primitive the agent should run.
type Action string

const (
	// ActionRunCmd runs a bash script, streaming output as logs. See RunCmdConfig.
	ActionRunCmd Action = "run_cmd"
	// ActionWriteFile writes a file with given content/mode. See WriteFileConfig.
	ActionWriteFile Action = "write_file"
	// ActionStartDaemon launches a long-running tracked process killed on shutdown. See StartDaemonConfig.
	ActionStartDaemon Action = "start_daemon"
	// ActionShutdown kills all tracked background processes.
	ActionShutdown Action = "shutdown"
)

// RunCmdConfig is the payload for ActionRunCmd.
type RunCmdConfig struct {
	// Script is an opaque bash script. The agent runs it via `bash -c` and
	// streams every output line as a log. The script is built entirely on the
	// server — the agent never inspects or branches on its contents.
	Script string `json:"script"`
	// Exclusive serializes this command against other exclusive commands on the
	// same agent by holding the apt mutex. Used for apt/dpkg operations that
	// must not run concurrently (dpkg lock contention).
	Exclusive bool `json:"exclusive,omitempty"`
}

// WriteFileConfig is the payload for ActionWriteFile. Content is fully rendered
// on the server (config bodies come from internal/domain/dbconfig).
type WriteFileConfig struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    uint32 `json:"mode,omitempty"`   // file mode, default 0644
	Append  bool   `json:"append,omitempty"` // append instead of truncate
	// Owner, when set ("user" or "user:group"), chowns the file after writing.
	Owner string `json:"owner,omitempty"`
	// MkdirParents creates parent directories (default true).
	MkdirParents *bool `json:"mkdir_parents,omitempty"`
}

// StartDaemonConfig is the payload for ActionStartDaemon.
type StartDaemonConfig struct {
	Name string            `json:"name"` // pool key, used for logs + shutdown
	Bin  string            `json:"bin"`  // absolute path to the binary
	Args []string          `json:"args,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

// Report is sent from agent back to server.
type Report struct {
	CommandID string       `json:"command_id"`
	Status    ReportStatus `json:"status"`
	Error     string       `json:"error,omitempty"`
	Output    string       `json:"output,omitempty"`
}

// ReportStatus indicates the outcome of a command execution.
type ReportStatus string

const (
	ReportRunning   ReportStatus = "running"
	ReportCompleted ReportStatus = "completed"
	ReportFailed    ReportStatus = "failed"
)

// LogLine is a streamed log entry from agent to server.
type LogLine struct {
	CommandID string `json:"command_id"`
	MachineID string `json:"machine_id,omitempty"`
	Action    string `json:"action,omitempty"` // the command Label, e.g. "install_postgres"
	Line      string `json:"line"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
}
