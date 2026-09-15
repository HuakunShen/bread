// Package bread carries repository-level assets that the bread CLI embeds.
//
// The canonical agent skill lives at
// skills/efficient-codebase-navigation/SKILL.md so the Skills CLI can discover
// it, while the Go toolchain embeds the same file here. Keeping one source of
// truth means `bread skill` always prints and installs the reviewed guide.
package bread

import _ "embed"

// NavigationSkill is the canonical agent skill shipped with the bread CLI.
//
//go:embed skills/efficient-codebase-navigation/SKILL.md
var NavigationSkill string
