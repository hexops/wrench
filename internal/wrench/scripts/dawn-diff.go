package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "dawn-diff",
		Args:        []string{"old generated branch", "new generated branch"},
		Description: "Produce a diff between Dawn generated branches helpful when updating build.zig",
		Execute: func(args ...string) error {
			if len(args) != 2 {
				return errors.New("expected [old generated branch] [new generated branch] arguments")
			}
			oldBranch := args[0]
			newBranch := args[1]

			before, err := GitLsTreeFull(os.Stderr, oldBranch, ".", ".")
			if err != nil {
				return err
			}
			after, err := GitLsTreeFull(os.Stderr, newBranch, ".", ".")
			if err != nil {
				return err
			}
			dirsBefore := map[string]struct{}{}
			dirsAfter := map[string]struct{}{}
			for _, path := range before {
				dirsBefore[filepath.Dir(path)] = struct{}{}
			}
			for _, path := range after {
				dirsAfter[filepath.Dir(path)] = struct{}{}
			}
			var deletions []string
			var additions []string
			for path := range dirsBefore {
				if _, ok := dirsAfter[path]; ok {
					continue
				}
				deletions = append(deletions, path)
			}
			for path := range dirsAfter {
				if _, ok := dirsBefore[path]; ok {
					continue
				}
				additions = append(additions, path)
			}
			sort.Strings(deletions)
			sort.Strings(additions)

			for _, path := range deletions {
				fmt.Println("D", path)
			}
			for _, path := range additions {
				fmt.Println("A", path)
			}
			return nil
		},
	})
}
