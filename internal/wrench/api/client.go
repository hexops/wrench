package api

import (
	"bytes"
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

func clientDo[Request any, Response any](c *Client, ctx context.Context, r *Request, endpoint string) (*Response, error) {
	if c.client == nil {
		c.client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+c.Secret)))
				return nil
			},
		}
	}

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+endpoint, bytes.NewBuffer(jsonBytes))
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

func (c *Client) RunnerJobUpdate(ctx context.Context, r *RunnerJobUpdateRequest) (*RunnerJobUpdateResponse, error) {
	return clientDo[RunnerJobUpdateRequest, RunnerJobUpdateResponse](c, ctx, r, "/api/runner/job-update")
}

func (c *Client) RunnerList(ctx context.Context, r *RunnerListRequest) (*RunnerListResponse, error) {
	return clientDo[RunnerListRequest, RunnerListResponse](c, ctx, r, "/api/runner/list")
}

func (c *Client) SecretsList(ctx context.Context, r *SecretsListRequest) (*SecretsListResponse, error) {
	return clientDo[SecretsListRequest, SecretsListResponse](c, ctx, r, "/api/secrets/list")
}

func (c *Client) SecretsDelete(ctx context.Context, r *SecretsDeleteRequest) (*SecretsDeleteResponse, error) {
	return clientDo[SecretsDeleteRequest, SecretsDeleteResponse](c, ctx, r, "/api/secrets/delete")
}

func (c *Client) SecretsUpsert(ctx context.Context, r *SecretsUpsertRequest) (*SecretsUpsertResponse, error) {
	return clientDo[SecretsUpsertRequest, SecretsUpsertResponse](c, ctx, r, "/api/secrets/upsert")
}
