package scripts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "mach-update-dawn",
		Args:        []string{"generated-branch-name"},
		Description: "update the Dawn repository and produce a new generated branch",
		Execute: func(args ...string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected [generated-branch-name] argument")
			}
			generatedBranchName := args[0]
			workDir := "dawn"
			repoURL := "https://github.com/hexops/dawn"
			dev := false
			push := true
			gclientClean := true
			stopAfterCleanup := false
			cleanupOnly := false

			if cleanupOnly {
				if err := dawnCleanupThirdParty(workDir); err != nil {
					return errors.Wrap(err, "dawnCleanupThirdParty")
				}
				return nil
			}

			// Install depot_tools if necessary.
			_, err := os.Stat("depot_tools")
			if os.IsNotExist(err) {
				if runtime.GOOS == "windows" {
					// TODO: download an extract depot_tools zip archive:
					// https://chromium.googlesource.com/chromium/src/+/4b470c5b3ffd5b74476cb3ffbcf3ff416ea30d72/docs/windows_build_instructions.md#install
					return fmt.Errorf("wrench can't install depot_tools on Windows yet. ")
				}
				if err := GitClone(os.Stderr, "depot_tools", "https://chromium.googlesource.com/chromium/tools/depot_tools.git"); err != nil {
					return errors.Wrap(err, "GitClone")
				}
				if !dev {
					if err := EnsureOnPathPermanent("depot_tools"); err != nil {
						return errors.Wrap(err, "EnsureOnPathPermanent")
					}
				}
			}

			// Clone or update the repository.
			if err := GitCloneOrUpdateAndClean(os.Stderr, workDir, repoURL); err != nil {
				return errors.Wrap(err, "GitCloneOrUpdateAndClean")
			}
			err = GitConfigureRepo(os.Stderr, workDir)
			if err != nil {
				return errors.Wrap(err, "GitConfigureRepo")
			}

			// Update our "upstream" branch to point to latest upstream@main version.
			if err := GitRemoteAdd(os.Stderr, workDir, "upstream", "https://dawn.googlesource.com/dawn"); err != nil {
				fmt.Fprintf(os.Stderr, "ignoring: GitRemoteAdd: %s", err)
			}
			if err := GitFetch(os.Stderr, workDir, "upstream"); err != nil {
				return errors.Wrap(err, "GitFetch")
			}
			if push {
				if err := ExecArgs("git", []string{
					"push",
					GitRemoteURLWithAuth(repoURL),
					"refs/remotes/upstream/main:refs/heads/upstream",
				}, WorkDir(workDir))(os.Stderr); err != nil {
					return errors.Wrap(err, "GitPush")
				}
			}

			// Update our "main" branch by merging upstream into it.
			if err := GitCheckout(os.Stderr, workDir, "main"); err != nil {
				return errors.Wrap(err, "GitCheckout")
			}
			if err := GitMerge(os.Stderr, workDir, "upstream/main"); err != nil {
				return errors.Wrap(err, "GitMerge")
			}
			if push {
				if err := ExecArgs("git", []string{
					"push",
					GitRemoteURLWithAuth(repoURL),
					"HEAD:main",
				}, WorkDir(workDir))(os.Stderr); err != nil {
					return errors.Wrap(err, "GitPush")
				}
			}

			// Create the new [generated-branch-name] branch.
			if err := GitCheckout(os.Stderr, workDir, "main"); err != nil {
				return errors.Wrap(err, "GitCheckout")
			}
			if err := GitCheckoutNewBranch(os.Stderr, workDir, generatedBranchName); err != nil {
				return errors.Wrap(err, "GitCheckout")
			}

			// Bootstrap our gclient config
			if err := CopyFile(
				filepath.Join(workDir, "scripts/standalone.gclient"),
				filepath.Join(workDir, ".gclient"),
			)(os.Stderr); err != nil {
				return errors.Wrap(err, "CopyFile")
			}

			// Wipe existing gclient download directories.
			os.RemoveAll(filepath.Join(workDir, "build/"))
			if gclientClean {
				os.RemoveAll(filepath.Join(workDir, "third_party/"))
				if err := GitCheckoutRestore(os.Stderr, workDir, "third_party/"); err != nil {
					return errors.Wrap(err, "GitCheckoutRestore")
				}
			}
			// Ask gclient to sync dependencies.
			if err := Exec("gclient sync", WorkDir(workDir))(os.Stderr); err != nil {
				return errors.Wrap(err, "gclient sync")
			}

			// Execute ninja for code generation
			pairs := [][2]string{
				{"win", "x64"},
				{"mac", "arm64"},
				{"linux", "x64"},
			}
			os.RemoveAll(filepath.Join(workDir, "out/"))
			for _, pair := range pairs {
				dawnOS, dawnArch := pair[0], pair[1]
				fmt.Fprintf(os.Stderr, "generating for %s/%s\n", dawnOS, dawnArch)
				if dawnOS == "linux" {
					if err := Exec(
						"python3 build/linux/sysroot_scripts/install-sysroot.py --arch=amd64",
						WorkDir(workDir),
					)(os.Stderr); err != nil {
						return errors.Wrap(err, "install-sysroot.py")
					}
				}
				if dawnOS == "win" {
					if err := dawnWriteVSToolchainHacks(workDir); err != nil {
						return errors.Wrap(err, "writeVSToolchainHacks")
					}
				}
				if err := dawnWriteNinjaArgs(workDir, dawnOS, dawnArch); err != nil {
					return errors.Wrap(err, "gclient sync")
				}
				if err := Exec("gn gen out/Debug", WorkDir(workDir))(os.Stderr); err != nil {
					return errors.Wrap(err, "gen gen out/Debug")
				}

				targets, err := dawnFindNinjaGenerationTargets(workDir)
				if err != nil {
					return errors.Wrap(err, "FindNinjaGenerationTargets")
				}
				if err := ExecArgs(
					"ninja",
					append([]string{"-C", "./out/Debug"}, targets...),
					WorkDir(workDir),
				)(os.Stderr); err != nil {
					return errors.Wrap(err, "gen gen out/Debug")
				}
			}

			// Add generated sources
			err = ExecArgs("git", []string{"add", "-f", "out/Debug"}, WorkDir(workDir))(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "git add -f out/Debug")
			}
			for _, pattern := range []string{
				"**/*.ninja*",
				"out/Debug/args.gn **/*.gn",
				"**/*.runtime_deps",
				"**/*.stamp",
				"**/*.allowed_output_dirs",
				"**/*.expected_outputs",
				"**/*.stale_dirs",
				"**/*.json_tarball.d",
				"out/Debug/gen/third_party/vulkan-deps/vulkan-validation-layers/src/old_vvl_files_are_removed",
			} {
				err = ExecArgs("git", []string{"reset", "HEAD", "--", pattern}, WorkDir(workDir))(os.Stderr)
				if err != nil {
					return errors.Wrap(err, "git reset HEAD -- "+pattern)
				}
			}
			err = ExecArgs("git", []string{"commit", "-s", "-m", "generated: commit generated code"}, WorkDir(workDir))(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "git commit")
			}

			// Cleanup the third_party/ directory
			if err := dawnCleanupThirdParty(workDir); err != nil {
				return errors.Wrap(err, "dawnCleanupThirdParty")
			}
			if stopAfterCleanup {
				return nil
			}

			// Commit vendored dependencies
			err = ExecArgs("git", []string{"add", "-f", "third_party/"}, WorkDir(workDir))(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "git add -f third_party/")
			}
			err = ExecArgs("git", []string{"commit", "-s", "-m", "generated: commit vendored dependencies"}, WorkDir(workDir))(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "git commit")
			}
			if push {
				if err := ExecArgs("git", []string{
					"push",
					GitRemoteURLWithAuth(repoURL),
					"HEAD:" + generatedBranchName,
				}, WorkDir(workDir))(os.Stderr); err != nil {
					return errors.Wrap(err, "GitPush")
				}
			}
			return nil
		},
	})
}

