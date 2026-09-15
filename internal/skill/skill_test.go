package skill

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sample = "---\nname: efficient-codebase-navigation\n---\n\n# Guide\n"

func isolateAgentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func TestInstallGlobalWritesAgentsAndExistingClaude(t *testing.T) {
	isolateAgentEnv(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := Install(Options{Content: sample, Scope: ScopeGlobal, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	wantAgents := filepath.Join(home, ".agents", "skills", Name, "SKILL.md")
	wantClaude := filepath.Join(home, ".claude", "skills", Name, "SKILL.md")
	if len(paths) != 2 || paths[0] != wantAgents || paths[1] != wantClaude {
		t.Fatalf("paths = %#v, want [%s %s]", paths, wantAgents, wantClaude)
	}
	if got := readFile(t, wantAgents); got != sample {
		t.Errorf("agents manifest = %q, want %q", got, sample)
	}
	if got := readFile(t, wantClaude); got != sample {
		t.Errorf("claude manifest = %q, want %q", got, sample)
	}
}

func TestInstallGlobalSkipsMissingClaudeDirectoryByDefault(t *testing.T) {
	isolateAgentEnv(t)
	home := t.TempDir()

	paths, err := Install(Options{Content: sample, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %#v, want only the agents manifest", paths)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("default install created %s/.claude", home)
	}
}

func TestInstallExplicitClaudeTargetCreatesDirectory(t *testing.T) {
	isolateAgentEnv(t)
	home := t.TempDir()

	paths, err := Install(Options{Content: sample, Scope: ScopeGlobal, Target: TargetClaude, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(home, ".claude", "skills", Name, "SKILL.md")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %#v, want [%s]", paths, want)
	}
}

func TestInstallHonorsClaudeConfigDir(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "claude-alt")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()

	paths, err := Install(Options{Content: sample, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(configDir, "skills", Name, "SKILL.md")
	if len(paths) != 2 || paths[1] != want {
		t.Fatalf("paths = %#v, want claude manifest at %s", paths, want)
	}
}

func TestInstallProjectUsesWorkingDirectory(t *testing.T) {
	isolateAgentEnv(t)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := Install(Options{Content: sample, Scope: ScopeProject, WorkingDir: project})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	wantAgents := filepath.Join(project, ".agents", "skills", Name, "SKILL.md")
	wantClaude := filepath.Join(project, ".claude", "skills", Name, "SKILL.md")
	if len(paths) != 2 || paths[0] != wantAgents || paths[1] != wantClaude {
		t.Fatalf("paths = %#v, want project manifests", paths)
	}
}

func TestInstallDirOverridesScopeAndTarget(t *testing.T) {
	isolateAgentEnv(t)
	dir := filepath.Join(t.TempDir(), "custom-skills")
	home := t.TempDir()

	paths, err := Install(Options{Content: sample, Dir: dir, HomeDir: home, Target: TargetClaude})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(dir, Name, "SKILL.md")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %#v, want [%s]", paths, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("Dir override also wrote into the home directory")
	}
}

func TestInstallDedupesSymlinkedClaudeRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated privileges on Windows")
	}
	isolateAgentEnv(t)
	home := t.TempDir()
	agents := filepath.Join(home, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(agents, filepath.Join(home, ".claude")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	paths, err := Install(Options{Content: sample, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %#v, want one physical manifest", paths)
	}
}

func TestInstallGlobalWritesExistingCodexDirectory(t *testing.T) {
	isolateAgentEnv(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := Install(Options{Content: sample, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(home, ".codex", "skills", Name, "SKILL.md")
	found := false
	for _, path := range paths {
		if path == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("paths = %#v, want codex manifest at %s", paths, want)
	}
}

func TestInstallHonorsCodexHome(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "codex-alt")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", codexHome)
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()

	paths, err := Install(Options{Content: sample, Target: TargetCodex, HomeDir: home})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(codexHome, "skills", Name, "SKILL.md")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %#v, want [%s]", paths, want)
	}
}

func TestInstallProjectCodexUsesSharedAgentsDirectory(t *testing.T) {
	isolateAgentEnv(t)
	project := t.TempDir()

	paths, err := Install(Options{Content: sample, Scope: ScopeProject, Target: TargetCodex, WorkingDir: project})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(project, ".agents", "skills", Name, "SKILL.md")
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %#v, want [%s]", paths, want)
	}
}

func TestInstallRejectsInvalidInput(t *testing.T) {
	isolateAgentEnv(t)
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "empty content", opts: Options{HomeDir: t.TempDir()}, want: "empty"},
		{name: "missing home", opts: Options{Content: sample}, want: "home directory"},
		{name: "missing working directory", opts: Options{Content: sample, Scope: ScopeProject}, want: "current directory"},
		{name: "invalid scope", opts: Options{Content: sample, Scope: "universe", HomeDir: t.TempDir()}, want: "invalid scope"},
		{name: "invalid target", opts: Options{Content: sample, Target: "vim", HomeDir: t.TempDir()}, want: "invalid target"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Install(test.opts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Install error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestInstallAddsTrailingNewlineAndOverwrites(t *testing.T) {
	isolateAgentEnv(t)
	home := t.TempDir()

	if _, err := Install(Options{Content: "# Guide", Target: TargetAgents, HomeDir: home}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := Install(Options{Content: "# Updated", Target: TargetAgents, HomeDir: home}); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	path := filepath.Join(home, ".agents", "skills", Name, "SKILL.md")
	if got := readFile(t, path); got != "# Updated\n" {
		t.Fatalf("manifest = %q, want %q", got, "# Updated\n")
	}
}

func TestValidTarget(t *testing.T) {
	for _, value := range []string{"all", "agents", "claude", "codex"} {
		if !ValidTarget(value) {
			t.Errorf("ValidTarget(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "vim", "All"} {
		if ValidTarget(value) {
			t.Errorf("ValidTarget(%q) = true, want false", value)
		}
	}
}
