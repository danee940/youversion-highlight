package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHandleStatus_validToken(t *testing.T) {
	exp := time.Now().Add(48 * time.Hour)
	state.set("tok", "user1", exp)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["expired"] != false {
		t.Errorf("expired = %v, want false", body["expired"])
	}
	if _, ok := body["expires_at"]; !ok {
		t.Error("missing expires_at field")
	}
	if _, ok := body["expires_in"]; !ok {
		t.Error("missing expires_in field")
	}
}

func TestHandleStatus_expiredToken(t *testing.T) {
	state.set("tok", "user1", time.Now().Add(-time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	handleStatus(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["expired"] != true {
		t.Errorf("expired = %v, want true", body["expired"])
	}
}

func TestHandleRandom_expiredToken(t *testing.T) {
	state.set("tok", "user1", time.Now().Add(-time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/random", nil)
	w := httptest.NewRecorder()
	handleRandom(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "token expired" {
		t.Errorf("error = %q, want %q", body["error"], "token expired")
	}
	if body["refresh_url"] != "/refresh" {
		t.Errorf("refresh_url = %q, want %q", body["refresh_url"], "/refresh")
	}
}

func TestHandleRefresh_getServesForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/refresh", nil)
	w := httptest.NewRecorder()
	handleRefresh(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("unexpected status %d", w.Code)
	}
}

func TestHandleRefresh_postEmptyToken(t *testing.T) {
	form := url.Values{}
	form.Set("token", "")
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handleRefresh(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRefresh_postInvalidToken(t *testing.T) {
	form := url.Values{}
	form.Set("token", "this.is.notvalid")
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handleRefresh(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRefresh_postValidToken(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	token := makeJWT("newuser", exp)

	form := url.Values{}
	form.Set("token", token)
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handleRefresh(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (redirect)", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}

	yva, userID, _ := state.get()
	if yva != token {
		t.Errorf("stored yva = %q, want %q", yva, token)
	}
	if userID != "newuser" {
		t.Errorf("stored userID = %q, want %q", userID, "newuser")
	}
}

func TestWriteJSON_setsContentTypeAndStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["key"] != "val" {
		t.Errorf("body[key] = %q, want %q", body["key"], "val")
	}
}

func TestApplyToken_valid(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	token := makeJWT("applyuser", exp)

	if err := applyToken(token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, userID, _ := state.get()
	if userID != "applyuser" {
		t.Errorf("userID = %q, want %q", userID, "applyuser")
	}
}

func TestApplyToken_invalid(t *testing.T) {
	if err := applyToken("bad.token"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}
