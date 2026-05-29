package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func makeJWT(sub string, exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{"sub": sub, "exp": exp})
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s.%s.fakesig", header, encodedPayload)
}

func TestParseToken_valid(t *testing.T) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	token := makeJWT("user123", exp)

	userID, gotExp, err := parseToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "user123" {
		t.Errorf("userID = %q, want %q", userID, "user123")
	}
	if gotExp.Unix() != exp {
		t.Errorf("exp = %d, want %d", gotExp.Unix(), exp)
	}
}

func TestParseToken_wrongPartCount(t *testing.T) {
	_, _, err := parseToken("only.two")
	if err == nil {
		t.Fatal("expected error for token with only 2 parts")
	}
}

func TestParseToken_invalidBase64(t *testing.T) {
	_, _, err := parseToken("header.!!!notbase64!!!.sig")
	if err == nil {
		t.Fatal("expected error for invalid base64 payload")
	}
}

func TestParseToken_invalidJSON(t *testing.T) {
	badPayload := base64.RawURLEncoding.EncodeToString([]byte(`not json`))
	token := fmt.Sprintf("header.%s.sig", badPayload)
	_, _, err := parseToken(token)
	if err == nil {
		t.Fatal("expected error for non-JSON payload")
	}
}

func TestParseToken_missingSub(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":9999999999}`))
	token := fmt.Sprintf("header.%s.sig", payload)
	_, _, err := parseToken(token)
	if err == nil {
		t.Fatal("expected error when sub claim is missing")
	}
}

func TestTokenIsExpired_future(t *testing.T) {
	if tokenIsExpired(time.Now().Add(time.Hour)) {
		t.Error("future time should not be expired")
	}
}

func TestTokenIsExpired_past(t *testing.T) {
	if !tokenIsExpired(time.Now().Add(-time.Hour)) {
		t.Error("past time should be expired")
	}
}

func TestTokenIsExpired_justNow(t *testing.T) {
	if !tokenIsExpired(time.Now().Add(-time.Second)) {
		t.Error("one second ago should be expired")
	}
}
