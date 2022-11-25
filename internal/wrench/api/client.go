package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	URL    string
	Secret string

	client *http.Client
}

func clientDo[Request comparable, Response comparable](c *Client, ctx context.Context, r *Request, endpoint string) (*Response, error) {
	if c.client == nil {
		c.client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+c.Secret)))
				return nil
			},
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+c.Secret)))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response code: %v %v", resp.StatusCode, string(body))
	}

	var rsp Response
	if err := json.NewDecoder(resp.Body).Decode(&rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

func (c *Client) RunnerPoll(ctx context.Context, r *RunnerPollRequest) (*RunnerPollResponse, error) {
	return clientDo[RunnerPollRequest, RunnerPollResponse](c, ctx, r, "/api/runner/poll")
}
