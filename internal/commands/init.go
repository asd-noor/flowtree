package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
)

// errStop ends the init pipeline early without failing the command.
var errStop = errors.New("stop")

type initOpts struct {
	baseDir  string
	force    bool
	defaults bool
	flow     flowOpts
}

// initRun is the state threaded through the init steps.
type initRun struct {
	d      Deps
	opts   initOpts
	git    gitx.Git
	main   string
	cfg    config.Config   // the configuration before init
	pairs  []config.Pair   // settings init will store
	stdin  *bufio.Reader   // one reader for every prompt, so buffered input is not lost
	kept   bool            // existing settings are being extended, not replaced
	flowS  *flowSettingsIn // the flow settings chosen, when --flow is given
	reset  bool            // --force removed the repository's settings; undone if init fails
	backup []config.Pair   // the repository's own settings before the reset
}

// initStep is one stage of init.
type initStep func(context.Context, *initRun) error

// initSteps is the init pipeline, in order.
var initSteps = []initStep{loadSettings, checkNotInitialised, chooseBaseDir, chooseFlowSettings, writeSettings, setupFlowBranches}

func newInitCmd(d Deps) *cobra.Command {
	var o initOpts
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure flowtree for this repository",
		Long: "Stores flowtree's settings in the repository's git config (section flowtree, in .git/config), so nothing\n" +
			"is added to the working tree. Plain init only sets the worktree directory (base dir): -w, or the answer to the\n" +
			"prompt on a terminal. When neither differs from the default (../worktrees, or $FLOWTREE_PARENT/<repo name>\n" +
			"when FLOWTREE_PARENT is defined) nothing is stored, so the default keeps following the environment.\n\n" +
			"If flowtree is already configured, init prints a note and changes nothing (exit 0); --force resets the\n" +
			"repository's flowtree settings first.\n\n" +
			"With --flow it also sets up git-flow: it stores the branch names, prefixes and tag prefix, creates the\n" +
			"master and develop branches if they are missing, and puts the main worktree on develop. On a repository\n" +
			"that is configured without flow, --flow adds the flow settings and keeps the rest. With -d init never asks.\n" +
			"Use 'flowtree config' to see or change single settings later.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runInit(cmd.Context(), d, o) },
	}
	f := cmd.Flags()
	f.StringVarP(&o.baseDir, "base-dir", "w", "", "directory worktrees are created under")
	f.BoolVarP(&o.force, "force", "f", false, "reset the repository's flowtree settings first")
	f.BoolVarP(&o.defaults, "defaults", "d", false, "do not prompt; use defaults")
	bindFlowFlags(f, &o.flow)
	return cmd
}

func runInit(ctx context.Context, d Deps, o initOpts) error {
	cwd, err := d.Cwd()
	if err != nil {
		return err
	}
	main, err := gitx.MainWorktree(gitx.Bind(ctx, d.Run, cwd))
	if err != nil || main.Path == "" {
		return errors.New("not in a git repository")
	}
	run := &initRun{
		d: d, opts: o, main: main.Path,
		git:   gitx.Bind(ctx, d.Run, main.Path),
		stdin: bufio.NewReader(d.Stdin),
	}
	err = runSteps(ctx, run, initSteps)
	if err != nil {
		run.restore()
	}
	return err
}

// runSteps runs steps in order, stopping at the first error; errStop is a clean stop.
func runSteps(ctx context.Context, r *initRun, steps []initStep) error {
	if len(steps) == 0 {
		return nil
	}
	if err := steps[0](ctx, r); err != nil {
		if errors.Is(err, errStop) {
			return nil
		}
		return err
	}
	return runSteps(ctx, r, steps[1:])
}

func (r *initRun) interactive() bool { return r.d.IsTTY() && !r.opts.defaults }

// loadSettings reads what is configured now. With --force it first resets the repository's own flowtree
// settings, so that everything after it (defaults, the prompt, the remote, whether flow is set up) is computed
// against the reset state; global and system settings stay. The old settings are remembered so a failing
// init can put them back. A broken configuration does not block the reset.
func loadSettings(_ context.Context, r *initRun) error {
	if r.opts.force {
		r.resetLocal()
	}
	cfg, err := config.Load(r.git, r.main, r.d.Getenv)
	if err != nil && !r.opts.force {
		return err
	}
	if err != nil {
		cfg = config.Defaults()
		cfg.Parent = r.d.Getenv(config.ParentEnv)
	}
	r.cfg = cfg
	return nil
}

