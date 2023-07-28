package scripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "web-check-assets",
		Args:        nil,
		Description: "wrench checks machengine.org asset URLs",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			website := "https://machengine.org/next"
			allowedURLPrefixes := []string{
				"https://machengine.org",
				"https://raw.githubusercontent.com",
				"https://github.com",
			}

			if err := installMuffet(); err != nil {
				return nil, err
			}

			results, err := muffet(website)
			if err != nil {
				return nil, err
			}
			var notAllowed [][2]string
			for _, result := range results {
			l:
				for _, link := range result.Links {
					u, _ := url.Parse(link.URL)
					ext := path.Ext(u.Path)
					if ext == "" {
						ext = path.Ext(u.Query().Get("url"))
					}
					if ext == "" {
						continue
					}
					for _, allowedPrefix := range allowedURLPrefixes {
						if strings.HasPrefix(link.URL, allowedPrefix) {
							continue l
						}
					}
					notAllowed = append(notAllowed, [2]string{link.URL, result.URL})
				}
			}

			var buf bytes.Buffer
			if len(notAllowed) > 0 {
				fmt.Fprintf(&buf, "[Wrench](https://wrench.machengine.org) here! I found these URLs linking assets on %s that are not allowed:\n\n", website)
				hosts := map[string]struct{}{}
				for _, pair := range notAllowed {
					onPageURL, urlString := pair[0], pair[1]
					u, _ := url.Parse(urlString)
					host := u.Scheme + "://" + u.Host
					if _, ok := hosts[host]; !ok {
						hosts[host] = struct{}{}
						fmt.Fprintf(&buf, "* `%s` (on [this page](%s))\n", host, onPageURL)
					}
				}
				fmt.Fprintf(&buf, "\nThe allowlist can be found [here](https://github.com/hexops/wrench/blob/main/internal/wrench/scripts/web_check_assets.go).")
			}

			return &api.ScriptResponse{UpsertIssues: []api.UpsertIssue{
				{
					RepoPair: "hexops/machengine.org",
					Title:    "website: found asset URLs that are not allowed",
					Body:     buf.String(),
				},
			}}, nil
		},
	})
}

func installMuffet() error {
	return Exec(`go install github.com/raviqqe/muffet/v2@latest`)(os.Stderr)
}

type muffetResult struct {
	URL   string
	Links []struct {
		URL    string
		Status int
		Error  string
	}
}

func muffet(url string) ([]muffetResult, error) {
	args := []string{"--buffer-size=32768", "--verbose", "--format=json", url}
	output, err := OutputArgs(os.Stderr, "muffet", args)
	if err != nil {
		if !strings.Contains(err.Error(), "exit code") {
			return nil, err
		}
	}
	var results []muffetResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return results, nil
}
