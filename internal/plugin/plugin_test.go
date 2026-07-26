package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseJSONObject(t *testing.T) {
	out, err := Parse([]byte(`{"value": 12, "max": 33, "label": "aplicar", "pr": "#42"}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if out.Value == nil || *out.Value != 12 {
		t.Errorf("value = %v, want 12", out.Value)
	}
	if out.Max == nil || *out.Max != 33 {
		t.Errorf("max = %v, want 33", out.Max)
	}
	if out.Label != "aplicar" {
		t.Errorf("label = %q, want aplicar", out.Label)
	}
	// Unreserved keys are how one plugin reports several things.
	if out.Extra["pr"] != "#42" {
		t.Errorf("extra[pr] = %q, want #42", out.Extra["pr"])
	}
}

func TestParsePlainTextIsValid(t *testing.T) {
	out, err := Parse([]byte("12 tasks left\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if out.Text != "12 tasks left" {
		t.Errorf("text = %q", out.Text)
	}
}

func TestParseMultilineTextTakesFirstLine(t *testing.T) {
	// A stray second line must not break the status line into two.
	out, _ := Parse([]byte("first\nsecond\n"))
	if out.Text != "first" {
		t.Errorf("text = %q, want first", out.Text)
	}
}

func TestParseEmptyHides(t *testing.T) {
	out, err := Parse([]byte("   \n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !out.Hide {
		t.Error("empty output should hide the segment")
	}
}

func TestParseBrokenJSONErrors(t *testing.T) {
	if _, err := Parse([]byte(`{"value": }`)); err == nil {
		t.Error("want an error for malformed JSON")
	}
}

func TestParseNumericStringsCoerce(t *testing.T) {
	// jq and shell plugins commonly emit numbers as strings.
	out, _ := Parse([]byte(`{"value": "7", "max": "10"}`))
	if out.Value == nil || *out.Value != 7 {
		t.Errorf("value = %v, want 7", out.Value)
	}
	if got := out.Percent(); got != 70 {
		t.Errorf("Percent() = %d, want 70", got)
	}
}

func TestPercentWithoutMax(t *testing.T) {
	out, _ := Parse([]byte(`{"value": 5}`))
	if got := out.Percent(); got != -1 {
		t.Errorf("Percent() = %d, want -1 when no max is reported", got)
	}
}

func TestPercentZeroMaxDoesNotDivideByZero(t *testing.T) {
	out, _ := Parse([]byte(`{"value": 5, "max": 0}`))
	if got := out.Percent(); got != -1 {
		t.Errorf("Percent() = %d, want -1", got)
	}
}

func TestColorNameThresholds(t *testing.T) {
	spec := Spec{Thresholds: []Threshold{
		{At: 0, Color: "red"},
		{At: 31, Color: "yellow"},
		{At: 61, Color: "green"},
	}}

	cases := []struct {
		value, max float64
		want       string
	}{
		{0, 30, "red"},
		{4, 30, "red"},     // 13%
		{10, 30, "yellow"}, // 33%
		{15, 30, "yellow"}, // 50%
		{20, 30, "green"},  // 67%
		{30, 30, "green"},  // 100%
	}
	for _, c := range cases {
		v, m := c.value, c.max
		got := spec.ColorName(Output{Value: &v, Max: &m})
		if got != c.want {
			t.Errorf("%v/%v: color = %q, want %q", c.value, c.max, got, c.want)
		}
	}
}

func TestColorNameInvertedRampNeedsNoFlag(t *testing.T) {
	// Listing the colours the other way round is the whole mechanism.
	spec := Spec{Thresholds: []Threshold{
		{At: 0, Color: "green"},
		{At: 60, Color: "yellow"},
		{At: 85, Color: "red"},
	}}
	v, m := 90.0, 100.0
	if got := spec.ColorName(Output{Value: &v, Max: &m}); got != "red" {
		t.Errorf("color = %q, want red", got)
	}
}

func TestExplicitStateBeatsThresholds(t *testing.T) {
	spec := Spec{Thresholds: []Threshold{{At: 0, Color: "red"}}}
	v, m := 1.0, 100.0
	if got := spec.ColorName(Output{Value: &v, Max: &m, State: "ok"}); got != "green" {
		t.Errorf("color = %q, want green", got)
	}
}

func TestColorNameWithoutThresholds(t *testing.T) {
	v, m := 50.0, 100.0
	if got := (Spec{}).ColorName(Output{Value: &v, Max: &m}); got != "" {
		t.Errorf("color = %q, want empty when no thresholds are configured", got)
	}
}

func TestLoadMissingFileHidesRatherThanErrors(t *testing.T) {
	// A plugin that hasn't written its file yet is normal, not a failure.
	spec := Spec{Name: "x", File: filepath.Join(t.TempDir(), "nope.json")}
	out, err := spec.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !out.Hide {
		t.Error("a missing source file should hide the segment")
	}
}

func TestLoadReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	if err := os.WriteFile(path, []byte(`{"value":1,"max":4}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Spec{Name: "x", File: path}.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := out.Percent(); got != 25 {
		t.Errorf("Percent() = %d, want 25", got)
	}
}

func TestLoadWithoutSourceErrors(t *testing.T) {
	if _, err := (Spec{Name: "x"}).Load(); err == nil {
		t.Error("want an error when neither file nor command is set")
	}
}

// A command that accidentally prints a file must not reach the status line.
func TestParseRejectsOversizedOutput(t *testing.T) {
	big := make([]byte, MaxOutputBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if _, err := Parse(big); err == nil {
		t.Error("want an error for output over the size limit")
	}
	if _, err := Parse(big[:MaxOutputBytes]); err != nil {
		t.Errorf("output exactly at the limit should be accepted: %v", err)
	}
}

func TestShouldHide(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	cases := []struct {
		name     string
		hideWhen string
		out      Output
		want     bool
	}{
		{"zero hides a zero value", "zero", Output{Value: f(0), Max: f(30)}, true},
		{"zero keeps a non-zero value", "zero", Output{Value: f(4), Max: f(30)}, false},
		{"full hides a complete counter", "full", Output{Value: f(10), Max: f(10)}, true},
		{"full hides an over-complete counter", "full", Output{Value: f(11), Max: f(10)}, true},
		{"full keeps an incomplete counter", "full", Output{Value: f(9), Max: f(10)}, false},
		{"full needs a max", "full", Output{Value: f(9)}, false},
		{"unset never hides", "", Output{Value: f(0)}, false},
		{"never never hides", "never", Output{Value: f(0)}, false},
		{"unknown value is ignored", "sometimes", Output{Value: f(0)}, false},
		{"no value is never hidden", "zero", Output{Text: "hi"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (Spec{Name: "x", HideWhen: c.hideWhen}).ShouldHide(c.out); got != c.want {
				t.Errorf("ShouldHide = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPluginNameFromFile(t *testing.T) {
	cases := []struct {
		file string
		want string
		ok   bool
	}{
		{"issues-aabbccdd1122.json", "issues", true},
		// A name containing dashes still resolves, because the hash is fixed width.
		{"my-dashed-plugin-aabbccdd1122.json", "my-dashed-plugin", true},
		{"issues-aabbccdd1122.json.lock", "", false},
		{"nothash-zzzzzzzzzzzz.json", "", false},
		{"short.json", "", false},
		{"random.txt", "", false},
	}
	for _, c := range cases {
		got, ok := pluginNameFromFile(c.file)
		if ok != c.ok || got != c.want {
			t.Errorf("pluginNameFromFile(%q) = %q,%v — want %q,%v", c.file, got, ok, c.want, c.ok)
		}
	}
}

func TestPruneRemovesOrphansAndExpiredOnly(t *testing.T) {
	dir := tempCache(t)
	plugins := filepath.Join(dir, "claude-status-line-go", "plugins")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(file string, age time.Duration) string {
		p := filepath.Join(plugins, file)
		if err := os.WriteFile(p, []byte(`{"fetched_at":1,"raw":"{}"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
		return p
	}

	live := write("keep-aaaaaaaaaaaa.json", time.Minute)
	otherProject := write("keep-bbbbbbbbbbbb.json", time.Hour)
	orphan := write("gone-cccccccccccc.json", time.Minute)
	expired := write("keep-dddddddddddd.json", 30*24*time.Hour)
	freshLock := write("keep-eeeeeeeeeeee.json.lock", time.Minute)
	staleLock := write("keep-ffffffffffff.json.lock", 3*time.Hour)

	got, err := Prune([]string{"keep"}, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if len(got.Orphaned) != 1 || got.Orphaned[0] != "gone" {
		t.Errorf("orphaned = %v, want [gone]", got.Orphaned)
	}
	if len(got.Expired) != 1 || got.Expired[0] != "keep" {
		t.Errorf("expired = %v, want [keep]", got.Expired)
	}

	for _, p := range []string{live, otherProject, freshLock} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should have survived", filepath.Base(p))
		}
	}
	for _, p := range []string{orphan, expired, staleLock} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s should have been removed", filepath.Base(p))
		}
	}
}

func TestPruneWithNoCacheDirectory(t *testing.T) {
	tempCache(t) // points at an empty temp dir; the plugins subdir doesn't exist
	got, err := Prune([]string{"x"}, time.Hour)
	if err != nil {
		t.Errorf("Prune on a missing cache dir should not error: %v", err)
	}
	if got.Total() != 0 {
		t.Errorf("removed %d entries from nothing", got.Total())
	}
}
