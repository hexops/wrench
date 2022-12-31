package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "mach-update-gpu-dawn",
		Args:        []string{},
		Description: "update mach/libs/gpu-dawn to the latest generated branch of hexops/dawn",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			workDir := "mach-update-gpu-dawn"
			dawnRepoDir := filepath.Join(workDir, "dawn")
			dawnRepoURL := "https://github.com/hexops/dawn"
			machRepoDir := filepath.Join(workDir, "mach")
			machRepoURL := "https://github.com/hexops/mach"
			updateDawn := true

			if updateDawn {
				timeNow := time.Now()
				date := timeNow.Format("2006-01-02")
				generatedBranch := fmt.Sprintf("generated-%s.%v", date, timeNow.Unix())
				if err := Exec("wrench script mach-update-dawn " + generatedBranch)(os.Stderr); err != nil {
					return nil, errors.Wrap(err, "mach-update-dawn")
				}
			}

			// Clone or update the repositories
			if err := GitCloneOrUpdateAndClean(os.Stderr, dawnRepoDir, dawnRepoURL); err != nil {
				return nil, errors.Wrap(err, "GitCloneOrUpdateAndClean")
			}
			if err := GitCloneOrUpdateAndClean(os.Stderr, machRepoDir, machRepoURL); err != nil {
				return nil, errors.Wrap(err, "GitCloneOrUpdateAndClean")
			}

			// Find the current version used by Mach
			re := regexp.MustCompile(`generated-\d{4}-\d{2}-\d{2}(\.\d*)?`)
			fileContents, err := os.ReadFile(filepath.Join(machRepoDir, "libs/gpu-dawn/sdk.zig"))
			if err != nil {
				return nil, errors.Wrap(err, "ReadFile")
			}
			currentVersion := re.FindString(string(fileContents))
			if currentVersion == "" {
				return nil, errors.New("failed to find current generated-yyyy-mm-dd[.unixstamp] in mach/libs/gpu-dawn/sdk.zig")
			}

			branches, err := GitBranches(os.Stderr, workDir)
			if err != nil {
				return nil, errors.Wrap(err, "GitBranches")
			}
			var latestBranchTime *time.Time
			latestBranch := ""
			for _, branch := range branches {
				if !strings.HasPrefix(branch, "generated-") {
					continue
				}
				// generated-yyyy-mm-dd OR generated-yyyy-mm-dd.unixtimestamp
				var t time.Time
				if strings.Contains(branch, ".") {
					unixStampStr := strings.Split(strings.TrimPrefix(branch, "generated-"), ".")[1]
					unixStamp, err := strconv.ParseInt(unixStampStr, 10, 64)
					if err != nil {
						return nil, errors.Wrap(err, "parsing unix timestamp at end of: "+branch)
					}

					t = time.Unix(unixStamp, 0)
				} else {
					t, err = time.Parse("2006-01-02", strings.TrimPrefix(branch, "generated-"))
					if err != nil {
						return nil, errors.Wrap(err, "parsing date in: "+branch)
					}
				}
				if latestBranchTime == nil || t.After(*latestBranchTime) {
					latestBranch = branch
					latestBranchTime = &t
				}
			}

			// Find and replace old -> new branch
			oldBranch := currentVersion
			newBranch := latestBranch
			if err := FindAndReplace(machRepoDir, []string{"**/*.zig", "**/*.md"}, func(name string, contents []byte) ([]byte, error) {
				contents = re.ReplaceAll(contents, []byte(newBranch))
				return contents, nil
			})(os.Stderr); err != nil {
				return nil, errors.Wrap(err, "FindAndReplace")
			}

			// Push changes if there are any
			changesExist, err := GitChangesExist(os.Stderr, workDir)
			if err != nil {
				return nil, errors.Wrap(err, "GitChangesExist")
			}
			if !changesExist {
				return &api.ScriptResponse{}, nil
			}

			err = GitCheckoutNewBranch(os.Stderr, workDir, os.Getenv("WRENCH_GIT_PUSH_BRANCH_NAME"))
			if err != nil {
				return nil, errors.Wrap(err, "GitCommit")
			}
			err = GitConfigureRepo(os.Stderr, workDir)
			if err != nil {
				return nil, errors.Wrap(err, "GitConfigureRepo")
			}
			err = GitCommit(os.Stderr, workDir, "gpu-dawn: update to latest version "+newBranch)
			if err != nil {
				return nil, errors.Wrap(err, "GitCommit")
			}
			forcePush := true
			err = GitPush(os.Stderr, workDir, machRepoURL, forcePush)
			if err != nil {
				return nil, errors.Wrap(err, "GitCommit")
			}

			dawnDiffGni, err := Output(os.Stderr, "git diff "+oldBranch+".."+newBranch+" -- *.gni", WorkDir(dawnRepoDir))
			if err != nil {
				return nil, errors.Wrap(err, "dawnDiffGni")
			}
			dawnDiffGn, err := Output(os.Stderr, "git diff "+oldBranch+".."+newBranch+" -- *.gn", WorkDir(dawnRepoDir))
			if err != nil {
				return nil, errors.Wrap(err, "dawnDiffGn")
			}
			dawnDiffBuild, err := Output(os.Stderr, "git diff "+oldBranch+".."+newBranch+" -- BUILD", WorkDir(dawnRepoDir))
			if err != nil {
				return nil, errors.Wrap(err, "dawnDiffBuild")
			}
			dawnDiffBuildAll := fmt.Sprintf("%s\n%s\n%s\n", dawnDiffGni, dawnDiffGn, dawnDiffBuild)
			webgpuDiffHeader, err := Output(os.Stderr, "git diff "+oldBranch+".."+newBranch+" -- out/Debug/gen/include/dawn/webgpu.h", WorkDir(dawnRepoDir))
			if err != nil {
				return nil, errors.Wrap(err, "webgpuDiffHeader")
			}
			webgpuDiffDawnJson, err := Output(os.Stderr, "git diff "+oldBranch+".."+newBranch+" -- dawn.json", WorkDir(dawnRepoDir))
			if err != nil {
				return nil, errors.Wrap(err, "webgpuDiffDawnJson")
			}

			return &api.ScriptResponse{
				PushedRepos: []string{machRepoURL},
				CustomLogs: map[string]string{
					"dawn-diff-build":  dawnDiffBuildAll,
					"dawn-diff-header": webgpuDiffHeader,
					"dawn-diff-json":   webgpuDiffDawnJson,
				},
				Metadata: map[string]string{
					"OldBranch": oldBranch,
					"NewBranch": newBranch,
				},
			}, nil
		},
	})
}
