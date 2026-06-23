package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"servcache/pkg/cache"
	"servcache/pkg/server"
)

func TestInMemoryCacheOperations(t *testing.T) {
	c := cache.NewInMemoryCache(100 * time.Millisecond)

	// Test GET non-existent key
	_, found, err := c.Get("non-existent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected key to not be found")
	}

	// Test SET & GET
	err = c.Set("my_key", "my_val", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, found, err := c.Get("my_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found || val != "my_val" {
		t.Errorf("expected 'my_val', got '%v'", val)
	}

	// Test DELETE
	err = c.Delete("my_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, found, _ = c.Get("my_key")
	if found {
		t.Error("expected key to be deleted")
	}

	// Test CLEAR
	_ = c.Set("k1", "v1", 0)
	_ = c.Set("k2", "v2", 0)
	_ = c.Clear()

	_, found1, _ := c.Get("k1")
	_, found2, _ := c.Get("k2")
	if found1 || found2 {
		t.Error("expected cache to be fully cleared")
	}
}

func TestCacheTTLEviction(t *testing.T) {
	c := cache.NewInMemoryCache(50 * time.Millisecond)

	// Set key with 100ms TTL
	err := c.Set("ttl_key", "ttl_val", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key immediately exists
	val, found, _ := c.Get("ttl_key")
	if !found || val != "ttl_val" {
		t.Errorf("expected key to exist, got %v", val)
	}

	// Wait 150ms
	time.Sleep(150 * time.Millisecond)
	c.EvictExpired() // manually trigger eviction

	// Verify key is gone
	_, found, _ = c.Get("ttl_key")
	if found {
		t.Error("expected key to be evicted after TTL expiration")
	}
}

func TestCacheServerRESTAPI(t *testing.T) {
	c := cache.NewInMemoryCache(10 * time.Second)
	srv := server.NewServer(c)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Post SET request
	payload := map[string]interface{}{
		"key":   "api_key",
		"value": "api_value",
		"ttl":   "10s",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/api/cache", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post set request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// 2. Get request
	getResp, err := http.Get(ts.URL + "/api/cache/api_key")
	if err != nil {
		t.Fatalf("failed to query get: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", getResp.StatusCode)
	}

	var resData map[string]interface{}
	_ = json.NewDecoder(getResp.Body).Decode(&resData)
	if resData["value"] != "api_value" {
		t.Errorf("expected value 'api_value', got '%v'", resData["value"])
	}

	// 3. Delete key
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/cache/api_key", nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	delResp.Body.Close()

	// 4. Verify 404
	getResp2, _ := http.Get(ts.URL + "/api/cache/api_key")
	getResp2.Body.Close()
	if getResp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", getResp2.StatusCode)
	}
}
