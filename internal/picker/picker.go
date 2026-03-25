package picker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
)

var ErrCancelled = errors.New("selection cancelled")

type Item struct {
	ID      string
	Label   string
	Details []string
}

type Options struct {
	Prompt   string
	Multi    bool
	ForceGo  bool
	Stdout   io.Writer
	Stderr   io.Writer
	IsTTY    func() bool
	LookPath func(string) (string, error)
	FZFRun   func(context.Context, Options, []Item) ([]string, error)
	GoRun    func(Options, []Item) ([]string, error)
}

func Select(ctx context.Context, opts Options, items []Item) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items available")
	}
	isTTY := opts.IsTTY
	if isTTY == nil {
		isTTY = IsInteractive
	}
	if !isTTY() {
		return nil, fmt.Errorf("interactive selection requires a terminal")
	}

	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.FZFRun == nil {
		opts.FZFRun = runFZF
	}
	if opts.GoRun == nil {
		opts.GoRun = runGoFuzzy
	}

	if !opts.ForceGo {
		if _, err := opts.LookPath("fzf"); err == nil {
			return opts.FZFRun(ctx, opts, items)
		}
	}
	return opts.GoRun(opts, items)
}

func SelectOne(ctx context.Context, opts Options, items []Item) (string, error) {
	ids, err := Select(ctx, opts, items)
	if err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("expected one selection, got %d", len(ids))
	}
	return ids[0], nil
}

func IsInteractive() bool {
	return isCharacterDevice(os.Stdin) && isCharacterDevice(os.Stdout)
}

func isCharacterDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func runFZF(ctx context.Context, opts Options, items []Item) ([]string, error) {
	var input strings.Builder
	for _, item := range items {
		row := []string{item.ID, item.Label}
		row = append(row, item.Details...)
		input.WriteString(strings.Join(row, "\t"))
		input.WriteByte('\n')
	}

	args := []string{
		"--prompt", opts.Prompt,
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--height", "40%",
		"--layout", "reverse",
		"--border",
	}
	if opts.Multi {
		args = append(args, "--multi")
	}

	cmd := exec.CommandContext(ctx, "fzf", args...)
	cmd.Stdin = strings.NewReader(input.String())
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = opts.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 1 {
				return nil, ErrCancelled
			}
		}
		return nil, err
	}
	return parseSelectedIDs(&stdout, opts.Multi)
}

func runGoFuzzy(opts Options, items []Item) ([]string, error) {
	preview := func(i, _, _ int) string {
		if i < 0 || i >= len(items) {
			return ""
		}
		return strings.Join(items[i].Details, "\n")
	}
	if opts.Multi {
		idxs, err := fuzzyfinder.FindMulti(
			items,
			func(i int) string { return items[i].Label },
			fuzzyfinder.WithPromptString(opts.Prompt),
			fuzzyfinder.WithPreviewWindow(preview),
		)
		if err != nil {
			if errors.Is(err, fuzzyfinder.ErrAbort) {
				return nil, ErrCancelled
			}
			return nil, err
		}
		ids := make([]string, 0, len(idxs))
		for _, idx := range idxs {
			ids = append(ids, items[idx].ID)
		}
		return ids, nil
	}

	idx, err := fuzzyfinder.Find(
		items,
		func(i int) string { return items[i].Label },
		fuzzyfinder.WithPromptString(opts.Prompt),
		fuzzyfinder.WithPreviewWindow(preview),
	)
	if err != nil {
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	return []string{items[idx].ID}, nil
}

func parseSelectedIDs(r io.Reader, multi bool) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	output := strings.TrimSpace(string(data))
	if output == "" {
		return nil, ErrCancelled
	}
	lines := strings.Split(output, "\n")
	ids := make([]string, 0, len(lines))
	for _, line := range lines {
		id, _, _ := strings.Cut(strings.TrimSpace(line), "\t")
		if id == "" {
			return nil, fmt.Errorf("invalid selection")
		}
		ids = append(ids, id)
	}
	if !multi && len(ids) != 1 {
		return nil, fmt.Errorf("invalid selection count")
	}
	return ids, nil
}
