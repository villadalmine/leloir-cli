package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// client is a thin HTTP client for the control plane API. The API key travels
// only as an Authorization header and is never logged.
type client struct {
	server  string
	apiKey  string
	tenant  string // single-user dev only (?tenant=)
	jsonOut bool
	http    *http.Client
}

func newClient(g globals) (*client, error) {
	ctx, err := resolveContext(g)
	if err != nil {
		return nil, err
	}
	return &client{
		server:  strings.TrimSuffix(ctx.Server, "/"),
		apiKey:  ctx.APIKey,
		tenant:  ctx.Tenant,
		jsonOut: g.jsonOut,
		http:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// apiURL builds {server}/api/v1{path}, appending ?tenant= in single-user dev.
func (c *client) apiURL(path string) string {
	u := c.server + "/api/v1" + path
	if c.apiKey == "" && c.tenant != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		u += sep + "tenant=" + url.QueryEscape(c.tenant)
	}
	return u
}

func (c *client) do(method, path string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.apiURL(path), reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("no pude hablar con %s: %w", c.server, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

func (c *client) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// rawGet fetches a non-API path (e.g. /healthz) without auth decoration.
func (c *client) rawGet(path string) ([]byte, error) {
	resp, err := c.http.Get(c.server + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// apiError turns a non-2xx response into a readable error.
func apiError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		msg = e.Error
	}
	return fmt.Errorf("API %d: %s", status, msg)
}

// getJSON GETs path; on success prints raw body if --json and returns it.
func (c *client) getJSON(path string, out any) ([]byte, error) {
	body, status, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, apiError(status, body)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, fmt.Errorf("respuesta inesperada: %w", err)
		}
	}
	return body, nil
}

// emitJSON prints the raw API body when --json is set. Returns true if handled.
func (c *client) emitJSON(body []byte) bool {
	if !c.jsonOut {
		return false
	}
	fmt.Println(strings.TrimSpace(string(body)))
	return true
}

// fail prints an error and returns exit code 1.
func fail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}

// newFlagSet builds a FlagSet that reports usage errors with exit code 2 semantics.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// tabWriter-lite: fixed column printf helpers keep the output stable for
// scripts that cut/awk it.
func printRow(cols ...string) {
	fmt.Println(strings.Join(cols, "  "))
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
