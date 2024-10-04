package scripts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hexops/wrench/internal/errors"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "nominate-zig-index-update",
		Args:        []string{"version"},
		Description: "Nominate a new Zig version by updating index.json",
		Execute: func(args ...string) error {
			// Read our index.json file
			data, err := os.ReadFile("index.json")
			if err != nil {
				return errors.Wrap(err, "opening ./index.json (if you are starting a new index, create an empty {} JSON file first.)")
			}
			index := orderedmap.New[string, *orderedmap.OrderedMap[string, any]]()
			if err := json.Unmarshal(data, &index); err != nil {
				return errors.Wrap(err, "parsing ./index.json")
			}

			// Fetch the latest upstream Zig index.json, which has the current nightly build at the time of querying.
			resp, err := http.Get("https://ziglang.org/download/index.json")
			if err != nil {
				return errors.Wrap(err, "fetching upstream index.json")
			}
			defer resp.Body.Close()
			latestIndex := orderedmap.New[string, *orderedmap.OrderedMap[string, any]]()
			if err := json.NewDecoder(resp.Body).Decode(&latestIndex); err != nil {
				return errors.Wrap(err, "parsing upstream index.json")
			}

			// Our index intends to be a superset of the upstream index.json, so copy everything from theirs over to ours.
			for pair := latestIndex.Oldest(); pair != nil; pair = pair.Next() {
				index.Set(pair.Key, pair.Value)
			}

			var fetches []string
			switch args[0] {
			case "update":
				// Do nothing, we're just pulling the upstream.
			case "nominate":
				// We are nominating a new version.
				if len(args) != 2 || !strings.HasSuffix(args[1], "-wip") {
					return errors.New("usage: wrench script nominated-zig-index-update nominate [2024.1.0-mach-wip]")
				}
				newVersionString := args[1]

				masterVersion, ok := latestIndex.Get("master")
				if !ok {
					panic("never here")
				}
				newVersion := orderedmap.New[string, any]()
				for pair := masterVersion.Oldest(); pair != nil; pair = pair.Next() {
					key := pair.Key
					value := pair.Value
					if sub, ok := value.(map[string]any); ok {
						cpy := map[string]any{}
						for subKey, subVal := range sub {
							cpy[subKey] = subVal
							if subKey == "tarball" {
								cpy["tarball"] = strings.Replace(subVal.(string), "https://ziglang.org/builds/", "https://pkg.machengine.org/zig/", 1)
								fetches = append(fetches, cpy["tarball"].(string))
								cpy["zigTarball"] = subVal.(string)
							}
						}
						value = cpy
					}
					newVersion.Set(key, value)
					if key == "stdDocs" {
						newVersion.Set("machDocs", "https://machengine.org/docs/nominated-zig")
						newVersion.Set("machNominated", time.Now().Format("2006-01-02"))
					}
				}
				index.Set(newVersionString, newVersion)
				index.MoveToFront(newVersionString)
			case "finalize":
				// We are finalizing the nomination of a new version.
				if len(args) != 2 || !strings.HasSuffix(args[1], "-wip") {
					return errors.New("usage: wrench script nominate-zig-index-update finalize [2024.1.0-mach-wip]")
				}
				wipVersionString := args[1]
				newVersionString := strings.TrimSuffix(wipVersionString, "-wip")

				wipVersion, ok := index.Get(wipVersionString)
				if !ok {
					return fmt.Errorf("index.json missing version entry: %q", wipVersionString)
				}
				index.Set(newVersionString, wipVersion)
				index.MoveToFront(newVersionString)
				index.Set("mach-latest", wipVersion)
				index.MoveToFront("mach-latest")
				index.Delete(wipVersionString)
			case "tag":
				// We are tagging a new version.
				if len(args) != 3 {
					return errors.New("usage: wrench script nominate-zig-index-update tag [2024.1.0-mach] [0.3.0-mach]")
				}
				targetVersionString := args[1]
				tagVersionString := args[2]

				targetVersion, ok := index.Get(targetVersionString)
				if !ok {
					return fmt.Errorf("index.json missing version entry: %q", targetVersionString)
				}
				index.Set(tagVersionString, targetVersion)
				index.MoveToFront(tagVersionString)
				index.MoveToFront("mach-latest")
			}

			// Save our updated index.json file
			data, err = json.MarshalIndent(index, "", "  ")
			if err != nil {
				return errors.Wrap(err, "marshalling index.json")
			}
			err = os.WriteFile("index.json", data, 0o700)
			if err != nil {
				return errors.Wrap(err, "writing index.json")
			}

			if len(fetches) > 0 {
				tmpDir := "nominate-zig-tmp"
				_ = os.RemoveAll(tmpDir)
				if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
					return errors.Wrap(err, "MkdirAll")
				}

				fmt.Fprintln(os.Stderr, "Fetching nominated Zig version for every OS/Arch, so pkg.machengine.org is updated")
				for _, url := range fetches {
					fileName := base64.StdEncoding.EncodeToString([]byte(url))
					err = DownloadFile(url, filepath.Join(tmpDir, fileName))(os.Stderr)
					if err != nil {
						return errors.Wrap(err, "DownloadFile")
					}
				}
				_ = os.RemoveAll(tmpDir)
			}

			return nil
		},
	})
}
