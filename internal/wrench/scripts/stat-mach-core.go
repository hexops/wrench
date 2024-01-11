package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "stat-mach-core",
		Args:        nil,
		Description: "Build and collect stats about mach-core",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			tmpDir := "stat-mach-core-tmp"
			repoDir := filepath.Join(tmpDir, "mach-core")

			_ = os.RemoveAll(tmpDir)
			if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
				return nil, err
			}

			step := time.Now()
			zigVersion, err := QueryZigVersion("mach-latest")
			if err != nil {
				return nil, errors.Wrap(err, "QueryZigVersion")
			}
			durationQueryZigVersion := time.Since(step)
			step = time.Now()

			// Download the Zig archive
			extension := "tar.xz"
			exeExt := ""
			stripPathComponents := 1
			if runtime.GOOS == "windows" {
				extension = "zip"
				exeExt = ".exe"
				stripPathComponents = 0
			}
			url := fmt.Sprintf("https://pkg.machengine.org/zig/zig-%s-%s-%s.%s", zigOS(), zigArch(), zigVersion, extension)
			archiveFilePath := filepath.Join(tmpDir, "zig."+extension)
			_ = os.RemoveAll(archiveFilePath)
			defer os.RemoveAll(archiveFilePath)
			err = DownloadFile(url, archiveFilePath)(os.Stderr)
			if err != nil {
				return nil, errors.Wrap(err, "DownloadFile")
			}
			durationDownloadZig := time.Since(step)
			step = time.Now()

			zigBinaryLocation := filepath.Join(tmpDir, "zig/zig"+exeExt)
			fmt.Fprintln(os.Stderr, "Zig", zigVersion, "installing to:", zigBinaryLocation)

			// Extract the Zig archive
			err = ExtractArchive(archiveFilePath, filepath.Join(tmpDir, "zig"), stripPathComponents)(os.Stderr)
			if err != nil {
				return nil, errors.Wrap(err, "ExtractArchive")
			}
			durationExtractZig := time.Since(step)
			step = time.Now()

			err = ExecArgs("zig", []string{"-h"})(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationRunZigHelp := time.Since(step)
			step = time.Now()

			err = ExecArgs("zig", []string{"-h"})(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationRunZigHelp2 := time.Since(step)
			step = time.Now()

			err = ExecArgs("git", []string{"clone", "https://github.com/hexops/mach-core", repoDir})(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationGitClone := time.Since(step)
			step = time.Now()

			statRepoSizeBytes, statNumFiles, err := DirStats(repoDir)
			if err != nil {
				return nil, err
			}
			durationDirStats := time.Since(step)
			step = time.Now()

			err = ExecArgs("zig", []string{"build"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuild := time.Since(step)

			wipeBuildCache := func() {
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-out"))
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-cache/h"))
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-cache/i"))
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-cache/o"))
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-cache/tmp"))
				_ = os.RemoveAll(filepath.Join(repoDir, "zig-cache/z"))
			}
			wipeBuildCache()

			step = time.Now()
			err = ExecArgs("zig", []string{"build"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuild2 := time.Since(step)

			fiTexturedCube, err := os.Stat(filepath.Join(repoDir, "zig-out/bin/textured-cube"+exeExt))
			if err != nil {
				return nil, errors.Wrap(err, "fiTexturedCube")
			}
			fiTriangle, err := os.Stat(filepath.Join(repoDir, "zig-out/bin/triangle"+exeExt))
			if err != nil {
				return nil, errors.Wrap(err, "fiTriangle")
			}
			statBinarySizeBytesTexturedCube := fiTexturedCube.Size()
			statBinarySizeBytesTriangle := fiTriangle.Size()

			statNumExecutables := 0
			infos, err := os.ReadDir(filepath.Join(repoDir, "zig-out/bin"))
			if err != nil {
				return nil, errors.Wrap(err, "ReadDir")
			}
			for _, info := range infos {
				if filepath.Ext(info.Name()) == exeExt {
					statNumExecutables++
				}
			}

			step = time.Now()
			statRepoSizeBytesPostBuild, statNumFilesPostBuild, err := DirStats(repoDir)
			if err != nil {
				return nil, err
			}
			durationDirStatsPostBuild := time.Since(step)

			statDawnSizeBytes, _, err := DirStats(filepath.Join(repoDir, "zig-cache/mach"))
			if err != nil {
				return nil, err
			}

			wipeBuildCache()
			step = time.Now()
			err = ExecArgs("zig", []string{"build", "textured-cube"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuildTexturedCube := time.Since(step)

			wipeBuildCache()
			step = time.Now()
			err = ExecArgs("zig", []string{"build", "triangle"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuildTriangle := time.Since(step)

			wipeBuildCache()
			step = time.Now()
			err = ExecArgs("zig", []string{"build", "textured-cube", "-Doptimize=ReleaseFast"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuildTexturedCubeReleaseFast := time.Since(step)
			fiTexturedCube, err = os.Stat(filepath.Join(repoDir, "zig-out/bin/textured-cube"+exeExt))
			if err != nil {
				return nil, errors.Wrap(err, "fiTexturedCube")
			}
			statBinarySizeBytesTexturedCubeReleaseFast := fiTexturedCube.Size()

			wipeBuildCache()
			step = time.Now()
			err = ExecArgs("zig", []string{"build", "triangle", "-Doptimize=ReleaseFast"}, WorkDir(repoDir))(os.Stderr)
			if err != nil {
				return nil, err
			}
			durationZigBuildTriangleReleaseFast := time.Since(step)
			fiTriangle, err = os.Stat(filepath.Join(repoDir, "zig-out/bin/triangle"+exeExt))
			if err != nil {
				return nil, errors.Wrap(err, "fiTriangle")
			}
			statBinarySizeBytesTriangleReleaseFast := fiTriangle.Size()

			repoHEAD, err := GitRevParse(os.Stderr, repoDir, "HEAD")
			if err != nil {
				return nil, errors.Wrap(err, "GitRevParse")
			}

			fmt.Fprintln(os.Stderr, "durationQueryZigVersion", durationQueryZigVersion)
			fmt.Fprintln(os.Stderr, "durationDownloadZig", durationDownloadZig)
			fmt.Fprintln(os.Stderr, "durationExtractZig", durationExtractZig)
			fmt.Fprintln(os.Stderr, "durationRunZigHelp", durationRunZigHelp)
			fmt.Fprintln(os.Stderr, "durationRunZigHelp2", durationRunZigHelp2)
			fmt.Fprintln(os.Stderr, "durationGitClone", durationGitClone)
			fmt.Fprintln(os.Stderr, "durationDirStats", durationDirStats)
			fmt.Fprintln(os.Stderr, "statRepoSizeBytes", statRepoSizeBytes)
			fmt.Fprintln(os.Stderr, "statNumFiles", statNumFiles)
			fmt.Fprintln(os.Stderr, "durationZigBuild", durationZigBuild)
			fmt.Fprintln(os.Stderr, "durationZigBuild2", durationZigBuild2)
			fmt.Fprintln(os.Stderr, "statNumExecutables", statNumExecutables)
			fmt.Fprintln(os.Stderr, "durationDirStatsPostBuild", durationDirStatsPostBuild)
			fmt.Fprintln(os.Stderr, "statRepoSizeBytesPostBuild", statRepoSizeBytesPostBuild)
			fmt.Fprintln(os.Stderr, "statNumFilesPostBuild", statNumFilesPostBuild)
			fmt.Fprintln(os.Stderr, "statDawnSizeBytes", statDawnSizeBytes)
			fmt.Fprintln(os.Stderr, "durationZigBuildTexturedCube", durationZigBuildTexturedCube)
			fmt.Fprintln(os.Stderr, "durationZigBuildTriangle", durationZigBuildTriangle)
			fmt.Fprintln(os.Stderr, "statBinarySizeBytesTexturedCube", statBinarySizeBytesTexturedCube)
			fmt.Fprintln(os.Stderr, "statBinarySizeBytesTriangle", statBinarySizeBytesTriangle)
			fmt.Fprintln(os.Stderr, "durationZigBuildTexturedCubeReleaseFast", durationZigBuildTexturedCubeReleaseFast)
			fmt.Fprintln(os.Stderr, "durationZigBuildTriangleReleaseFast", durationZigBuildTriangleReleaseFast)
			fmt.Fprintln(os.Stderr, "statBinarySizeBytesTexturedCubeReleaseFast", statBinarySizeBytesTexturedCubeReleaseFast)
			fmt.Fprintln(os.Stderr, "statBinarySizeBytesTriangleReleaseFast", statBinarySizeBytesTriangleReleaseFast)

			meta := map[string]any{
				"zig version": zigVersion,
				"repo":        repoHEAD,
			}
			return &api.ScriptResponse{
				Stats: []api.Stat{
					{
						ID:       "mach-core-time-build-examples",
						Type:     api.StatTypeNs,
						Value:    durationZigBuild2.Nanoseconds(),
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-repo",
						Type:     api.StatTypeBytes,
						Value:    statRepoSizeBytes,
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-dawn",
						Type:     api.StatTypeBytes,
						Value:    statDawnSizeBytes,
						Metadata: meta,
					},

					{
						ID:       "mach-core-time-build-textured-cube-debug",
						Type:     api.StatTypeNs,
						Value:    durationZigBuildTexturedCube.Nanoseconds(),
						Metadata: meta,
					},
					{
						ID:       "mach-core-time-build-triangle-debug",
						Type:     api.StatTypeNs,
						Value:    durationZigBuildTriangle.Nanoseconds(),
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-textured-cube-debug",
						Type:     api.StatTypeBytes,
						Value:    statBinarySizeBytesTexturedCube,
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-triangle-debug",
						Type:     api.StatTypeBytes,
						Value:    statBinarySizeBytesTriangle,
						Metadata: meta,
					},

					{
						ID:       "mach-core-time-build-textured-cube-release-fast",
						Type:     api.StatTypeNs,
						Value:    durationZigBuildTexturedCubeReleaseFast.Nanoseconds(),
						Metadata: meta,
					},
					{
						ID:       "mach-core-time-build-triangle-release-fast",
						Type:     api.StatTypeNs,
						Value:    durationZigBuildTriangleReleaseFast.Nanoseconds(),
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-textured-cube-release-fast",
						Type:     api.StatTypeBytes,
						Value:    statBinarySizeBytesTexturedCubeReleaseFast,
						Metadata: meta,
					},
					{
						ID:       "mach-core-size-triangle-release-fast",
						Type:     api.StatTypeBytes,
						Value:    statBinarySizeBytesTriangleReleaseFast,
						Metadata: meta,
					},
				},
			}, nil
		},
	})
}

func DirStats(path string) (int64, int64, error) {
	var size int64
	var numFiles int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			numFiles++
		}
		return err
	})
	return size, numFiles, err
}
