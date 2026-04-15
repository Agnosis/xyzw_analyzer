package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"xyzw_study/internal/proxy"
)

func setupTestAuthDataPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	authDataFilePath = filepath.Join(dir, "auth_data.txt")
}

func TestHandleGetAuthData_Empty(t *testing.T) {
	setupTestAuthDataPath(t)
	proxy.AuthData = ""

	req := httptest.NewRequest(http.MethodGet, "/api/auth/data", nil)
	w := httptest.NewRecorder()
	HandleGetAuthData(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["hasData"] != false {
		t.Errorf("expected hasData=false, got %v", resp["hasData"])
	}
}

func TestHandleGetAuthData_WithData(t *testing.T) {
	setupTestAuthDataPath(t)
	proxy.AuthData = "4f70deadbeef"

	req := httptest.NewRequest(http.MethodGet, "/api/auth/data", nil)
	w := httptest.NewRecorder()
	HandleGetAuthData(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["hasData"] != true {
		t.Errorf("expected hasData=true, got %v", resp["hasData"])
	}
	if resp["length"].(float64) != float64(len("4f70deadbeef")) {
		t.Errorf("expected correct length")
	}
}

func TestHandleSetAuthData_Valid(t *testing.T) {
	setupTestAuthDataPath(t)
	proxy.AuthData = ""

	body, _ := json.Marshal(map[string]string{"authData": "deadbeef1234"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	HandleSetAuthData(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if proxy.AuthData != "deadbeef1234" {
		t.Errorf("proxy.AuthData not set, got %q", proxy.AuthData)
	}
	data, err := os.ReadFile(authDataFilePath)
	if err != nil || string(data) != "deadbeef1234" {
		t.Errorf("file not written correctly: %v %s", err, data)
	}
}

func TestHandleSetAuthData_EmptyBody(t *testing.T) {
	setupTestAuthDataPath(t)
	proxy.AuthData = "existing"

	req := httptest.NewRequest(http.MethodPost, "/api/auth/data", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	HandleSetAuthData(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty authData, got %d", w.Code)
	}
}

func TestLoadAuthDataFromFile(t *testing.T) {
	setupTestAuthDataPath(t)
	proxy.AuthData = ""

	os.WriteFile(authDataFilePath, []byte("cafebabe"), 0644)
	LoadAuthDataFromFile()

	if proxy.AuthData != "cafebabe" {
		t.Errorf("expected cafebabe, got %q", proxy.AuthData)
	}
}

func TestSaveAuthData(t *testing.T) {
	setupTestAuthDataPath(t)
	SaveAuthData("aabbcc")

	data, _ := os.ReadFile(authDataFilePath)
	if string(data) != "aabbcc" {
		t.Errorf("expected aabbcc, got %s", data)
	}
}
