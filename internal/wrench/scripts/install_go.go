package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "install-go",
		Args:        []string{"force"},
		Description: "ensure that wrench's desired Go version is installed",
		Execute: func(args ...string) error {
			wantGoVersion := "1.22.2"

			force := len(args) == 1 && args[0] == "true"
			goVersion, err := Output(os.Stderr, "go version")
			if err == nil && strings.Contains(goVersion, wantGoVersion) && !force {
				fmt.Fprintf(os.Stderr, wantGoVersion+" already installed")
				return nil
			}

			pathToGo, err := exec.LookPath("go")
			if err == nil {
				pathToGo, _ = filepath.Abs(pathToGo)
			}

			// Download the Go archive
			extension := "tar.gz"
			exeExt := ""
			if runtime.GOOS == "windows" {
				extension = "zip"
				exeExt = ".exe"
			}
			url := fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.%s", wantGoVersion, runtime.GOOS, runtime.GOARCH, extension)
			archiveFilePath := "golang." + extension
			_ = os.RemoveAll(archiveFilePath)
			defer os.RemoveAll(archiveFilePath)
			err = DownloadFile(url, archiveFilePath)(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "DownloadFile")
			}

			// Remove existing install dir if it exists.
			_ = os.RemoveAll("go")

			goBinaryLocation, err := filepath.Abs("go/bin/go" + exeExt)
			if err != nil {
				return errors.Wrap(err, "Abs")
			}
			if pathToGo != goBinaryLocation {
				fmt.Fprintln(os.Stderr, "warning: existing Go installation may conflict:", pathToGo)
			}
			fmt.Fprintln(os.Stderr, "installing to:", goBinaryLocation)

			// Update system-wide env vars.
			err = EnsureOnPathPermanent(filepath.Dir(goBinaryLocation))
			if err != nil {
				return errors.Wrap(err, "EnsureOnPathPermanent")
			}

			// Extract the Go archive
			err = ExtractArchive(archiveFilePath, ".", 0)(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "ExtractArchive")
			}

			if err := ensureGoBinOnPath(); err != nil {
				return errors.Wrap(err, "ensureGoBinOnPath")
			}
			return nil
		},
	})
}

func ensureGoBinOnPath() error {
	goBin, err := Output(os.Stderr, "go env GOBIN")
	if err != nil {
		return err
	}
	goBin = strings.TrimSpace(goBin)
	if goBin == "" {
		goPath, err := Output(os.Stderr, "go env GOPATH")
		if err != nil {
			return err
		}
		goPath = strings.TrimSpace(goPath)
		goBin = filepath.Join(goPath, "bin")
	}
	return EnsureOnPathPermanent(goBin)
}
