package scripts

import (
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "web-check-assets",
		Args:        nil,
		Description: "wrench checks machengine.org asset URLs",
		ExecuteResponse: func(args ...string) (*api.ScriptResponse, error) {
			website := "https://machengine.org/next"
			_ = website

			return &api.ScriptResponse{UpsertIssues: []api.UpsertIssue{
				{
					RepoPair: "hexops/machengine.org",
					Title:    "test issue 1",
					Body:     "test issue 1 body",
				},
			}}, nil
		},
	})
}