// os: "win", "mac", "linux"
// arch: "x64"
func dawnWriteNinjaArgs(workDir, dawnOS, dawnArch string) error {
	ninjaArgs := fmt.Sprintf(`
# Common
is_debug=true
use_cxx11=true
target_os="%s"
target_cpu="%s"

`, dawnOS, dawnArch)

	switch dawnOS {
	case "mac":
		ninjaArgs += `
# macOS
use_swiftshader_with_subzero=false
use_system_xcode=true
`
	case "win":
		ninjaArgs += `
# Windows
use_swiftshader_with_subzero=false
`
	case "linux":
		ninjaArgs += `
use_swiftshader_with_subzero=false
dawn_use_x11=true
`
	default:
		return errors.New("OS not supported: " + dawnOS)
	}

	fmt.Fprintf(os.Stderr, "$ mkdir -p %s\n", filepath.Join(workDir, "out/Debug"))
	if err := os.MkdirAll(filepath.Join(workDir, "out/Debug"), os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}
	fmt.Fprintf(os.Stderr, "WriteFile: %s\n", filepath.Join(workDir, "out/Debug/args.gn"))
	if err := os.WriteFile(filepath.Join(workDir, "out/Debug/args.gn"), []byte(ninjaArgs), 0o655); err != nil {
		return errors.Wrap(err, "WriteFile")
	}
	return nil
}

