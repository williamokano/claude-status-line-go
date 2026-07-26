package plugin

// Command sources never run on the render path.
//
// A render reads a cache file and nothing else. When that file is older than
// the plugin's interval it hands the work to a detached copy of this binary
// and draws the stale value immediately, so the status line costs the same
// whether a plugin takes 5ms or 5 seconds. The alternative — running the
// command inline — puts a 344ms `gh` call in front of a 3ms render, on a
// command Claude Code re-runs constantly.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultInterval = 60 * time.Second
	defaultTimeout  = 5 * time.Second

	// RefreshFlag is the hidden mode a detached refresh runs in.
	RefreshFlag = "--refresh-plugin"
)

// entry is one cached plugin result.
type entry struct {
	FetchedAt int64  `json:"fetched_at"`
	Raw       string `json:"raw"`
}

// Interval is how long a cached value stays fresh.
func (s Spec) IntervalDuration() time.Duration {
	return parseDuration(s.Interval, defaultInterval, s.Name, "interval")
}

// Timeout caps a single run of the command.
func (s Spec) TimeoutDuration() time.Duration {
	return parseDuration(s.Timeout, defaultTimeout, s.Name, "timeout")
}

func parseDuration(raw string, def time.Duration, name, field string) time.Duration {
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		fmt.Fprintf(os.Stderr, "claude-status-line-go: plugin %q: bad %s %q, using %s\n", name, field, raw, def)
		return def
	}
	return d
}

// CacheAge is how long ago this plugin's cached value was written, for the
// `plugins` command. ok is false when there's no usable cache yet.
func (s Spec) CacheAge(projectDir string) (time.Duration, bool) {
	e, err := readCache(s.CachePath(projectDir))
	if err != nil {
		return 0, false
	}
	return time.Since(time.Unix(e.FetchedAt, 0)), true
}

// Resolve produces the segment data for this render. needsRefresh reports that
// the caller should kick off a background refresh; it never means "wait".
func (s Spec) Resolve(projectDir string) (out Output, needsRefresh bool, err error) {
	if s.Command == "" {
		out, err = s.Load()
		return out, false, err
	}

	e, readErr := readCache(s.CachePath(projectDir))
	if readErr != nil {
		// No cache yet, or an unreadable one. Show nothing and go fetch.
		return Output{Hide: true}, true, nil
	}

	out, err = Parse([]byte(e.Raw))
	if err != nil {
		return Output{Hide: true}, true, nil
	}

	age := time.Since(time.Unix(e.FetchedAt, 0))
	return out, age >= s.IntervalDuration(), nil
}

// CachePath keys on the project directory as well as the plugin name: task
// counts and PR state are per-repo, and a shared cache would show one
// project's numbers while you're sitting in another.
// userCacheDir is a variable so tests can redirect the cache without depending
// on the platform's real cache location.
var userCacheDir = os.UserCacheDir

func (s Spec) CachePath(projectDir string) string {
	base, err := userCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	if projectDir == "" {
		projectDir = "global"
	}
	sum := sha256.Sum256([]byte(projectDir))
	file := fmt.Sprintf("%s-%s.json", sanitize(s.Name), hex.EncodeToString(sum[:])[:12])
	return filepath.Join(base, "claude-status-line-go", "plugins", file)
}

// Refresh runs the command and replaces the cache. This is the detached
// child's whole job; nothing on the render path calls it.
func (s Spec) Refresh(projectDir string, input []byte) error {
	if s.Command == "" {
		return fmt.Errorf("plugin %q: no command to run", s.Name)
	}

	path := s.CachePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// One refresh at a time. Claude Code re-invokes the status line freely,
	// and without this a slow command collects a new process on every render.
	release, ok := acquireLock(path+".lock", s.TimeoutDuration()*2)
	if !ok {
		return nil // another refresh already has it
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), s.TimeoutDuration())
	defer cancel()

	name, args := shell(s.Command)
	cmd := exec.CommandContext(ctx, name, args...)
	isolate(cmd)
	// Belt and braces: even with the group killed, don't let a stray inherited
	// pipe hold this process open indefinitely.
	cmd.WaitDelay = time.Second
	cmd.Env = append(os.Environ(),
		"CSL_PROJECT_DIR="+projectDir,
		"CSL_PLUGIN_NAME="+s.Name,
	)
	if projectDir != "" {
		cmd.Dir = projectDir
	}
	cmd.Stdin = strings.NewReader(string(input))

	// Capture stderr rather than discarding it. A plugin is a script someone
	// wrote and can't attach a debugger to; without this, a command that fails
	// inside a pipeline just produces an empty segment and no explanation.
	var stderr strings.Builder
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("plugin %q: %w%s", s.Name, err, formatStderr(stderr.String()))
	}

	// A pipeline reports the exit status of its last command, so `gh ... | jq`
	// exits 0 even when gh fails. Treat empty output from a command that also
	// wrote to stderr as a failure rather than caching the silence.
	if len(strings.TrimSpace(string(stdout))) == 0 && stderr.Len() > 0 {
		return fmt.Errorf("plugin %q: no output%s", s.Name, formatStderr(stderr.String()))
	}

	// Only overwrite the cache once the output parses. A command that starts
	// failing shouldn't replace a good value with garbage.
	if _, err := Parse(stdout); err != nil {
		return fmt.Errorf("plugin %q: %w%s", s.Name, err, formatStderr(stderr.String()))
	}

	return writeCache(path, entry{FetchedAt: time.Now().Unix(), Raw: string(stdout)})
}

// SpawnRefresh starts a detached copy of this binary to refresh one plugin and
// returns immediately. The child outlives us, which is the point: the render
// process must exit at once.
func SpawnRefresh(name, projectDir string, input []byte) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(self, RefreshFlag, name)
	// The child must key the cache on the same directory the render did, or
	// it writes an entry the next render will never look for.
	cmd.Env = append(os.Environ(), "CSL_PROJECT_DIR="+projectDir)
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return err
	}
	// Release lets the child be reaped by init instead of leaving a zombie
	// attached to a process that is about to exit anyway.
	return cmd.Process.Release()
}

// formatStderr appends a plugin's stderr to an error message, trimmed so one
// noisy command can't flood the terminal.
func formatStderr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	const max = 300
	if len(s) > max {
		s = s[:max] + "…"
	}
	return ": " + strings.ReplaceAll(s, "\n", " / ")
}

func readCache(path string) (entry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return entry{}, err
	}
	var e entry
	if err := json.Unmarshal(b, &e); err != nil {
		return entry{}, err
	}
	return e, nil
}

// writeCache replaces the file atomically, so a render never reads a half
// written cache entry.
func writeCache(path string, e entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// acquireLock takes an exclusive lock, stealing one that has outlived the
// window in which its owner could still be running — otherwise a killed
// refresh would block every future one.
func acquireLock(path string, stale time.Duration) (func(), bool) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return func() {}, false
	}

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return func() { os.Remove(path) }, true
		}

		info, statErr := os.Stat(path)
		if statErr != nil || time.Since(info.ModTime()) < stale {
			return func() {}, false
		}
		os.Remove(path) // abandoned, take it over
	}
	return func() {}, false
}

func shell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", command}
	}
	return "sh", []string{"-c", command}
}

// sanitize keeps a plugin name usable as a filename.
func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, name)
}
