# claude-status-line-go

Go CLI tool that reads Claude Code JSON from stdin and prints a formatted status line.

## Output Format

```
🧠 Opus 4.8·1M │ 📁 payments-api │ 🌿 feature/calendar ●3
🟡5h ████████░░ 83% ↺ 22m │ CTX ██████░░░░ 68% │ I420k O77k ⚡2.3M │ 📅7d ███████░░░ 74% ↺ 2d4h │ $7.92
```

Both rate-limit windows render the same way — icon, progress bar, percentage and
time until reset — and share the `limit_warn` / `limit_crit` color thresholds.
The weekly window only appears once it reaches `weekly_show_at` (60% by default),
and its countdown switches to `2d4h` form when the reset is more than a day out.

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

#### Automatic (recommended)

```bash
claude-status-line-go install
```

This registers the binary as the `statusLine` command in `~/.claude/settings.json`,
preserving any other settings already in that file. Use `--project` to register it
in the current project's `.claude/settings.json` instead:

```bash
claude-status-line-go install --project
```

#### Manual

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

## CLI Commands & Flags

| Command | Description |
|---------|-------------|
| `install` | Register this binary as the Claude Code status line |

| Flag | Description |
|------|-------------|
| `-h, --help` | Print usage information |
| `-v, --version` | Print version |
| `--no-color` | Disable ANSI color output |
| `--completion bash\|zsh\|fish` | Print shell completion script |
| `--project` | With `install`: register in `./.claude/settings.json` instead of the global settings |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CSL_SHOW_COST` | `true` | Show session cost |
| `CSL_SHOW_WEEKLY` | `true` | Show weekly usage |
| `CSL_SHOW_TOKENS` | `true` | Show token counts |
| `CSL_SHOW_GIT` | `true` | Show git branch |
| `CSL_SHOW_GIT_DIRTY` | `true` | Show dirty file count |
| `CSL_BAR_SIZE` | `10` | Progress bar width |
| `CSL_LIMIT_WARN` | `60` | Rate limit warning threshold (%), 5-hour and weekly |
| `CSL_LIMIT_CRIT` | `85` | Rate limit critical threshold (%), 5-hour and weekly |
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
| `{model}` | Model name as reported by Claude Code (e.g. Opus 4.8, Sonnet 5) |
| `{ctx_size}` | Context window size (200k, 1M) |
| `{project}` | Project folder name |
| `{branch}` | Git branch name |
| `{dirty}` | Dirty file count (●3) |
| `{limit_bar}` | 5-hour rate limit progress bar |
| `{limit_pct}` | 5-hour rate limit percentage |
| `{limit_color}` | 5-hour rate limit color ANSI code |
| `{limit_reset}` | Time until the 5-hour limit resets |
| `{ctx_bar}` | Context progress bar |
| `{ctx_pct}` | Context percentage |
| `{ctx_color}` | Context color ANSI code |
| `{tokens_in}` | Input tokens (42k, 2.3M) |
| `{tokens_out}` | Output tokens |
| `{tokens_cache}` | Cache tokens |
| `{cost}` | Session cost |
| `{weekly_bar}` | Weekly usage progress bar |
| `{weekly_pct}` | Weekly usage percentage |
| `{weekly_color}` | Weekly usage color ANSI code |
| `{weekly_reset}` | Time until the weekly limit resets |
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
