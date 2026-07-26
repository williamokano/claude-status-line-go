package service

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/williamokano/claude-status-line-go/internal/config"
	"github.com/williamokano/claude-status-line-go/internal/plugin"
)

// stubLoader swaps loadPlugin for the duration of a test.
func stubLoader(t *testing.T, fn func(plugin.Spec) (plugin.Output, error)) {
	t.Helper()
	orig := loadPlugin
	loadPlugin = fn
	t.Cleanup(func() { loadPlugin = orig })
}

func specs(names ...string) []plugin.Spec {
	out := make([]plugin.Spec, 0, len(names))
	for _, n := range names {
		out = append(out, plugin.Spec{Name: n, File: "/dev/null"})
	}
	return out
}

// The whole point of the fan-out: five plugins that each take 50ms must finish
// in roughly 50ms, not 250ms.
func TestBuildPluginsRunsConcurrently(t *testing.T) {
	const (
		delay = 50 * time.Millisecond
		n     = 5
	)

	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		time.Sleep(delay)
		return plugin.Parse([]byte(fmt.Sprintf("%q", s.Name)))
	})

	svc := New(config.Config{BarSize: 10, Plugins: specs("a", "b", "c", "d", "e")})

	start := time.Now()
	got := svc.buildPlugins()
	elapsed := time.Since(start)

	if len(got) != n {
		t.Fatalf("got %d segments, want %d", len(got), n)
	}
	// Serial would be n*delay = 250ms. Allow generous headroom for slow CI
	// while still failing loudly if the fan-out is lost.
	if limit := time.Duration(n-1) * delay; elapsed >= limit {
		t.Errorf("took %v, want well under %v — plugins appear to be serialised", elapsed, limit)
	}
}

// Goroutines finish in whatever order they like; the line must not reshuffle.
func TestBuildPluginsPreservesDeclarationOrder(t *testing.T) {
	// Make the first plugin the slowest, so completion order is the reverse of
	// declaration order and a naive append would be visibly wrong.
	delays := map[string]time.Duration{
		"first": 60 * time.Millisecond,
		"mid":   30 * time.Millisecond,
		"last":  0,
	}
	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		time.Sleep(delays[s.Name])
		return plugin.Output{Text: s.Name}, nil
	})

	svc := New(config.Config{BarSize: 10, Plugins: specs("first", "mid", "last")})

	got := svc.buildPlugins()
	want := []string{"first", "mid", "last"}
	if len(got) != len(want) {
		t.Fatalf("got %d segments, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].name != name {
			t.Errorf("position %d = %q, want %q", i, got[i].name, name)
		}
	}
}

func TestBuildPluginsLoadsEachSpecExactlyOnce(t *testing.T) {
	var calls atomic.Int32
	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		calls.Add(1)
		return plugin.Output{Text: s.Name}, nil
	})

	svc := New(config.Config{BarSize: 10, Plugins: specs("a", "b", "c")})
	svc.buildPlugins()

	if got := calls.Load(); got != 3 {
		t.Errorf("loader called %d times, want 3", got)
	}
}

// One failing plugin must cost only its own segment.
func TestBuildPluginsIsolatesFailures(t *testing.T) {
	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		if s.Name == "bad" {
			return plugin.Output{}, fmt.Errorf("boom")
		}
		return plugin.Output{Text: s.Name}, nil
	})

	svc := New(config.Config{BarSize: 10, Plugins: specs("good", "bad", "alsogood")})

	got := svc.buildPlugins()
	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	if got[0].name != "good" || got[1].name != "alsogood" {
		t.Errorf("got %q and %q, want good and alsogood", got[0].name, got[1].name)
	}
}

func TestBuildPluginsDropsHiddenAndEmpty(t *testing.T) {
	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		switch s.Name {
		case "hidden":
			return plugin.Output{Hide: true}, nil
		case "empty":
			return plugin.Output{}, nil
		}
		return plugin.Output{Text: s.Name}, nil
	})

	svc := New(config.Config{BarSize: 10, Plugins: specs("hidden", "shown", "empty")})

	got := svc.buildPlugins()
	if len(got) != 1 || got[0].name != "shown" {
		t.Fatalf("got %+v, want only the shown segment", got)
	}
}

func TestBuildPluginsNoneConfigured(t *testing.T) {
	if got := New(config.Config{BarSize: 10}).buildPlugins(); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// Plugin segments must land in the default layout ahead of cost, in order.
func TestRenderPlacesPluginsBeforeCost(t *testing.T) {
	stubLoader(t, func(s plugin.Spec) (plugin.Output, error) {
		return plugin.Output{Text: s.Name}, nil
	})

	svc := New(config.Config{
		BarSize: 10, ShowCost: true, ShowTokens: true,
		Plugins: specs("alpha", "beta"),
	})

	out := stripANSI(svc.render(svc.buildView(Input{})))

	iAlpha := strings.Index(out, "alpha")
	iBeta := strings.Index(out, "beta")
	iCost := strings.Index(out, "$")
	if iAlpha < 0 || iBeta < 0 || iCost < 0 {
		t.Fatalf("missing a segment in %q", out)
	}
	if !(iAlpha < iBeta && iBeta < iCost) {
		t.Errorf("want alpha < beta < cost, got %d/%d/%d in %q", iAlpha, iBeta, iCost, out)
	}
}
