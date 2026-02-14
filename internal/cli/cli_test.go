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

	expected := []string{"init", "gitops", "app", "version", "destroy", "status", "validate", "kubeconfig"}
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
		{"repo-url", ""},
		{"version", "*"},
		{"layer", "apps"},
		{"output-dir", ""},
		{"gitops-path", "clusters/production"},
		{"depends-on", ""},
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

func TestParseChartReference(t *testing.T) {
	cases := []struct {
		input     string
		wantRepo  string
		wantChart string
		wantErr   bool
	}{
		{"bitnami/nginx", "bitnami", "nginx", false},
		{"mycompany/myapp", "mycompany", "myapp", false},
		{"noseparator", "", "", true},
		{"too/many/slashes", "", "", true},
		{"/empty-repo", "", "", true},
		{"empty-chart/", "", "", true},
	}

	for _, tc := range cases {
		repo, chart, err := parseChartReference(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseChartReference(%q): expected error, got repo=%q chart=%q", tc.input, repo, chart)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseChartReference(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if repo != tc.wantRepo {
			t.Errorf("parseChartReference(%q): repo = %q, want %q", tc.input, repo, tc.wantRepo)
		}
		if chart != tc.wantChart {
			t.Errorf("parseChartReference(%q): chart = %q, want %q", tc.input, chart, tc.wantChart)
		}
	}
}

func TestGenerateAppKustomizationYAML_NoDependency(t *testing.T) {
	yaml := generateAppKustomizationYAML("my-api", "apps", "")

	expected := []string{
		"apiVersion: kustomize.toolkit.fluxcd.io/v1",
		"kind: Kustomization",
		"name: my-api",
		"namespace: flux-system",
		"path: ./apps/my-api",
		"prune: true",
		"wait: true",
	}
	for _, s := range expected {
		if !strings.Contains(yaml, s) {
			t.Errorf("expected YAML to contain %q, got:\n%s", s, yaml)
		}
	}
	if strings.Contains(yaml, "dependsOn") {
		t.Errorf("expected no dependsOn block, got:\n%s", yaml)
	}
}

func TestGenerateAppKustomizationYAML_WithDependency(t *testing.T) {
	yaml := generateAppKustomizationYAML("my-api", "apps", "redis")

	if !strings.Contains(yaml, "dependsOn") {
		t.Errorf("expected dependsOn block, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "name: redis") {
		t.Errorf("expected dependency on redis, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "path: ./apps/my-api") {
		t.Errorf("expected path ./apps/my-api, got:\n%s", yaml)
	}
}

func TestGenerateHelmRepositoryYAML(t *testing.T) {
	yaml := generateHelmRepositoryYAML("bitnami", "https://charts.bitnami.com/bitnami")

	expected := []string{
		"apiVersion: source.toolkit.fluxcd.io/v1",
		"kind: HelmRepository",
		"name: bitnami",
		"namespace: flux-system",
		"interval: 1h0m0s",
		"url: https://charts.bitnami.com/bitnami",
	}
	for _, s := range expected {
		if !strings.Contains(yaml, s) {
			t.Errorf("expected YAML to contain %q, got:\n%s", s, yaml)
		}
	}
}

func TestGenerateHelmReleaseYAML_NoValues(t *testing.T) {
	yaml := generateHelmReleaseYAML("my-api", "default", "nginx", "bitnami", "*", "")

	expected := []string{
		"apiVersion: helm.toolkit.fluxcd.io/v2",
		"kind: HelmRelease",
		"name: my-api",
		"namespace: default",
		"chart: nginx",
		`version: "*"`,
		"name: bitnami",
		"namespace: flux-system",
	}
	for _, s := range expected {
		if !strings.Contains(yaml, s) {
			t.Errorf("expected YAML to contain %q, got:\n%s", s, yaml)
		}
	}
	if strings.Contains(yaml, "values:") {
		t.Errorf("expected no values block, got:\n%s", yaml)
	}
}

func TestGenerateHelmReleaseYAML_WithValues(t *testing.T) {
	values := "replicaCount: 3\nimage:\n  tag: latest"
	yaml := generateHelmReleaseYAML("my-api", "production", "nginx", "bitnami", "1.2.3", values)

	if !strings.Contains(yaml, "values:") {
		t.Errorf("expected values block, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "replicaCount: 3") {
		t.Errorf("expected replicaCount in values, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "tag: latest") {
		t.Errorf("expected image tag in values, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, `version: "1.2.3"`) {
		t.Errorf("expected version 1.2.3, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "namespace: production") {
		t.Errorf("expected namespace production, got:\n%s", yaml)
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

func TestDestroyCommand_HasFlags(t *testing.T) {
	flags := destroyCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"cluster", ""},
		{"force", "false"},
		{"cleanup", "false"},
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

func TestStatusCommand_HasFlags(t *testing.T) {
	flags := statusCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"cluster", ""},
		{"watch", "false"},
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

func TestValidateCommand_HasFlags(t *testing.T) {
	flags := validateCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"cluster", ""},
		{"quick", "false"},
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

func TestGenerateGitRepositoryYAML(t *testing.T) {
	yaml := generateGitRepositoryYAML("https://github.com/user/repo.git", "main")

	expected := []string{
		"kind: GitRepository",
		"namespace: flux-system",
		"branch: main",
		"url: https://github.com/user/repo.git",
		"apiVersion: source.toolkit.fluxcd.io/v1",
	}
	for _, s := range expected {
		if !strings.Contains(yaml, s) {
			t.Errorf("expected YAML to contain %q, got:\n%s", s, yaml)
		}
	}
}

func TestGenerateKustomizationYAML_NoDependency(t *testing.T) {
	yaml := generateKustomizationYAML("infrastructure", "clusters/production/infrastructure", "")

	expected := []string{
		"kind: Kustomization",
		"name: infrastructure",
		"namespace: flux-system",
		"path: ./clusters/production/infrastructure",
		"apiVersion: kustomize.toolkit.fluxcd.io/v1",
		"prune: true",
	}
	for _, s := range expected {
		if !strings.Contains(yaml, s) {
			t.Errorf("expected YAML to contain %q, got:\n%s", s, yaml)
		}
	}
	if strings.Contains(yaml, "dependsOn") {
		t.Errorf("expected no dependsOn block, got:\n%s", yaml)
	}
}

func TestGenerateKustomizationYAML_WithDependency(t *testing.T) {
	yaml := generateKustomizationYAML("apps", "clusters/production/apps", "infrastructure")

	if !strings.Contains(yaml, "dependsOn") {
		t.Errorf("expected dependsOn block, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "name: infrastructure") {
		t.Errorf("expected dependency on infrastructure, got:\n%s", yaml)
	}
}

func TestKubeconfigCommand_HasFlags(t *testing.T) {
	flags := kubeconfigCmd.Flags()

	cases := []struct {
		name     string
		defValue string
	}{
		{"cluster", ""},
		{"output", "./kubeconfig"},
		{"merge", "false"},
		{"set-context", "false"},
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
