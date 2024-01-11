package scripts

import (
	"os"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "mach-push-rewrite-zig-version",
		Args:        nil,
		Description: "wrench installs prerequisites (Go), rebuilds itself, and restarts the service",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			wantZigVersion, err := QueryZigVersion("mach-latest")
			if err != nil {
				return nil, errors.Wrap(err, "QueryZigVersion")
			}

			pushed := []string{}
			workDir := "zig-rewrite-work"
			defer os.RemoveAll(workDir)
			for _, repo := range AllRepos {
				if repo.Name != "hexops/machengine.org" && repo.CI != Zig {
					continue
				}
				repoURL := "github.com/" + repo.Name
				_ = os.RemoveAll(workDir)
				if err := GitClone(os.Stderr, workDir, repoURL); err != nil {
					return nil, errors.Wrap(err, "GitClone")
				}
				err = GitConfigureRepo(os.Stderr, workDir)
				if err != nil {
					return nil, errors.Wrap(err, "GitConfigureRepo")
				}

				err := Exec("wrench script rewrite-zig-version "+wantZigVersion, WorkDir(workDir))(os.Stderr)
				if err != nil {
					return nil, errors.Wrap(err, "rewrite-zig-version")
				}

				changesExist, err := GitChangesExist(os.Stderr, workDir)
				if err != nil {
					return nil, errors.Wrap(err, "GitChangesExist")
				}
				if !changesExist {
					continue
				}

				err = GitCheckoutNewBranch(os.Stderr, workDir, os.Getenv("WRENCH_GIT_PUSH_BRANCH_NAME"))
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				err = GitCommit(os.Stderr, workDir, "all: update Zig to version "+wantZigVersion)
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				forcePush := true
				err = GitPush(os.Stderr, workDir, repoURL, forcePush)
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				pushed = append(pushed, repoURL)
			}
			return &api.ScriptResponse{PushedRepos: pushed}, nil
		},
	})
}
