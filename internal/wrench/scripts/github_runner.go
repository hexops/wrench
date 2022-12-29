package scripts

import (
	"fmt"
	"os"
	"runtime"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "github-runner",
		Args:        []string{"force"},
		Description: "begin a github action runner",
		Execute: func(args ...string) error {
			force := len(args) == 1 && args[0] == "true"

			extension := "tar.gz"
			scriptExt := ".sh"
			if runtime.GOOS == "windows" {
				extension = "zip"
				scriptExt = ".cmd"
			}

			_, err := os.Stat("github-runner")
			if os.IsNotExist(err) || force {
				// Download the archive
				url := fmt.Sprintf("https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-%s-%s-2.299.1.%s", githubOS(), githubArch(), extension)
				archiveFilePath := "github-runner." + extension
				_ = os.RemoveAll(archiveFilePath)
				defer os.RemoveAll(archiveFilePath)
				err = DownloadFile(url, archiveFilePath)(os.Stderr)
				if err != nil {
					return errors.Wrap(err, "DownloadFile")
				}

				// Remove existing install dir if it exists.
				_ = os.RemoveAll("github-runner")
				fmt.Fprintln(os.Stderr, "installing to: github-runner/")

				// Extract the archive
				err = ExtractArchive(archiveFilePath, "github-runner")(os.Stderr)
				if err != nil {
					return errors.Wrap(err, "ExtractArchive")
				}

				secretRunnerURL := os.Getenv("WRENCH_SECRET_GITHUB_RUNNER_URL")
				secretRunnerToken := os.Getenv("WRENCH_SECRET_GITHUB_RUNNER_TOKEN")
				if secretRunnerURL == "" {
					_ = os.RemoveAll("github-runner")
					return errors.New("no WRENCH_SECRET_GITHUB_RUNNER_URL found")
				}
				if secretRunnerToken == "" {
					_ = os.RemoveAll("github-runner")
					return errors.New("no WRENCH_SECRET_GITHUB_RUNNER_TOKEN found")
				}
				wrenchRunnerID := os.Getenv("WRENCH_RUNNER_ID")

				if err := os.Setenv("RUNNER_ALLOW_RUNASROOT", "1"); err != nil {
					return errors.Wrap(err, "Setenv")
				}
				err = Exec(
					fmt.Sprintf("./config%s --unattended --name %s --replace --url %s --token %s", scriptExt, wrenchRunnerID, secretRunnerURL, secretRunnerToken),
					WorkDir("github-runner"),
				)(os.Stderr)
				if err != nil {
					_ = os.RemoveAll("github-runner")
					return errors.Wrap(err, "./config"+scriptExt)
				}
			}

			if err := os.Setenv("RUNNER_ALLOW_RUNASROOT", "1"); err != nil {
				return errors.Wrap(err, "Setenv")
			}
			err = Exec("./run"+scriptExt, WorkDir("github-runner"))(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "./run"+scriptExt+" --unattended")
			}
			return nil
		},
	})
}

func githubArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		panic("unsupported GOARCH: " + runtime.GOARCH)
	}
}

func githubOS() string {
	switch runtime.GOOS {
	case "windows":
		return "win"
	case "linux":
		return "linux"
	case "darwin":
		return "osx"
	default:
		panic("unsupported GOOS: " + runtime.GOOS)
	}
}
