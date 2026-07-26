# claude-status-line-go

Go CLI tool that reads Claude Code JSON from stdin and prints a formatted status line.

📖 **[okano.dev/claude-status-line-go](https://okano.dev/claude-status-line-go/)**

## Output Format

```
🧠 Opus 5·1M │ 📁 claude-status-line-go │ 🌿 feature/cache-stats ●3
🟡5h ████████░░ 83% ↺ 22m │ CTX ██████░░░░ 68% │ Σ115k ↓277 ⚡99% │ 📅7d ███████░░░ 74% ↺ 2d4h │ $7.92
```

The token segment reads: `Σ` total prompt size, `↓` tokens generated, `⚡` share of
the prompt served from the cache. `Σ` is the sum of all three input fields Claude
Code reports — `input_tokens` counts only the part that *missed* the cache, so on a
warm conversation it can be a couple of tokens while the real prompt is 100k+.
`⚡` counts cache **reads** only; a cold turn that writes 48k to the cache reports
0%, because those tokens were billed at a premium rather than served cheaply.

Both rate-limit windows render the same way — icon, progress bar, percentage and
time until reset — and share the `limit_warn` / `limit_crit` color thresholds.
The weekly window only appears once it reaches `weekly_show_at` (60% by default),
and its countdown switches to `2d4h` form when the reset is more than a day out.

## Installation

### macOS and Linux

```bash
curl -fsSL https://okano.dev/claude-status-line-go/install.sh | sh
```

Works out your OS and architecture, resolves the newest release, checks the
SHA-256 against `checksums.txt`, and installs to `/usr/local/bin` — or
`~/.local/bin` if that isn't yours to write to. It never calls `sudo`.
[Read it first](https://okano.dev/claude-status-line-go/install.sh) if you'd rather.

Pin a version, or pick the destination:

```bash
curl -fsSL https://okano.dev/claude-status-line-go/install.sh | VERSION=1.4.0 INSTALL_DIR=~/bin sh
```

<details>
<summary>Prefer not to pipe into a shell?</summary>

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
  https://github.com/williamokano/claude-status-line-go/releases/latest | sed 's#.*/v##')

FILE="claude-status-line-go_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/williamokano/claude-status-line-go/releases/download/v${VERSION}"

curl -fsSLO "${BASE}/${FILE}"
curl -fsSLO "${BASE}/checksums.txt"
grep " ${FILE}$" checksums.txt | sha256sum -c -   # macOS: shasum -a 256 -c -

tar -xzf "$FILE" claude-status-line-go
sudo install -m 755 claude-status-line-go /usr/local/bin/
```

</details>

### Windows

The Windows archive holds `claude-status-line-go.exe`. `tar` ships with Windows
10 build 17063 and later, so nothing else is needed.

```powershell
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # or Invoke-WebRequest crawls

$Repo    = 'williamokano/claude-status-line-go'
$Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name.TrimStart('v')
$Arch    = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }

$File = "claude-status-line-go_${Version}_windows_${Arch}.tar.gz"
$Base = "https://github.com/$Repo/releases/download/v$Version"

Invoke-WebRequest "$Base/$File" -OutFile $File
Invoke-WebRequest "$Base/checksums.txt" -OutFile checksums.txt

$want = (Select-String checksums.txt -Pattern ([regex]::Escape($File))).Line.Split(' ')[0]
$got  = (Get-FileHash $File -Algorithm SHA256).Hash.ToLower()
if ($want -ne $got) { throw 'checksum mismatch' }

$Dest = "$env:LOCALAPPDATA\Programs\claude-status-line-go"
New-Item -ItemType Directory -Force -Path $Dest | Out-Null
tar -xzf $File -C $Dest claude-status-line-go.exe
```

Add `$Dest` to your `PATH` to finish. There's no Chocolatey or WinGet package —
both want a published, moderated manifest rather than a link to a release.

### Using go install

```bash
go install github.com/williamokano/claude-status-line-go/cmd/claude-status-line-go@latest
```

Because `@latest` resolves through the Go module proxy, a freshly pushed tag can
take a few minutes to become visible. Check what you actually got with
`claude-status-line-go --version`.

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

Precedence is defaults, then this file, then the environment — so a `CSL_*`
variable always wins over the file.

## Plugins

Plugins add your own segments. A plugin reports **data** — a value, an optional
maximum, maybe a label — and this tool draws it. That way your segment gets the
same bar glyphs, the same `bar_size` and the same `NO_COLOR` handling as the
built-in ones, instead of every plugin reinventing bars and colours slightly
differently.

```yaml
plugins:
  - name: issues
    command: >-
      printf '{"value":%s,"max":100}'
      "$(gh issue list --json number --jq 'length')"
    interval: 60s
    icon: "🎯"
    bar: true
    thresholds:
      - { at: 0,  color: green }
      - { at: 40, color: yellow }
      - { at: 70, color: red }
```

```
… │ Σ115k ↓277 ⚡99% │ 🎯 ███░░░░░░░ 31/100 │ $7.92
```

### Sources

| Key | Description |
|-----|-------------|
| `file` | A path read at render time. Costs nothing — ideal when something already writes the data, such as a Claude Code hook |
| `command` | A shell command. **Never runs on the render path** — see below |

A command's result is cached and refreshed out of band. A render reads the cache
and, if it's older than `interval`, hands the work to a detached process and
draws the stale value immediately. A 350 ms `gh` call therefore costs the status
line nothing; it only means the number can be up to `interval` old.

The command gets the Claude Code JSON on stdin, runs in the project directory,
and has `CSL_PROJECT_DIR` and `CSL_PLUGIN_NAME` set. Its cache is keyed per
project, so task counts and PR state don't leak between repos. If it fails, the
last good value stays on screen and the error goes to stderr.

### Output contract

Emit a JSON object on stdout:

```json
{"value": 12, "max": 33, "label": "aplicar"}
```

| Field | Description |
|-------|-------------|
| `value` | The number. With `max`, drives the percentage, bar and thresholds |
| `max` | Optional. Without it the segment is just text — no bar, no colour |
| `text` | Overrides the auto-formatted `value/max` |
| `label` | A short name, available to `display` |
| `state` | `ok`, `warn` or `crit` — skips threshold resolution |
| `hide` | `true` to drop the segment this render |
| `raw` | Pre-rendered output, ANSI and all. Opts out of everything above |

Bare text works too and is taken as `text`, so `echo "12 left"` is a valid
plugin. **Any key not listed above** is reachable as `{plugin.<name>.<key>}` —
that's how one plugin reports several pieces of information.

### Options

| Key | Default | Description |
|-----|---------|-------------|
| `name` | — | Required. Names the placeholders |
| `icon` | — | Prefix glyph |
| `bar` | `false` | Draw a progress bar, using the global `bar_size` |
| `display` | — | Layout template, e.g. `"{icon} {label} {bar} {value}/{max}"` |
| `thresholds` | — | Colour stops, see below |
| `interval` | `60s` | How long a `command` result stays fresh |
| `timeout` | `5s` | Caps one run of a `command` |

`thresholds` is a list, where `at` is a lower bound in percent and each colour
applies upward. There's no "invert" flag — for a ramp where more is better, list
the colours the other way round:

```yaml
thresholds:
  - { at: 0,  color: red }
  - { at: 31, color: yellow }
  - { at: 61, color: green }
```

### Placeholders

Plugin segments append to the default layout in declaration order, just before
cost. To place them yourself, use `format`:

| Placeholder | Description |
|-------------|-------------|
| `{plugin.<name>}` | The finished segment |
| `{plugin.<name>.value}` | Reported value |
| `{plugin.<name>.max}` | Reported maximum |
| `{plugin.<name>.pct}` | Percentage |
| `{plugin.<name>.bar}` | Progress bar |
| `{plugin.<name>.text}` | Text |
| `{plugin.<name>.label}` | Label |
| `{plugin.<name>.color}` | Resolved colour name |
| `{plugin.<name>.<key>}` | Any extra key the plugin reported |

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
| `{tokens_total}` | Total prompt tokens — uncached + cache creation + cache read |
| `{tokens_out}` | Output tokens |
| `{cache_hit_pct}` | Share of the prompt served from cache (cache reads / total) |
| `{tokens_in}` | Uncached input tokens only — the part that missed the cache |
| `{tokens_cache}` | Cache tokens (creation + read) |
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
