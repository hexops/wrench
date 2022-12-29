package scripts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "install-zig",
		Args:        []string{"force"},
		Description: "ensure that wrench's desired Zig version is installed",
		Execute: func(args ...string) error {
			force := len(args) == 1 && args[0] == "true"

			wantZigVersion, err := QueryLatestZigVersion()
			if err != nil {
				return errors.Wrap(err, "QueryLatestZigVersion")
			}

			zigVersion, err := Output(os.Stderr, "zig version")
			if err == nil && zigVersion != wantZigVersion && !force {
				fmt.Fprintf(os.Stderr, wantZigVersion+" already installed")
				return nil
			}

			// Download the Zig archive
			extension := "tar.xz"
			exeExt := ""
			if runtime.GOOS == "windows" {
				extension = "zip"
				exeExt = ".exe"
			}
			url := fmt.Sprintf("https://ziglang.org/builds/zig-%s-%s-%s.%s", zigOS(), zigArch(), wantZigVersion, extension)
			archiveFilePath := "zig." + extension
			_ = os.RemoveAll(archiveFilePath)
			defer os.RemoveAll(archiveFilePath)
			err = DownloadFile(url, archiveFilePath)(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "DownloadFile")
			}

			pathToZig, err := exec.LookPath("zig")
			if err == nil {
				pathToZig, _ = filepath.Abs(pathToZig)
			}

			// Remove existing install dir if it exists.
			_ = os.RemoveAll("zig")

			zigBinaryLocation, err := filepath.Abs("zig/zig" + exeExt)
			if err != nil {
				return errors.Wrap(err, "Abs")
			}
			if pathToZig != zigBinaryLocation {
				fmt.Fprintln(os.Stderr, "warning: existing Zig installation may conflict:", pathToZig)
			}
			fmt.Fprintln(os.Stderr, "installing to:", zigBinaryLocation)

			// Update system-wide env vars.
			err = EnsureOnPathPermanent(filepath.Dir(zigBinaryLocation))
			if err != nil {
				return errors.Wrap(err, "EnsureOnPathPermanent")
			}

			// Extract the Go archive
			err = ExtractArchive(archiveFilePath, "zig", 1)(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "ExtractArchive")
			}
			return nil
		},
	})
}

func QueryLatestZigVersion() (string, error) {
	resp, err := http.Get("https://ziglang.org/download/index.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var v *struct {
		Master struct {
			Version string
		}
	} = nil
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.Master.Version, nil
}

func zigArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		panic("unsupported GOARCH: " + runtime.GOARCH)
	}
}

func zigOS() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	case "darwin":
		return "macos"
	default:
		panic("unsupported GOOS: " + runtime.GOOS)
	}
}
