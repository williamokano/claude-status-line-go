# Installation

## Using go install

```bash
# Latest version
go install github.com/williamokano/claude-status-line-go/cmd/claude-status-line-go@latest

# Specific version
go install github.com/williamokano/claude-status-line-go/cmd/claude-status-line-go@v1.0.0
```

## From Source

```bash
git clone https://github.com/williamokano/claude-status-line-go.git
cd claude-status-line-go
go build -o claude-status-line-go ./cmd/claude-status-line-go
```

## Configuration

All options can be configured via environment variables (prefix `CSL_`):

| Variable | Default | Description |
|----------|---------|-------------|
| `CSL_SHOW_COST` | `true` | Show cost in status line |
| `CSL_SHOW_WEEKLY` | `true` | Show weekly usage |
| `CSL_SHOW_TOKENS` | `true` | Show token counts |
| `CSL_SHOW_GIT` | `true` | Show git branch |
| `CSL_SHOW_GIT_DIRTY` | `true` | Show dirty file count |
| `CSL_BAR_SIZE` | `10` | Progress bar width |
| `CSL_LIMIT_WARN` | `60` | Rate limit warning threshold (%) |
| `CSL_LIMIT_CRIT` | `85` | Rate limit critical threshold (%) |
| `CSL_CTX_WARN` | `60` | Context warning threshold (%) |
| `CSL_CTX_CRIT` | `85` | Context critical threshold (%) |
| `CSL_WEEKLY_SHOW_AT` | `60` | Show weekly when >= this % |

Example:
```bash
export CSL_SHOW_TOKENS=false
export CSL_BAR_SIZE=15
export CSL_LIMIT_WARN=50
claude-status-line-go
```

### Config File

The tool also reads `claude-status-line.yaml` from:
- `~/.config/claude-status-line-go/claude-status-line.yaml` (Linux/macOS)
- `%APPDATA%\claude-status-line-go\claude-status-line.yaml` (Windows)
- Current directory (`.`)

Example config file:
```yaml
show_cost: true
show_weekly: true
show_tokens: true
show_git: true
show_git_dirty: true
bar_size: 10
limit_warn: 60
limit_crit: 85
ctx_warn: 60
ctx_crit: 85
weekly_show_at: 60
```

## Usage

### As Claude Code Status Line

Add to your Claude Code settings (`~/.claude/settings.json` or project `.claude/settings.json`):

```json
{
  "statusLine": {
    "command": "claude-status-line-go",
    "args": []
  }
}
```

With environment variables:
```json
{
  "statusLine": {
    "command": "claude-status-line-go",
    "args": [],
    "env": {
      "CSL_BAR_SIZE": "15",
      "CSL_SHOW_TOKENS": "false"
    }
  }
}
```

### Standalone (for testing)

Pipe Claude Code JSON output to the tool:

```bash
claude-code --output-format json | claude-status-line-go
```

## Output Format

```
🧠 O4.7·1M │ 📁 payments-api │ 🌿 feature/calendar ●3
🟡5h ████████░░ 83% ↺22m │ CTX ██████░░░░ 68% │ I420k O77k ⚡2.3M │ 7d 74% │ $7.92
```

- **Top line**: Model, project folder, git branch (+ dirty count)
- **Bottom line**: 5h rate limit, context usage, tokens (in/out/cache), weekly usage, cost