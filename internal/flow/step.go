package flow

import "strings"

// Step is one effect of a command. Commands are written as a plan (a checked list of steps)
// that Run executes, or Describe prints for --dry-run.
type Step struct {
	Desc string
	Do   func(*Env) error
}

// Run executes the steps in order and stops at the first error.
func Run(e *Env, steps []Step) error {
	if len(steps) == 0 {
		return nil
	}
	if err := steps[0].Do(e); err != nil {
		return err
	}
	return Run(e, steps[1:])
}

// Describe renders the steps for a dry run, one per line.
func Describe(steps []Step) string {
	lines := make([]string, len(steps))
	for i, s := range steps {
		lines[i] = "  " + s.Desc
	}
	return strings.Join(lines, "\n")
}

// If includes the steps only when cond holds.
func If(cond bool, steps []Step) []Step {
	if !cond {
		return nil
	}
	return steps
}

// Plan concatenates step groups, so a plan reads as a list of named parts.
func Plan(groups ...[]Step) []Step {
	var out []Step
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// One wraps a single step as a group.
func One(desc string, do func(*Env) error) []Step { return []Step{{Desc: desc, Do: do}} }
