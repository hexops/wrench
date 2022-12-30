package scripts

import (
	"os"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "mach-push-rewrite-zig-version",
		Args:        nil,
		Description: "wrench installs prerequisites (Go), rebuilds itself, and restarts the service",
		ExecuteResponse: func(args ...string) (*Response, error) {
			wantZigVersion, err := QueryLatestZigVersion()
			if err != nil {
				return nil, errors.Wrap(err, "QueryLatestZigVersion")
			}

			repos := []string{
				"github.com/hexops/mach",
				// "github.com/hexops/mach-examples",
			}
			pushed := []string{}
			workDir := "zig-rewrite-work"
			defer os.RemoveAll(workDir)
			for _, repoURL := range repos {
				_ = os.RemoveAll(workDir)
				if err := GitClone(os.Stderr, "zig-rewrite-work", repoURL); err != nil {
					return nil, errors.Wrap(err, "GitClone")
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

				err = GitCheckoutNewBranch(os.Stderr, workDir, "wrench/rewrite-zig-version")
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				err = GitCommit(os.Stderr, workDir, "all: update Zig to version "+wantZigVersion)
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				err = GitPush(os.Stderr, workDir, repoURL)
				if err != nil {
					return nil, errors.Wrap(err, "GitCommit")
				}
				pushed = append(pushed, repoURL)
			}
			return &Response{PushedRepos: pushed}, nil
		},
	})
}
