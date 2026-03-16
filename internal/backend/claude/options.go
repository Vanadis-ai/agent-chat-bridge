package claude

import (
	"os"
	"strings"

	claudecode "github.com/severity1/claude-agent-sdk-go"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func buildOptions(b *ClaudeBackend, req core.BackendRequest) []claudecode.Option {
	opts := []claudecode.Option{
		claudecode.WithPartialStreaming(),
	}

	if b.binary != "" {
		opts = append(opts, claudecode.WithCLIPath(b.binary))
	}
	if req.WorkingDir != "" {
		opts = append(opts, claudecode.WithCwd(req.WorkingDir))
	}
	if req.Model != "" {
		opts = append(opts, claudecode.WithModel(req.Model))
	}
	if req.SystemPrompt != "" {
		opts = append(opts, claudecode.WithAppendSystemPrompt(req.SystemPrompt))
	}
	if req.SessionID != "" {
		opts = append(opts, claudecode.WithResume(req.SessionID))
	}
	if req.Agent != nil {
		opts = append(opts, buildAgentOptions(req.Agent)...)
	}

	pm := mapPermissionMode(req.PermissionMode)
	opts = append(opts, claudecode.WithPermissionMode(pm))
	opts = append(opts, claudecode.WithEnv(b.env))
	opts = append(opts, claudecode.WithStderrCallback(func(line string) {}))

	return opts
}

func buildAgentOptions(agent *core.AgentDef) []claudecode.Option {
	var opts []claudecode.Option

	def := claudecode.AgentDefinition{
		Description: agent.Description,
		Prompt:      agent.Prompt,
	}
	if len(agent.Tools) > 0 {
		def.Tools = agent.Tools
	}
	opts = append(opts, claudecode.WithAgent(agent.Name, def))

	name := agent.Name
	extraArgs := map[string]*string{"agent": &name}

	if agent.Tools != nil && len(agent.Tools) == 0 {
		noTools := ""
		extraArgs["tools"] = &noTools
	}

	opts = append(opts, claudecode.WithExtraArgs(extraArgs))
	return opts
}

func mapPermissionMode(mode string) claudecode.PermissionMode {
	switch mode {
	case "bypassPermissions":
		return claudecode.PermissionModeBypassPermissions
	case "plan":
		return claudecode.PermissionModePlan
	case "acceptEdits":
		return claudecode.PermissionModeAcceptEdits
	default:
		return claudecode.PermissionModeDefault
	}
}

func filteredEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if shouldFilterEnv(k) {
			continue
		}
		env[k] = v
	}
	return env
}

var filteredPrefixes = []string{"CLAUDE_CODE_", "CLAUDE_MANAGER_", "OTEL_"}

func shouldFilterEnv(key string) bool {
	if key == "CLAUDECODE" {
		return true
	}
	for _, p := range filteredPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
