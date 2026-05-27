package trace

import "time"

type ToolCallTrace struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
	Result   string         `json:"result"`
	IsError  bool           `json:"is_error"`
}

type Trajectory struct {
	Timestamp time.Time       `json:"timestamp"`
	Code      string          `json:"code"`
	ToolCalls []ToolCallTrace `json:"tool_calls"`
	Output    string          `json:"output"`
	Error     string          `json:"error,omitempty"`
}
