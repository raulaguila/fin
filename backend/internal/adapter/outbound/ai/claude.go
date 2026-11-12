package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/raulaguila/fin/config"
)

// Claude runs `claude -p` once per question. It has no built-in tools, no MCP
// server but the ones passed later, no session on disk and none of the
// workstation's settings, hooks or project CLAUDE.md.
type Claude struct {
	cfg config.AIConfig
}

func (c Claude) Ask(ctx context.Context, system, prompt string) (string, error) {
	args := []string{
		"-p", "--output-format", "json",
		"--tools", "", "--strict-mcp-config",
		"--no-session-persistence", "--setting-sources", "",
	}
	if system != "" {
		args = append(args, "--system-prompt", system)
	}
	if c.cfg.Model != "" {
		args = append(args, "--model", c.cfg.Model)
	}
	cmd := exec.CommandContext(ctx, c.cfg.ClaudeBin, args...)
	cmd.Dir = os.TempDir() // away from any project CLAUDE.md
	cmd.Env = claudeEnv(c.cfg.ClaudeToken)
	cmd.Stdin = strings.NewReader(prompt) // financial data stays out of argv and ps
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, runErr := cmd.Output()
	if len(out) > 0 {
		answer, err := parseClaude(out)
		if err == nil || runErr == nil {
			return answer, err
		}
	}
	if runErr != nil {
		return "", fmt.Errorf("claude: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return "", errors.New("claude: empty output")
}

// claudeEnv passes only what the CLI needs: database and SMTP secrets of the
// backend must not reach the child process.
func claudeEnv(token string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "PATH", "HOME", "USER", "LANG", "TZ", "TMPDIR", "XDG_CONFIG_HOME", "CLAUDE_CONFIG_DIR":
			env = append(env, kv)
		}
	}
	if token != "" {
		env = append(env, "CLAUDE_CODE_OAUTH_TOKEN="+token)
	}
	return env
}

type claudeResult struct {
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

// parseClaude reads the --output-format json result. A failed call can still
// say "success" in subtype, so is_error decides.
func parseClaude(out []byte) (string, error) {
	var r claudeResult
	if err := json.Unmarshal(out, &r); err != nil {
		return "", fmt.Errorf("claude: unreadable output: %w", err)
	}
	if r.IsError || r.Subtype != "success" {
		return "", fmt.Errorf("claude: %s", r.Result)
	}
	return r.Result, nil
}
