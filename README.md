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

All options can be configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SHOW_COST` | `true` | Show cost in status line |
| `SHOW_WEEKLY` | `true` | Show weekly usage |
| `SHOW_TOKENS` | `true` | Show token counts |
| `SHOW_GIT` | `true` | Show git branch |
| `SHOW_GIT_DIRTY` | `true` | Show dirty file count |
| `BAR_SIZE` | `10` | Progress bar width |
| `LIMIT_WARN` | `60` | Rate limit warning threshold (%) |
| `LIMIT_CRIT` | `85` | Rate limit critical threshold (%) |
| `CTX_WARN` | `60` | Context warning threshold (%) |
| `CTX_CRIT` | `85` | Context critical threshold (%) |
| `WEEKLY_SHOW_AT` | `60` | Show weekly when >= this % |

Example:
```bash
export SHOW_TOKENS=false
export BAR_SIZE=15
export LIMIT_WARN=50
claude-status-line-go
```

## Usage

Pipe Claude Code JSON output to the tool:

```bash
claude-code --output-format json | claude-status-line-go
```