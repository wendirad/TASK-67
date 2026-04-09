//go:build integration

package api_tests

import (
	"testing"
)

func TestCreatePost(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/posts", map[string]interface{}{
		"title":   "Test Post",
		"content": "This is a test post body.",
	})
	if resp.Code != 201 {
		t.Fatalf("Create post failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestListPosts(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/posts")
	if resp.Code != 200 {
		t.Fatalf("List posts failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestModerationQueue(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/moderation/posts")
	if resp.Code != 200 {
		t.Fatalf("Moderation queue failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestModerationRequiresRole(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/moderation/posts")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

func TestPostsRequireAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/posts")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
