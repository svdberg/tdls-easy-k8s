package cli

import (
	"os"
	"strings"
	"testing"
)

func TestRootCommand_Exists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}
	if rootCmd.Use != "tdls-easy-k8s" {
		t.Errorf("expected root command use 'tdls-easy-k8s', got %q", rootCmd.Use)
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	commands := rootCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name()] = true
	}

	expected := []string{"init", "gitops", "app", "version"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

func TestVersionCommand_Output(t *testing.T) {
	// Capture stdout since the version command uses fmt.Printf
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs([]string{"version"})
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = origStdout

	if execErr != nil {
		t.Fatalf("version command failed: %v", execErr)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "tdls-easy-k8s") {
		t.Errorf("expected version output to contain 'tdls-easy-k8s', got: %s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("expected version output to contain 'commit:', got: %s", output)
	}
	if !strings.Contains(output, "go version:") {
		t.Errorf("expected version output to contain 'go version:', got: %s", output)
	}
}

func TestInitCommand_HasFlags(t *testing.T) {
	flags := initCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"provider", "aws"},
		{"region", "us-east-1"},
		{"name", ""},
		{"nodes", "3"},
		{"generate-config", "false"},
	}

	for _, tc := range cases {
		f := flags.Lookup(tc.name)
		if f == nil {
			t.Errorf("expected flag %q to exist", tc.name)
			continue
		}
		if f.DefValue != tc.defValue {
			t.Errorf("flag %q: expected default %q, got %q", tc.name, tc.defValue, f.DefValue)
		}
	}
}

func TestGitopsSetupCommand_HasFlags(t *testing.T) {
	flags := gitopsSetupCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"repo", ""},
		{"branch", "main"},
		{"path", "clusters/production"},
	}

	for _, tc := range cases {
		f := flags.Lookup(tc.name)
		if f == nil {
			t.Errorf("expected flag %q to exist", tc.name)
			continue
		}
		if f.DefValue != tc.defValue {
			t.Errorf("flag %q: expected default %q, got %q", tc.name, tc.defValue, f.DefValue)
		}
	}
}

func TestAppAddCommand_HasFlags(t *testing.T) {
	flags := appAddCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"chart", ""},
		{"values", ""},
		{"namespace", "default"},
	}

	for _, tc := range cases {
		f := flags.Lookup(tc.name)
		if f == nil {
			t.Errorf("expected flag %q to exist", tc.name)
			continue
		}
		if f.DefValue != tc.defValue {
			t.Errorf("flag %q: expected default %q, got %q", tc.name, tc.defValue, f.DefValue)
		}
	}
}

func TestGitopsCommand_HasSetupSubcommand(t *testing.T) {
	commands := gitopsCmd.Commands()
	found := false
	for _, cmd := range commands {
		if cmd.Name() == "setup" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'setup' subcommand under 'gitops'")
	}
}

func TestAppCommand_HasAddSubcommand(t *testing.T) {
	commands := appCmd.Commands()
	found := false
	for _, cmd := range commands {
		if cmd.Name() == "add" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'add' subcommand under 'app'")
	}
}