// resetLocal removes the repository's flowtree section, remembering what was in it.
func (r *initRun) resetLocal() {
	r.backup = localPairs(r.git)
	r.reset = true
	_, _ = r.git("config", "--local", "--remove-section", "flowtree") // fails when there is none
}

// restore undoes resetLocal after a failed init: the repository keeps the settings it had.
func (r *initRun) restore() {
	if !r.reset {
		return
	}
	_, _ = r.git("config", "--local", "--remove-section", "flowtree")
	_ = config.SetAll(r.git, config.Local, r.backup)
}

// localPairs lists the repository's own flowtree settings; unreadable ones (a broken configuration) are not kept.
func localPairs(g gitx.Git) []config.Pair {
	entries, err := config.Entries(g)
	if err != nil {
		return nil
	}
	var pairs []config.Pair
	for _, e := range entries {
		if e.Scope == "local" {
			pairs = append(pairs, config.Pair{Key: e.Key, Value: e.Value})
		}
	}
	return pairs
}

// configured reports whether this repository has local flowtree settings.
func (r *initRun) configured() bool {
	entries, err := config.Entries(r.git)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if e.Scope == "local" {
			return true
		}
	}
	return false
}

func checkNotInitialised(_ context.Context, r *initRun) error {
	if r.opts.force || !r.configured() {
		return nil
	}
	if r.opts.flow.enabled && !r.cfg.FlowEnabled() {
		r.kept = true
		return nil
	}
	if r.opts.flow.enabled {
		r.d.Printf("flowtree flow is already set up in %s; nothing changed (use --force to reset)\n", r.main)
	} else {
		r.d.Printf("flowtree is already configured in %s; nothing changed (use --force to reset)\n", r.main)
	}
	return errStop
}

// chooseBaseDir stores a base dir only when it differs from the default, so an unset base dir keeps
// following $FLOWTREE_PARENT.
func chooseBaseDir(_ context.Context, r *initRun) error {
	if r.kept {
		return nil
	}
	def := r.cfg.ResolveBaseDir(r.main) // shown as an absolute path: "../worktrees" says little in a prompt
	dir := r.opts.baseDir
	if dir == "" && r.interactive() {
		dir = ask(r.stdin, r.d.Stderr, "Worktree directory", def)
	}
	if dir != "" && (config.Config{BaseDir: dir}).ResolveBaseDir(r.main) != def {
		r.pairs = append(r.pairs, config.Pair{Key: "baseDir", Value: config.ExpandHome(dir)})
	}
	return nil
}

func writeSettings(_ context.Context, r *initRun) error {
	if err := config.SetAll(r.git, config.Local, r.pairs); err != nil {
		return err
	}
	cfg, err := config.Load(r.git, r.main, r.d.Getenv)
	if err != nil {
		return err
	}
	r.cfg = cfg
	r.report()
	return nil
}

func (r *initRun) report() {
	if len(r.pairs) == 0 {
		r.d.Printf("Nothing to store: the defaults apply (worktrees in %s)\n", r.cfg.ResolveBaseDir(r.main))
		return
	}
	r.d.Printf("Stored in the git config of %s:\n", r.main)
	for _, p := range r.pairs {
		r.d.Printf("  flowtree.%s = %s\n", p.Key, p.Value)
	}
	r.d.Printf("Worktrees go to %s\n", r.cfg.ResolveBaseDir(r.main))
}

// ask prompts for a line; an empty answer or EOF yields def.
func ask(in *bufio.Reader, out io.Writer, prompt, def string) string {
	_, _ = fmt.Fprintf(out, "%s [%s]: ", prompt, def)
	line, _ := in.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

// askYesNo prompts [y/N]; anything but y/yes, including EOF, is no.
func askYesNo(in *bufio.Reader, out io.Writer, prompt string) bool {
	_, _ = fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, _ := in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
