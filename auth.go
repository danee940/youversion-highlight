package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtPayload struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

func parseToken(token string) (userID string, exp time.Time, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", time.Time{}, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims jwtPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", time.Time{}, fmt.Errorf("parsing JWT payload: %w", err)
	}
	if claims.Sub == "" {
		return "", time.Time{}, fmt.Errorf("no sub claim in token")
	}
	return claims.Sub, time.Unix(claims.Exp, 0), nil
}

func tokenIsExpired(exp time.Time) bool {
	return time.Now().After(exp)
}
