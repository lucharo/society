package models

type TransportConfig struct {
	Type   string            `json:"type" yaml:"type"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

type AgentCard struct {
	Name               string           `json:"name"`
	Description        string           `json:"description,omitempty"`
	URL                string           `json:"url"`
	Version            string           `json:"version,omitempty"`
	Provider           *Provider        `json:"provider,omitempty"`
	Skills             []Skill          `json:"skills,omitempty"`
	Capabilities       *Capabilities    `json:"capabilities,omitempty"`
	DefaultInputModes  []string         `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string         `json:"defaultOutputModes,omitempty"`
	Authentication     *Authentication  `json:"authentication,omitempty"`
	DocumentationURL   string           `json:"documentationUrl,omitempty"`
	Transport          *TransportConfig `json:"transport,omitempty"`
}

type Provider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type Authentication struct {
	Schemes     []string `json:"schemes"`
	Credentials string   `json:"credentials,omitempty"`
}

type Capabilities struct {
	Streaming              bool `json:"streaming,omitempty"`
	PushNotifications      bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

type Task struct {
	ID        string     `json:"id"`
	Status    TaskStatus `json:"status"`
	Messages  []Message  `json:"messages,omitempty"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

type TaskStatus struct {
	State   TaskState `json:"state"`
	Message string    `json:"message,omitempty"`
}

type TaskState string

const (
	TaskStateSubmitted TaskState = "submitted"
	TaskStateWorking   TaskState = "working"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateCancelled TaskState = "cancelled"
)

type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

type Artifact struct {
	Name  string `json:"name,omitempty"`
	Parts []Part `json:"parts"`
}

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type SendTaskParams struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
}

type BackendConfig struct {
	Command          string   `yaml:"command"`
	Args             []string `yaml:"args,omitempty"`
	SessionFlag      string   `yaml:"session_flag,omitempty"`
	ResumeFlag       string   `yaml:"resume_flag,omitempty"`
	SystemPromptFlag string   `yaml:"system_prompt_flag,omitempty"`
	Env              []string `yaml:"env,omitempty"`
}

// backendDefaults maps known backend commands to their default system prompt flags.
var backendDefaults = map[string]string{
	"claude": "--system-prompt",
	"happy":  "--system-prompt", // happy wraps claude
	"goose":  "--system",
}

// ApplyDefaults fills in unset fields with known defaults for the backend command.
func (b *BackendConfig) ApplyDefaults() {
	if b.SystemPromptFlag == "" {
		if flag, ok := backendDefaults[b.Command]; ok {
			b.SystemPromptFlag = flag
		}
	}
}

type AgentConfig struct {
	Name         string         `yaml:"name"`
	Description  string         `yaml:"description,omitempty"`
	Port         int            `yaml:"port,omitempty"`
	Handler      string         `yaml:"handler"`
	Backend      *BackendConfig `yaml:"backend,omitempty"`
	Skills       []Skill        `yaml:"skills,omitempty"`
	SystemPrompt string         `yaml:"system_prompt,omitempty"`
}

type RegistryFile struct {
	Agents []AgentCard `json:"agents"`
}
