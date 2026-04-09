//go:build integration

package api_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

var baseURL = "http://localhost:8080"

func init() {
	if url := os.Getenv("API_BASE_URL"); url != "" {
		baseURL = url
	}
}

type apiResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type apiClient struct {
	t      *testing.T
	token  string
	client *http.Client
}

func newClient(t *testing.T) *apiClient {
	return &apiClient{t: t, client: &http.Client{}}
}

func (c *apiClient) login(username, password string) string {
	c.t.Helper()
	resp := c.post("/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if resp.Code != 200 {
		c.t.Fatalf("Login failed for %s: %s", username, resp.Msg)
	}
	var data struct {
		Token string `json:"token"`
	}
	json.Unmarshal(resp.Data, &data)
	c.token = data.Token
	return data.Token
}

func (c *apiClient) get(path string) *apiResponse {
	c.t.Helper()
	return c.request("GET", path, nil)
}

func (c *apiClient) post(path string, body interface{}) *apiResponse {
	c.t.Helper()
	return c.request("POST", path, body)
}

func (c *apiClient) put(path string, body interface{}) *apiResponse {
	c.t.Helper()
	return c.request("PUT", path, body)
}

func (c *apiClient) delete(path string) *apiResponse {
	c.t.Helper()
	return c.request("DELETE", path, nil)
}

func (c *apiClient) request(method, path string, body interface{}) *apiResponse {
	c.t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("Marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		c.t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("Do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("Read body: %v", err)
	}

	var ar apiResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		c.t.Fatalf("Unmarshal response for %s %s: %v (body: %s)", method, path, err, string(respBody))
	}
	return &ar
}

func (c *apiClient) requestRaw(method, path string, body interface{}) (*http.Response, []byte) {
	c.t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	req, _ := http.NewRequest(method, baseURL+path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("Do request: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody
}

func readAdminPassword() string {
	// Try reading from secrets volume (Docker)
	password, err := os.ReadFile("/run/secrets/admin_bootstrap_password")
	if err == nil && len(password) > 0 {
		return string(password)
	}
	// Fallback for local dev
	if p := os.Getenv("ADMIN_PASSWORD"); p != "" {
		return p
	}
	// Try common paths
	for _, path := range []string{"/tmp/campusrec_secrets/admin_bootstrap_password", "./secrets/admin_bootstrap_password"} {
		password, err = os.ReadFile(path)
		if err == nil && len(password) > 0 {
			return string(password)
		}
	}
	return ""
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, os.Getpid())
}
