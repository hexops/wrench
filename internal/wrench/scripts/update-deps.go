package scripts

import (
	"fmt"
	"net/url"
	"os"
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
				deps := tree.Child("dependencies")
				if deps == nil {
					continue
				}
				for i, dep := range deps.Tags {
					name := dep.Name
					urlNode := dep.Node.Child("url")
					hash := dep.Node.Child("hash")

					fmt.Println("check >", name, urlNode.StringLiteral)

					// Checkout the repository and determine the latest HEAD commit
					u, err := url.Parse(urlNode.StringLiteral)
					if err != nil {
						return errors.Wrap(err, "Parse")
					}
					split := strings.Split(u.Path, "/")
					repoURL := "github.com/" + split[1] + "/" + split[2]
					cloneWorkDir := "tmp-clone"
					_ = os.RemoveAll(cloneWorkDir)
					if err := GitClone(os.Stderr, cloneWorkDir, repoURL); err != nil {
						return errors.Wrap(err, "GitClone")
					}
					latestHEAD, err := GitRevParse(os.Stderr, cloneWorkDir, "HEAD")
					if err != nil {
						return errors.Wrap(err, "GitRevParse")
					}
					split[4] = latestHEAD + ".tar.gz"
					u.Path = strings.Join(split, "/")
					urlNode.StringLiteral = u.String()

					archiveFilePath := "tmp.tar.gz"
					tmpDir := "tmp"
					_ = os.RemoveAll(tmpDir)
					err = DownloadFile(urlNode.StringLiteral, archiveFilePath)(os.Stderr)
					if err != nil {
						return errors.Wrap(err, "DownloadFile")
					}
					stripPathComponents := 1
					err = ExtractArchive(archiveFilePath, tmpDir, stripPathComponents)(os.Stderr)
					if err != nil {
						return errors.Wrap(err, "ExtractArchive")
					}

					pkgHash, err := zon.ComputePackageHash(tmpDir)
					if err != nil {
						return errors.Wrap(err, "ComputePackageHash")
					}

					hash.StringLiteral = pkgHash
					deps.Tags[i] = dep
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
