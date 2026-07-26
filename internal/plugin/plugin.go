// Package plugin turns user-supplied data sources into status line segments.
//
// A plugin reports data, not appearance: a value, an optional maximum, maybe a
// label. Bars, colors and widths are the host's job, so every segment on the
// line — native or plugin — is drawn by the same code and honours the same
// bar_size, threshold and NO_COLOR settings. A plugin that genuinely needs to
// draw its own output can set "raw", which opts out of all of it.
package plugin

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Spec is one plugin as configured in claude-status-line.yaml.
type Spec struct {
	Name string `yaml:"name"`

	// Exactly one source. File is read directly and costs nothing at render
	// time; Command is cached and refreshed out of band, so a slow command
	// never lands on the render path.
	File    string `yaml:"file"`
	Command string `yaml:"command"`

	// Interval is how long a command's cached result stays fresh (default 60s).
	// Timeout caps one run of the command (default 5s). Both are Go durations:
	// "30s", "2m".
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`

	Icon    string `yaml:"icon"`
	Bar     bool   `yaml:"bar"`
	Display string `yaml:"display"`

	// Thresholds colour the segment by percentage. Ascending `at` values, each
	// applying upward. A descending list is how you express an inverted ramp
	// (more is better) without a separate flag.
	Thresholds []Threshold `yaml:"thresholds"`
}

type Threshold struct {
	At    int    `yaml:"at"`
	Color string `yaml:"color"`
}

// Output is what a plugin emits on stdout, or holds in its file.
type Output struct {
	Value *float64
	Max   *float64
	Text  string
	Label string
	State string // ok | warn | crit — skips threshold resolution when set
	Hide  bool
	Raw   string

	// Extra carries any key the contract doesn't define, reachable in a format
	// string as {plugin.<name>.<key>}. This is how one plugin reports several
	// pieces of information without needing to be several plugins.
	Extra map[string]string
}

// known lists the reserved keys, so everything else can fall through to Extra.
var known = map[string]bool{
	"value": true, "max": true, "text": true, "label": true,
	"state": true, "hide": true, "raw": true,
}

// MaxOutputBytes caps what a plugin may report. Without a limit, a command
// that accidentally prints a file ends up in the cache and then on the status
// line — a stray `cat` produced a two megabyte status line before this existed.
const MaxOutputBytes = 64 * 1024

// Parse reads plugin output. Anything that isn't a JSON object is taken as
// plain text, so `echo hello` is a valid plugin.
func Parse(b []byte) (Output, error) {
	if len(b) > MaxOutputBytes {
		return Output{}, fmt.Errorf("output is %d bytes, over the %d byte limit", len(b), MaxOutputBytes)
	}

	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return Output{Hide: true}, nil
	}

	if !strings.HasPrefix(trimmed, "{") {
		return Output{Text: firstLine(trimmed)}, nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Output{}, fmt.Errorf("parsing plugin output: %w", err)
	}

	out := Output{Extra: map[string]string{}}
	for k, v := range raw {
		if !known[k] {
			out.Extra[k] = toString(v)
			continue
		}
		switch k {
		case "value":
			out.Value = toFloat(v)
		case "max":
			out.Max = toFloat(v)
		case "text":
			out.Text = toString(v)
		case "label":
			out.Label = toString(v)
		case "state":
			out.State = toString(v)
		case "raw":
			out.Raw = toString(v)
		case "hide":
			b, _ := v.(bool)
			out.Hide = b
		}
	}
	return out, nil
}

// Load reads a plugin's source. Only file sources are wired up so far; a
// missing file hides the segment rather than failing the render, because a
// plugin that hasn't produced data yet is normal, not an error.
func (s Spec) Load() (Output, error) {
	switch {
	case s.File != "":
		b, err := os.ReadFile(expand(s.File))
		if err != nil {
			if os.IsNotExist(err) {
				return Output{Hide: true}, nil
			}
			return Output{}, err
		}
		return Parse(b)
	case s.Command != "":
		// Command sources go through Resolve, which reads the cache instead of
		// running anything. Load is the file path only.
		return Output{Hide: true}, fmt.Errorf("plugin %q: command sources resolve through the cache, not Load", s.Name)
	default:
		return Output{}, fmt.Errorf("plugin %q: needs a file or command source", s.Name)
	}
}

// Percent is the value as a share of max, or -1 when the plugin reported no
// max. A segment without a max still renders — it just has no bar, no
// percentage and no threshold colour.
func (o Output) Percent() int {
	if o.Value == nil || o.Max == nil || *o.Max <= 0 {
		return -1
	}
	return int(math.Round(*o.Value / *o.Max * 100))
}

// ColorName resolves the threshold list to a colour name, or "" to leave the
// segment unstyled. An explicit state on the output wins over the thresholds.
func (s Spec) ColorName(o Output) string {
	switch o.State {
	case "ok":
		return "green"
	case "warn":
		return "yellow"
	case "crit":
		return "red"
	}

	pct := o.Percent()
	if pct < 0 || len(s.Thresholds) == 0 {
		return ""
	}

	// Last matching stop wins, so the list reads top-to-bottom as increasing
	// bounds. Works equally for an inverted ramp, where the caller lists the
	// colours the other way round.
	name := ""
	for _, t := range s.Thresholds {
		if pct >= t.At {
			name = t.Color
		}
	}
	return name
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func toFloat(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		// Whole numbers are the common case and "30" beats "30.000000".
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func expand(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}
