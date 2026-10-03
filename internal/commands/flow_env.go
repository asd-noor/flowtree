package commands

import (
	"context"
	"path/filepath"

	"flowtree/internal/flow"
)

// flowEnv builds the environment flow operations run in.
func (d Deps) flowEnv(ctx context.Context, repo Repo, s flow.Settings) *flow.Env {
	cwd := repo.Cwd
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	return &flow.Env{
		Ctx: ctx, Run: d.Run, S: s, Config: repo.Config, Main: repo.Main, Cwd: cwd,
		Out: d.Stdout, Err: d.Stderr,
		CdTo: func(path string) { writeCdFile(d, path) },
	}
}

// OpenFlow locates the repository and its flow settings; it fails unless `init --flow` has run.
func (d Deps) OpenFlow(ctx context.Context) (*flow.Env, error) {
	repo, err := d.OpenRepo(ctx)
	if err != nil {
		return nil, err
	}
	s, err := flow.FromConfig(repo.Config)
	if err != nil {
		return nil, err
	}
	return d.flowEnv(ctx, repo, s), nil
}