func dawnWriteVSToolchainHacks(workDir string) error {
	fmt.Fprintf(os.Stderr, "$ mkdir -p %s\n", filepath.Join(workDir, "build/toolchain/win/"))
	if err := os.MkdirAll(filepath.Join(workDir, "build/toolchain/win/"), os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}
	vsToolchainPy := `
print('vs_path = ""')
print('sdk_path = ""')
print('sdk_version = ""')
print('vs_version = ""')
print('wdk_dir = ""')
print('runtime_dirs = ""')
`
	fmt.Fprintf(os.Stderr, "WriteFile: %s\n", filepath.Join(workDir, "build/vs_toolchain.py"))
	if err := os.WriteFile(filepath.Join(workDir, "build/vs_toolchain.py"), []byte(vsToolchainPy), 0o655); err != nil {
		return errors.Wrap(err, "WriteFile")
	}

	setupToolchainPy := `
print('include_flags_imsvc = ""')
print('libpath_flags = ""')
print('libpath_lldlink_flags = ""')
print('vc_lib_path = "tmp"')
print('vc_lib_um_path = "tmp"')
`
	fmt.Fprintf(os.Stderr, "WriteFile: %s\n", path.Join(workDir, "build/toolchain/win/setup_toolchain.py"))
	if err := os.WriteFile(filepath.Join(workDir, "build/toolchain/win/setup_toolchain.py"), []byte(setupToolchainPy), 0o655); err != nil {
		return errors.Wrap(err, "WriteFile")
	}
	return nil
}

