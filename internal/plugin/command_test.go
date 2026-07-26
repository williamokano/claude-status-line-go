package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tempCache points the plugin cache at a temp dir for the duration of a test.
func tempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := userCacheDir
	userCacheDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userCacheDir = orig })
	return dir
}

func seedCache(t *testing.T, s Spec, projectDir, raw string, age time.Duration) {
	t.Helper()
	path := s.CachePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(entry{FetchedAt: time.Now().Add(-age).Unix(), Raw: raw})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveWithNoCacheHidesAndAsksForRefresh(t *testing.T) {
	tempCache(t)
	s := Spec{Name: "pr", Command: "echo hi"}

	out, refresh, err := s.Resolve("/proj")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !out.Hide {
		t.Error("want the segment hidden until there's a cached value")
	}
	if !refresh {
		t.Error("want a refresh requested on a cold cache")
	}
}

func TestResolveFreshCacheDoesNotRefresh(t *testing.T) {
	tempCache(t)
	s := Spec{Name: "pr", Command: "echo hi", Interval: "60s"}
	seedCache(t, s, "/proj", `{"value":3,"max":9}`, 5*time.Second)

	out, refresh, err := s.Resolve("/proj")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := out.Percent(); got != 33 {
		t.Errorf("Percent() = %d, want 33", got)
	}
	if refresh {
		t.Error("a value 5s into a 60s interval should not trigger a refresh")
	}
}

// The point of the design: a stale value is still drawn, immediately, while
// the refresh happens behind it.
func TestResolveStaleCacheServesValueAndRefreshes(t *testing.T) {
	tempCache(t)
	s := Spec{Name: "pr", Command: "echo hi", Interval: "10s"}
	seedCache(t, s, "/proj", `{"value":7,"max":10}`, 30*time.Second)

	out, refresh, err := s.Resolve("/proj")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out.Hide {
		t.Error("a stale value should still render")
	}
	if got := out.Percent(); got != 70 {
		t.Errorf("Percent() = %d, want 70", got)
	}
	if !refresh {
		t.Error("want a refresh requested for a stale value")
	}
}

// Task counts and PR state are per-repo; sharing a cache across projects would
// show one project's numbers while you're sitting in another.
func TestCachePathIsPerProject(t *testing.T) {
	tempCache(t)
	s := Spec{Name: "pr", Command: "echo hi"}

	if a, b := s.CachePath("/one"), s.CachePath("/two"); a == b {
		t.Errorf("both projects share %s", a)
	}
	if a, b := s.CachePath("/one"), s.CachePath("/one"); a != b {
		t.Errorf("same project gave %s and %s", a, b)
	}
}

func TestRefreshThenResolveRoundTrip(t *testing.T) {
	tempCache(t)
	dir := t.TempDir()
	s := Spec{Name: "counter", Command: `echo '{"value":4,"max":30}'`, Interval: "60s"}

	if err := s.Refresh(dir, nil); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	out, refresh, err := s.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out.Hide {
		t.Fatal("want a visible segment after a successful refresh")
	}
	if got := out.Percent(); got != 13 {
		t.Errorf("Percent() = %d, want 13", got)
	}
	if refresh {
		t.Error("a value just written should be fresh")
	}
}

// A command that starts failing must not replace a good value with nothing.
func TestRefreshKeepsLastGoodValueOnFailure(t *testing.T) {
	tempCache(t)
	dir := t.TempDir()
	s := Spec{Name: "flaky", Command: "exit 1", Interval: "60s"}
	seedCache(t, s, dir, `{"value":5,"max":10}`, time.Second)

	if err := s.Refresh(dir, nil); err == nil {
		t.Error("want an error from a failing command")
	}

	out, _, err := s.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := out.Percent(); got != 50 {
		t.Errorf("Percent() = %d, want the previous 50", got)
	}
}

func TestRefreshRejectsUnparseableOutput(t *testing.T) {
	tempCache(t)
	dir := t.TempDir()
	s := Spec{Name: "bad", Command: `echo '{"value": '`, Interval: "60s"}
	seedCache(t, s, dir, `{"value":5,"max":10}`, time.Second)

	if err := s.Refresh(dir, nil); err == nil {
		t.Error("want an error for malformed output")
	}
	out, _, _ := s.Resolve(dir)
	if got := out.Percent(); got != 50 {
		t.Errorf("Percent() = %d, want the previous 50 to survive", got)
	}
}

func TestRefreshTimesOut(t *testing.T) {
	tempCache(t)
	s := Spec{Name: "slow", Command: "sleep 5", Timeout: "150ms"}

	start := time.Now()
	err := s.Refresh(t.TempDir(), nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("want an error when the command exceeds its timeout")
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %v — the timeout did not kill the command", elapsed)
	}
}

func TestRefreshPassesStdinToCommand(t *testing.T) {
	tempCache(t)
	dir := t.TempDir()
	// Echo back a field from the session JSON to prove it arrived.
	s := Spec{Name: "echo", Command: `cat`, Interval: "60s"}

	if err := s.Refresh(dir, []byte(`{"text":"from-stdin"}`)); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	out, _, _ := s.Resolve(dir)
	if out.Text != "from-stdin" {
		t.Errorf("text = %q, want from-stdin", out.Text)
	}
}

// Claude Code re-invokes the status line freely; without the lock a slow
// command would collect a new process on every render.
func TestRefreshLockPreventsStampede(t *testing.T) {
	tempCache(t)
	dir := t.TempDir()

	marker := filepath.Join(dir, "runs")
	s := Spec{
		Name:    "slow",
		Command: `echo x >> ` + marker + `; sleep 0.4; echo '{"value":1,"max":2}'`,
		Timeout: "5s",
	}

	var wg sync.WaitGroup
	var errs atomic.Int32
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Refresh(dir, nil); err != nil {
				errs.Add(1)
			}
		}()
	}
	wg.Wait()

	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the command never ran: %v", err)
	}
	if runs := len(b); runs != 2 {
		t.Errorf("command ran %d times, want exactly 1 (marker %q)", runs/2, b)
	}
}

func TestIntervalAndTimeoutDefaults(t *testing.T) {
	s := Spec{Name: "x"}
	if got := s.IntervalDuration(); got != defaultInterval {
		t.Errorf("interval = %v, want %v", got, defaultInterval)
	}
	if got := s.TimeoutDuration(); got != defaultTimeout {
		t.Errorf("timeout = %v, want %v", got, defaultTimeout)
	}
}

func TestBadDurationFallsBackToDefault(t *testing.T) {
	s := Spec{Name: "x", Interval: "banana", Timeout: "-3s"}
	if got := s.IntervalDuration(); got != defaultInterval {
		t.Errorf("interval = %v, want the default %v", got, defaultInterval)
	}
	if got := s.TimeoutDuration(); got != defaultTimeout {
		t.Errorf("timeout = %v, want the default %v", got, defaultTimeout)
	}
}

func TestStaleLockIsStolen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	release, ok := acquireLock(path, time.Minute)
	if !ok {
		t.Fatal("want an abandoned lock to be taken over")
	}
	release()
}
