// Package skill installs the bread agent skill into the skills directories
// that agent hosts read.
//
// The universal location is <root>/.agents/skills, which Cline and other
// agents share. Claude Code reads <claude-config>/skills, where the
// configuration directory is $CLAUDE_CONFIG_DIR when set and ~/.claude
// otherwise. Codex reads $CODEX_HOME/skills globally (~/.codex/skills by
// default) and the shared .agents directory inside a project. Global
// installation (the default) resolves those roots against the user profile;
// project installation resolves them against the working directory.
package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Name is the directory name of the bread navigation skill.
const Name = "efficient-codebase-navigation"

// manifestFile is the file every skill directory must contain.
const manifestFile = "SKILL.md"

// Scope selects the base directory an installation is rooted at.
type Scope string

const (
	// ScopeGlobal installs into the user profile. It is the default.
	ScopeGlobal Scope = "global"
	// ScopeProject installs into the current repository.
	ScopeProject Scope = "project"
)

// Target selects which agent skills directories receive the skill.
type Target string

const (
	// TargetAll installs into every available agent directory.
	TargetAll Target = "all"
	// TargetAgents installs only into the shared .agents directory.
	TargetAgents Target = "agents"
	// TargetClaude installs only into the Claude Code directory.
	TargetClaude Target = "claude"
	// TargetCodex installs only into the Codex directory.
	TargetCodex Target = "codex"
)

// Options controls an installation.
type Options struct {
	// Content is the skill manifest to write. It must not be empty.
	Content string
	// Scope is the installation base. The zero value means ScopeGlobal.
	Scope Scope
	// Target is the agent directory selection. The zero value means TargetAll.
	Target Target
	// HomeDir is the user profile used by ScopeGlobal.
	HomeDir string
	// WorkingDir is the repository root used by ScopeProject.
	WorkingDir string
	// Dir overrides the destination skills directory, which is the directory
	// that contains the skill directories. When set it wins over Scope and
	// Target.
	Dir string
}

// Install writes the skill manifest and returns the manifest paths that were
// written, in installation order.
func Install(opts Options) ([]string, error) {
	content := ensureTrailingNewline(opts.Content)
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("skill content is empty")
	}

	targets, err := resolveTargets(opts)
	if err != nil {
		return nil, err
	}

	var installed []string
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		if target.optional && !isDirectory(target.root) {
			continue
		}
		path := filepath.Join(target.skillsDir, Name, manifestFile)
		key := physicalPath(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := writeManifest(path, content); err != nil {
			return installed, err
		}
		installed = append(installed, path)
	}
	if len(installed) == 0 {
		return nil, errors.New("no agent skills directory was available")
	}
	return installed, nil
}

// ValidTarget reports whether value is a supported --target value.
func ValidTarget(value string) bool {
	switch Target(value) {
	case TargetAll, TargetAgents, TargetClaude, TargetCodex:
		return true
	default:
		return false
	}
}

// destination is one resolved skills directory.
type destination struct {
	// root is the agent configuration directory, used for existence checks.
	root string
	// skillsDir is the directory that contains skill directories.
	skillsDir string
	// optional marks a directory that is skipped when its root does not
	// already exist, so a default install does not create configuration
	// directories for agents the user does not have.
	optional bool
}

func resolveTargets(opts Options) ([]destination, error) {
	if opts.Dir != "" {
		return []destination{{
			root:      opts.Dir,
			skillsDir: opts.Dir,
		}}, nil
	}

	target := opts.Target
	if target == "" {
		target = TargetAll
	}
	if !ValidTarget(string(target)) {
		return nil, fmt.Errorf("invalid target %q (want all, agents, claude, or codex)", target)
	}

	scope := opts.Scope
	if scope == "" {
		scope = ScopeGlobal
	}

	var base string
	switch scope {
	case ScopeGlobal:
		if strings.TrimSpace(opts.HomeDir) == "" {
			return nil, errors.New("could not resolve the home directory for a global installation")
		}
		base = opts.HomeDir
	case ScopeProject:
		if strings.TrimSpace(opts.WorkingDir) == "" {
			return nil, errors.New("could not resolve the current directory for a project installation")
		}
		base = opts.WorkingDir
	default:
		return nil, fmt.Errorf("invalid scope %q (want global or project)", scope)
	}

	var targets []destination
	if target == TargetAll || target == TargetAgents {
		root := filepath.Join(base, ".agents")
		targets = append(targets, destination{
			root:      root,
			skillsDir: filepath.Join(root, "skills"),
		})
	}
	if target == TargetAll || target == TargetClaude {
		root := filepath.Join(base, ".claude")
		if scope == ScopeGlobal {
			if dir := claudeConfigDir(); dir != "" {
				root = dir
			}
		}
		targets = append(targets, destination{
			root:      root,
			skillsDir: filepath.Join(root, "skills"),
			// A default install only writes into an existing Claude Code
			// configuration; --target claude creates it explicitly.
			optional: target == TargetAll,
		})
	}
	if target == TargetAll || target == TargetCodex {
		// Codex keeps a dedicated global directory, but inside a project it
		// reads the shared .agents directory.
		root := filepath.Join(base, ".agents")
		if scope == ScopeGlobal {
			root = filepath.Join(base, ".codex")
			if dir := codexHome(); dir != "" {
				root = dir
			}
		}
		targets = append(targets, destination{
			root:      root,
			skillsDir: filepath.Join(root, "skills"),
			// A default install only writes into an existing Codex
			// configuration; --target codex creates it explicitly.
			optional: target == TargetAll,
		})
	}
	return targets, nil
}

// claudeConfigDir returns the configured Claude Code directory, or an empty
// string when the environment does not override it.
func claudeConfigDir() string {
	return strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
}

// codexHome returns the configured Codex directory, or an empty string when
// the environment does not override it.
func codexHome() string {
	return strings.TrimSpace(os.Getenv("CODEX_HOME"))
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func writeManifest(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// physicalPath resolves symlinked directories so an install that lands in the
// same physical file through two aliases is only written once. The target file
// may not exist yet, so the nearest existing ancestor is resolved and the
// remaining components are appended.
func physicalPath(path string) string {
	dir := filepath.Dir(path)
	rest := []string{filepath.Base(path)}
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
		dir = parent
	}
}

func ensureTrailingNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}
