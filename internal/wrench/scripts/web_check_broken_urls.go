package scripts

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "web-check-broken-urls",
		Args:        nil,
		Description: "wrench checks machengine.org for broken URLs",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			website := "https://machengine.org/next"
			ignoredBrokenURLPrefixes := []string{
				"https://stackoverflow.com", // SO prevents scraping so always 403
				"https://alain.xyz/blog/",   // JavaScript IDs
			}
			ignoredLargeBodySizeURLPrefixes := []string{
				"https://media.machengine.org", // videos
				"https://pkg.machengine.org",   // zig tarball downloads
				"https://github.com/hexops/machengine.org/archive/refs/heads/gh-pages.zip", // website download
			}

			if err := installMuffet(); err != nil {
				return nil, err
			}

			results, err := muffet(website)
			if err != nil {
				return nil, err
			}
			var brokenLinks [][3]string
			for _, result := range results {
				fmt.Fprintln(os.Stderr, "checked", result.URL)
			l:
				for _, link := range result.Links {
					if link.Error == "" {
						continue
					}
					for _, ignoredPrefix := range ignoredBrokenURLPrefixes {
						if strings.HasPrefix(link.URL, ignoredPrefix) {
							continue l
						}
					}
					if strings.Contains(link.Error, "body size exceeds the given limit") {
						// large file
						for _, ignoredPrefix := range ignoredLargeBodySizeURLPrefixes {
							if strings.HasPrefix(link.URL, ignoredPrefix) {
								continue l
							}
						}
					}
					brokenLinks = append(brokenLinks, [3]string{result.URL, link.URL, link.Error})
				}
			}

			var buf bytes.Buffer
			var issues []api.UpsertIssue
			if len(brokenLinks) > 0 {
				fmt.Fprintf(&buf, "[Wrench](https://wrench.machengine.org) here! I found these broken links on %s:\n\n", website)
				for _, pair := range brokenLinks {
					onPageURL, urlString, err := pair[0], pair[1], pair[2]
					fmt.Fprintf(&buf, "* %s (on [this page](%s), %s)\n", urlString, onPageURL, err)
				}
				fmt.Fprintf(&buf, "\nInvalid reports above can be ignored by adding to the [exclusion list here](https://github.com/hexops/wrench/blob/main/internal/wrench/scripts/web_check_broken_urls.go).")
				issues = append(issues, api.UpsertIssue{
					RepoPair: "hexops/mach",
					Title:    "website: broken links",
					Body:     buf.String(),
				})
			}
			return &api.ScriptResponse{UpsertIssues: issues}, nil
		},
	})
}
