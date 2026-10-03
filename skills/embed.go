// Package skills holds the agent skills shipped inside the flowtree binary.
// `flowtree install-skill <dir>` writes them out.
package skills

import "embed"

// FS contains one directory per skill, each with a SKILL.md at its root.
//
//go:embed all:flowtree
var FS embed.FS
