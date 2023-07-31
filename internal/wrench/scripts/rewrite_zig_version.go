package scripts

import (
	"os"
	"regexp"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "rewrite-zig-version",
		Args:        []string{"new version"},
		Description: "Rewrite the Zig version used inside a repository",
		Execute: func(args ...string) error {
			if len(args) != 1 {
				return errors.New("expected [new version] argument")
			}
			newVersion := args[0]

			zigVersionRegexp := regexp.MustCompile(`(\d\.?)+-[[:alnum:]]+.\d+\+[[:alnum:]]+`)

			replacer := func(name string, contents []byte) ([]byte, error) {
				contents = zigVersionRegexp.ReplaceAllLiteral(contents, []byte(newVersion))
				return contents, nil
			}
			err := FindAndReplace(".", []string{
				"**/*.md",
				"**/*.yml",
				"**/*.yaml",
				"build.zig",
			}, replacer)(os.Stderr)
			if err != nil {
				return errors.Wrap(err, "FindAndReplace")
			}
			return nil
		},
	})
}
