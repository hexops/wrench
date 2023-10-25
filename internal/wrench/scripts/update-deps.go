package scripts

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/zon"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "update-deps",
		Args:        []string{},
		Description: "Update build.zig.zon dependencies",
		Execute: func(args ...string) error {
			const wrenchUpdateCache = ".wrench-update-cache"
			if err := os.MkdirAll(wrenchUpdateCache, os.ModePerm); err != nil {
				return err
			}
			defer os.RemoveAll(wrenchUpdateCache)

			fsys := os.DirFS(".")
			matches, err := doublestar.Glob(fsys, "**/build.zig.zon")
			if err != nil {
				return errors.Wrap(err, "Glob")
			}
			for _, match := range matches {
				fmt.Println(":", match)
				zonFile, err := os.ReadFile(match)
				if err != nil {
					return errors.Wrap(err, "ReadFile")
				}
				tree, err := zon.Parse(string(zonFile))
				if err != nil {
					return errors.Wrap(err, "Parse "+match)
				}
				rootStruct := tree.FirstStruct()
				if rootStruct == nil {
					return errors.Wrap(err, "Unable to find root .{} struct")
				}
				deps := rootStruct.Child("dependencies")
				if deps == nil {
					continue
				}
				for i, dep := range deps.Children {
					if dep.DotName == "" {
						continue
					}
					name := dep.DotName
					urlNode := dep.DotValue.Child("url")
					hash := dep.DotValue.Child("hash")

					fmt.Println("check >", name, urlNode.StringLiteral)

					// Checkout the repository and determine the latest HEAD commit
					u, err := url.Parse(urlNode.StringLiteral)
					if err != nil {
						return errors.Wrap(err, "Parse")
					}
					split := strings.Split(u.Path, "/")

					orgName := split[1]
					repoName := split[2]
					if u.Host == "pkg.machengine.org" {
						// https://pkg.machengine.org/mach-ecs/3c5e29fb08b737a10fedfa70a7659d3506626435.tar.gz
						orgName = "hexops"
						repoName = split[1]
					}

					repoURL := "github.com/" + orgName + "/" + repoName
					cacheKey := orgName + "-" + repoName
					cloneWorkDir := filepath.Join(wrenchUpdateCache, cacheKey)

					if _, err := os.Stat(cloneWorkDir); os.IsNotExist(err) {
						if err := GitClone(os.Stderr, cloneWorkDir, repoURL); err != nil {
							return errors.Wrap(err, "GitClone")
						}
						if repoName == "mach-sysjs" {
							if err := GitCheckout(os.Stderr, cloneWorkDir, "v0"); err != nil {
								return errors.Wrap(err, "GitCheckout")
							}
						}
					}
					latestHEAD, err := GitRevParse(os.Stderr, cloneWorkDir, "HEAD")
					if err != nil {
						return errors.Wrap(err, "GitRevParse")
					}
					if u.Host == "github.com" {
						split[4] = latestHEAD + ".tar.gz"
					} else if u.Host == "pkg.machengine.org" {
						split[2] = latestHEAD + ".tar.gz"
					}
					u.Path = strings.Join(split, "/")
					urlNode.StringLiteral = u.String()

					pkgHash, err := calculatePkgHash(urlNode.StringLiteral)
					if err != nil {
						return errors.Wrap(err, "calculatePkgHash")
					}
					hash.StringLiteral = pkgHash
					deps.Children[i] = dep
				}

				// Write build.zig.zon
				f, err := os.Create(match)
				if err != nil {
					return errors.Wrap(err, "Create")
				}
				if err := tree.Write(f, "    ", ""); err != nil {
					f.Close()
					return errors.Wrap(err, "Write")
				}
				f.Close()
			}
			return nil
		},
	})
}

func calculatePkgHash(url string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "UserHomeDir")
	}
	pkgHashDir := filepath.Join(homeDir, ".cache/wrench/pkg-hash")
	if err := os.MkdirAll(pkgHashDir, os.ModePerm); err != nil {
		return "", errors.Wrap(err, "MkdirAll")
	}
	urlSha := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
	hashFile := filepath.Join(pkgHashDir, urlSha)
	if _, err := os.Stat(hashFile); os.IsNotExist(err) {
		zigHash, err := OutputArgs(os.Stderr, "zig", []string{"fetch", url})
		if err != nil {
			return "", errors.Wrap(err, "zig fetch "+url)
		}
		zigHash = strings.TrimSpace(zigHash)
		err = os.WriteFile(hashFile, []byte(zigHash), 0o700)
		if err != nil {
			return "", errors.Wrap(err, "writing "+hashFile)
		}
		return zigHash, nil
	}
	zigHash, err := os.ReadFile(hashFile)
	if err != nil {
		return "", errors.Wrap(err, "reading "+hashFile)
	}
	return string(zigHash), nil
}
