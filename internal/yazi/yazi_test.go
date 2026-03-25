package yazi

import (
	"strings"
	"testing"
)

func TestExamplePickerKeymapUsesSendCommand(t *testing.T) {
	snippet := ExamplePickerKeymap()
	if !strings.Contains(snippet, `sendrecv send \"$@\"`) {
		t.Fatalf("expected picker snippet to use send command, got %q", snippet)
	}
	if !strings.Contains(snippet, "[[mgr.prepend_keymap]]") {
		t.Fatalf("expected mgr.prepend_keymap, got %q", snippet)
	}
}

func TestExampleKeymapUsesRemoteHostFlag(t *testing.T) {
	snippet := ExampleKeymap("laptop")
	if !strings.Contains(snippet, "--remote-host laptop") {
		t.Fatalf("expected snippet to use --remote-host, got %q", snippet)
	}
}
