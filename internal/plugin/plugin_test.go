package plugin

import (
	"os"
	"path/filepath"
	"testing"
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
