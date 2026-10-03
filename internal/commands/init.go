package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
)

const excludeLine = "/" + config.FileName

// errStop ends the init pipeline early without failing the command.
var errStop = errors.New("stop")

type initOpts struct {
	baseDir  string
	force    bool
	defaults bool
	exclude  bool
}

// initRun is the state threaded through the init steps.
type initRun struct {
	d     Deps
	opts  initOpts
	git   gitx.Git
	main  string
	cfg   config.Config
	stdin *bufio.Reader // one reader for every prompt, so buffered input is not lost
}

// initStep is one stage of init; --flow will add its own step.
type initStep func(context.Context, *initRun) error

// initSteps is the init pipeline, in order.
var initSteps = []initStep{checkNotInitialised, chooseBaseDir, writeConfig, offerExclude}

func newInitCmd(d Deps) *cobra.Command {
	var o initOpts
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write .flowtree.json for this repository",
		Long: "Writes .flowtree.json in the main worktree.\n\n" +
			"If the file already exists, init prints a note and changes nothing (exit 0); use --force to overwrite it.\n" +
			"base_dir defaults to $FLOWTREE_PARENT/<repo name> when that is set, else ../worktrees.\n" +
			"On a terminal, init asks for base_dir and whether to add the file to .git/info/exclude (default: no).\n" +
			"With -d it never asks and never excludes; --exclude excludes without asking.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runInit(cmd.Context(), d, o) },
	}
	f := cmd.Flags()
	f.StringVarP(&o.baseDir, "base-dir", "w", "", "directory worktrees are created under")
	f.BoolVarP(&o.force, "force", "f", false, "overwrite an existing .flowtree.json")
	f.BoolVarP(&o.defaults, "defaults", "d", false, "do not prompt; use defaults")
	f.BoolVar(&o.exclude, "exclude", false, "add .flowtree.json to .git/info/exclude without asking")
	return cmd
}

func runInit(ctx context.Context, d Deps, o initOpts) error {
	cwd, err := d.Cwd()
	if err != nil {
		return err
	}
	here := gitx.Bind(ctx, d.Run, cwd)
	main, err := gitx.MainWorktree(here)
	if err != nil || main.Path == "" {
		return errors.New("not in a git repository")
	}
	run := &initRun{
		d: d, opts: o, main: main.Path, cfg: config.Defaults(),
		git:   gitx.Bind(ctx, d.Run, main.Path),
		stdin: bufio.NewReader(d.Stdin),
	}
	return runSteps(ctx, run, initSteps)
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

func checkNotInitialised(_ context.Context, r *initRun) error {
	if _, err := os.Stat(config.Path(r.main)); err != nil || r.opts.force {
		return nil
	}
	r.d.Printf("%s already exists in %s; nothing changed (use --force to overwrite)\n", config.FileName, r.main)
	return errStop
}

func chooseBaseDir(_ context.Context, r *initRun) error {
	def := config.DefaultBaseDirFor(r.main, r.d.Getenv)
	dir := r.opts.baseDir
	if dir == "" && r.interactive() {
		dir = ask(r.stdin, r.d.Stderr, "Worktree directory", def)
	}
	if dir == "" {
		dir = def
	}
	r.cfg.BaseDir = config.ExpandHome(dir)
	return nil
}

func writeConfig(_ context.Context, r *initRun) error {
	if err := config.Save(r.main, r.cfg); err != nil {
		return err
	}
	r.d.Printf("Created %s (base_dir: %s)\n", config.Path(r.main), r.cfg.BaseDir)
	return nil
}

func offerExclude(_ context.Context, r *initRun) error {
	want := r.opts.exclude
	if !want && r.interactive() {
		want = askYesNo(r.stdin, r.d.Stderr, "Add "+config.FileName+" to .git/info/exclude?")
	}
	if !want {
		return nil
	}
	return excludeConfig(r)
}

// excludeConfig appends the config file to info/exclude unless git already ignores or tracks it.
func excludeConfig(r *initRun) error {
	switch {
	case r.git.Ok("ls-files", "--error-unmatch", config.FileName):
		r.d.Printf("Note: %s is tracked by git; not excluding it\n", config.FileName)
		return nil
	case r.git.Ok("check-ignore", "-q", config.FileName):
		r.d.Printf("Note: %s is already ignored by git\n", config.FileName)
		return nil
	}
	path, err := excludePath(r.git, r.main)
	if err != nil {
		return err
	}
	if err := updateFile(path, func(s string) (string, bool) { return withLine(s, excludeLine) }); err != nil {
		return err
	}
	r.d.Printf("Added %s to %s\n", excludeLine, path)
	return nil
}

func excludePath(g gitx.Git, main string) (string, error) {
	common, err := g("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(main, common)
	}
	return filepath.Join(common, "info", "exclude"), nil
}

// withLine returns content with line appended (own line) and whether it changed.
func withLine(content, line string) (string, bool) {
	if hasLine(strings.Split(content, "\n"), line) {
		return content, false
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + line + "\n", true
}

func hasLine(lines []string, line string) bool {
	if len(lines) == 0 {
		return false
	}
	return strings.TrimSpace(lines[0]) == line || hasLine(lines[1:], line)
}

// updateFile rewrites path (created if missing) when f reports a change.
func updateFile(path string, f func(string) (string, bool)) error {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next, changed := f(string(old))
	if !changed {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(next), 0o644)
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
