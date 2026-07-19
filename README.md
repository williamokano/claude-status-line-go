# claude-status-line-go

Go CLI tool that reads Claude Code JSON from stdin and prints a formatted status line.

## Output Format

```
🧠 O4.7·1M │ 📁 payments-api │ 🌿 feature/calendar ●3
🟡5h ████████░░ 83% ↺22m │ CTX ██████░░░░ 68% │ I420k O77k ⚡2.3M │ 7d 74% │ $7.92
```

## Installation

### Using go install

```bash
go install github.com/williamokano/claude-status-line-go/cmd/claude-status-line-go@latest
```

### From Source

```bash
git clone https://github.com/williamokano/claude-status-line-go.git
cd claude-status-line-go
make build
```

## Usage

### As Claude Code Status Line

Add to your Claude Code settings (`~/.claude/settings.json` or project `.claude/settings.json`):

```json
{
  "statusLine": {
    "type": "command",
    "command": "claude-status-line-go"
  }
}
```

### Standalone

```bash
echo '{"model":...}' | claude-status-line-go
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `-h, --help` | Print usage information |
| `-v, --version` | Print version |
| `--no-color` | Disable ANSI color output |
| `--completion bash\|zsh\|fish` | Print shell completion script |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CSL_SHOW_COST` | `true` | Show session cost |
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
| `CSL_FORMAT` | — | Custom output format template |
| `NO_COLOR` | — | Set to any value to disable ANSI colors |

## Configuration File

Create `~/.config/claude-status-line-go/claude-status-line.yaml`:

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

## Custom Output Format

Use the `CSL_FORMAT` env var or `format` config field to customize the output.
Available placeholders:

| Placeholder | Description |
|-------------|-------------|
| `{model}` | Short model name (O4.7, S5, H) |
| `{ctx_size}` | Context window size (200k, 1M) |
| `{project}` | Project folder name |
| `{branch}` | Git branch name |
| `{dirty}` | Dirty file count (●3) |
| `{limit_bar}` | Rate limit progress bar |
| `{limit_pct}` | Rate limit percentage |
| `{limit_color}` | Rate limit color ANSI code |
| `{limit_reset}` | Time until rate limit reset |
| `{ctx_bar}` | Context progress bar |
| `{ctx_pct}` | Context percentage |
| `{ctx_color}` | Context color ANSI code |
| `{tokens_in}` | Input tokens (42k, 2.3M) |
| `{tokens_out}` | Output tokens |
| `{tokens_cache}` | Cache tokens |
| `{cost}` | Session cost |
| `{weekly_pct}` | Weekly usage percentage |
| `{reset}`, `{dim}`, `{bold}` | ANSI format codes |
| `{red}`, `{green}`, `{yellow}` | ANSI color codes |
| `{blue}`, `{magenta}`, `{cyan}`, `{white}` | ANSI color codes |

Example:

```yaml
format: "{cyan}🧠 {model}·{ctx_size}{reset} {dim}│ 📁 {project}{reset} {dim}│ 🌿 {branch}{reset}{dirty}"
```

## Shell Completions

```bash
eval "$(claude-status-line-go --completion bash)"  # Bash
eval "$(claude-status-line-go --completion zsh)"   # Zsh
claude-status-line-go --completion fish | source    # Fish
```

## Development

```bash
make build   # Build the binary
make test    # Run tests
make lint    # Lint code
make clean   # Clean artifacts
```