func dawnFindNinjaGenerationTargets(dir string) ([]string, error) {
	output, err := Output(os.Stderr, "ninja -C out/Debug/ -t targets", WorkDir(dir))
	if err != nil {
		return nil, errors.Wrap(err, "listing targets")
	}
	var targets []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pair := strings.Split(line, ": ")
		target, targetType := pair[0], pair[1]
		if targetType == "gn" || targetType == "solink" || targetType == "stamp" {
			continue
		}
		if strings.Contains(target, "tests") {
			continue
		}
		ignored := map[string]bool{
			"clang-tblgen.exe.pdb":                           true,
			"llvm-tblgen.exe.pdb":                            true,
			"clang-tblgen":                                   true,
			"llvm-tblgen":                                    true,
			"clang_lib_codegen":                              true,
			"third_party/gn/dxc:clang-tblgen":                true,
			"third_party/gn/dxc:clang_lib_codegen":           true,
			"third_party/gn/dxc:llvm-tblgen":                 true,
			"third_party/gn/dxc:gen_intrin_main_tables_15-h": true,
			"third_party/gn/dxc:hlsl_dxcversion_autogen":     true,
		}
		if _, ignore := ignored[target]; ignore {
			continue
		}
		spirvAllowed := strings.HasPrefix(target, "third_party/vulkan-deps/spirv-tools/src:")
		spirvAllowed = spirvAllowed && (strings.Contains(target, "_header") ||
			strings.Contains(target, "_tables") ||
			strings.Contains(target, "_enums") ||
			strings.Contains(target, "_inc") ||
			strings.Contains(target, "_build_version"))
		if !strings.Contains(target, "gen") && !spirvAllowed {
			continue
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func dawnCleanupThirdParty(dir string) error {
	// Remove gitmodules, some third_party/ repositories contain these and leaving them around would
	// cause any recursive submodule clones to fail because e.g. some reference internal Google
	// repositories. We don't want them anyway.
	if err := FindAndDelete(dir, []string{"third_party/**/.gitmodules"}, func(name string) (bool, error) {
		return true, nil
	})(os.Stderr); err != nil {
		return errors.Wrap(err, "FindAndDelete")
	}

	// Turn subrepositories into regular folders.
	if err := FindAndDelete(dir, []string{"third_party/**/.git"}, func(name string) (bool, error) {
		fi, err := os.Stat(name)
		return fi.IsDir(), err
	})(os.Stderr); err != nil {
		return errors.Wrap(err, "FindAndDelete")
	}
	_ = os.RemoveAll(filepath.Join(dir, "third_party/vulkan-deps/.gitignore"))

	// Remove files that are not needed.
	for _, path := range []string{
		"third_party/abseil-cpp/.github/",
		"third_party/abseil-cpp/ci",
		"third_party/abseil-cpp/absl/time/internal/cctz/testdata",
		"third_party/googletest/",
		"third_party/glfw/",
		"third_party/llvm-build/",
		"third_party/tint/test/",
		"third_party/tint/fuzzers/",
		"third_party/tint/tools/",
		"third_party/tint/kokoro/",
		"third_party/tint/infra/",
		"third_party/angle/doc/",
		"third_party/angle/extensions/",
		"third_party/angle/third_party/logdog/",
		"third_party/angle/src/android_system_settings",
		"third_party/vulkan-deps/glslang/src/Test",
		"third_party/vulkan-deps/spirv-cross",
		"third_party/vulkan-deps/spirv-tools/src/test",
		"third_party/vulkan-deps/spirv-tools/src/source/fuzz/",
		"third_party/vulkan-deps/spirv-tools/src/kokoro/",
		"third_party/vulkan-deps/glslang/src/kokoro/",
		"third_party/vulkan-deps/glslang/src/gtests/",
		"third_party/vulkan-deps/vulkan-tools/src/cube/",
		"third_party/vulkan-deps/vulkan-tools/src/scripts/",
		"third_party/vulkan-deps/vulkan-tools/src/vulkaninfo/",
		"third_party/vulkan-deps/vulkan-tools/src/windows-runtime-installer/",
		"third_party/vulkan-deps/vulkan-validation-layers/src/scripts/",
		"third_party/vulkan_memory_allocator/build/src/Release/",
		"third_party/vulkan_memory_allocator/build/src/VmaReplay/",
		"third_party/vulkan_memory_allocator/tools/",
		"third_party/zlib/google/test/",
		"third_party/webgpu-cts/",
		"third_party/benchmark/",
		"third_party/gpuweb-cts/",
		"third_party/protobuf/",
		"third_party/markupsafe/",
		"third_party/jinja2/",
		"third_party/catapult/",

		"third_party/swiftshader/third_party/SPIRV-Tools",   // already in third_party/vulkan-deps/spirv-tools
		"third_party/swiftshader/third_party/SPIRV-Headers", // already in third_party/vulkan-deps/spirv-headers
		"third_party/swiftshader/third_party/llvm-subzero",  // already in third_party/vulkan-deps/llvm-subzero
	} {
		_ = os.RemoveAll(filepath.Join(dir, path))
	}

	if err := FindAndDelete(dir, []string{
		"third_party/**/tests/**",
		"third_party/**/docs/**",
		"third_party/**/samples/**",
		"third_party/**/CMake/**",
		"third_party/**/*CMake*",
	}, func(name string) (bool, error) {
		return true, nil
	})(os.Stderr); err != nil {
		return errors.Wrap(err, "FindAndDelete")
	}
	return nil
}
