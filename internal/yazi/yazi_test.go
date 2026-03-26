package yazi

import (
	"strings"
	"testing"
)

func TestExamplePluginKeymapUsesPluginAction(t *testing.T) {
	snippet := ExamplePluginKeymap()
	if !strings.Contains(snippet, `run = "plugin sendrecv"`) {
		t.Fatalf("expected plugin snippet to use plugin action, got %q", snippet)
	}
	if !strings.Contains(snippet, "[[mgr.prepend_keymap]]") {
		t.Fatalf("expected mgr.prepend_keymap, got %q", snippet)
	}
}

func TestExampleFixedHostKeymapUsesRemoteHostFlag(t *testing.T) {
	snippet := ExampleFixedHostKeymap("laptop")
	if !strings.Contains(snippet, "--remote-host laptop") {
		t.Fatalf("expected snippet to use --remote-host, got %q", snippet)
	}
}

func TestExampleShellPickerKeymapUsesSendCommand(t *testing.T) {
	snippet := ExampleShellPickerKeymap()
	if !strings.Contains(snippet, `sendrecv send \"$@\"`) {
		t.Fatalf("expected shell picker snippet to use send command, got %q", snippet)
	}
}
