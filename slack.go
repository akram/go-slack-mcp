package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const slackAPIBase = "https://slack.com/api"

type slackClient struct {
	xoxcToken string
	xoxdToken string
	http      *http.Client
}

func newSlackClient(xoxc, xoxd string) *slackClient {
	return &slackClient{
		xoxcToken: xoxc,
		xoxdToken: xoxd,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *slackClient) request(method, endpoint string, payload map[string]any) (map[string]any, error) {
	apiURL := slackAPIBase + "/" + endpoint

	var req *http.Request
	var err error

	if method == "GET" {
		u, err := url.Parse(apiURL)
		if err != nil {
			return nil, err
		}
		if payload != nil {
			q := u.Query()
			for k, v := range payload {
				q.Set(k, fmt.Sprintf("%v", v))
			}
			u.RawQuery = q.Encode()
		}
		req, err = http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, err
		}
	} else {
		var body io.Reader
		if payload != nil {
			b, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(b)
		}
		req, err = http.NewRequest(method, apiURL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+c.xoxcToken)
	req.Header.Set("User-Agent", "MCP-Server/1.0")
	req.AddCookie(&http.Cookie{Name: "d", Value: c.xoxdToken})

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("slack API %s returned %d: %s", endpoint, resp.StatusCode, string(b))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
