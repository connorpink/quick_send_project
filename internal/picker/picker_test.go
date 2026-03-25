package picker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSelectUsesFZFWhenAvailable(t *testing.T) {
	var used string
	ids, err := Select(context.Background(), Options{
		Prompt: "pick> ",
		IsTTY:  func() bool { return true },
		LookPath: func(name string) (string, error) {
			if name == "fzf" {
				return "/usr/bin/fzf", nil
			}
			return "", errors.New("not found")
		},
		FZFRun: func(_ context.Context, _ Options, _ []Item) ([]string, error) {
			used = "fzf"
			return []string{"alpha"}, nil
		},
		GoRun: func(_ Options, _ []Item) ([]string, error) {
			used = "go"
			return []string{"beta"}, nil
		},
	}, []Item{{ID: "alpha", Label: "alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	if used != "fzf" {
		t.Fatalf("expected fzf backend, got %q", used)
	}
	if len(ids) != 1 || ids[0] != "alpha" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}

func TestSelectUsesGoFallbackWhenForced(t *testing.T) {
	var used string
	_, err := Select(context.Background(), Options{
		ForceGo: true,
		IsTTY:   func() bool { return true },
		LookPath: func(name string) (string, error) {
			return "/usr/bin/fzf", nil
		},
		FZFRun: func(_ context.Context, _ Options, _ []Item) ([]string, error) {
			used = "fzf"
			return []string{"alpha"}, nil
		},
		GoRun: func(_ Options, _ []Item) ([]string, error) {
			used = "go"
			return []string{"beta"}, nil
		},
	}, []Item{{ID: "beta", Label: "beta"}})
	if err != nil {
		t.Fatal(err)
	}
	if used != "go" {
		t.Fatalf("expected go backend, got %q", used)
	}
}

func TestParseSelectedIDs(t *testing.T) {
	ids, err := parseSelectedIDs(strings.NewReader("alpha\tAlpha\nbeta\tBeta\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "alpha" || ids[1] != "beta" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}
