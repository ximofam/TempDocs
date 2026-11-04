// SPDX-License-Identifier: Apache-2.0
package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestVerifyRejectsExpiredForgedAndMissingClaims(t *testing.T) {
	a := Auth{Key: []byte("test-key-with-at-least-thirty-two-bytes")}
	id := Identity{"alice", "ws-a", "employee"}
	token, err := a.Sign(id, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Verify(token)
	if err != nil || got != id {
		t.Fatalf("got %v, %v", got, err)
	}
	expired, _ := a.Sign(id, -time.Minute)
	other := Auth{Key: []byte("different-key-with-at-least-thirty-two-bytes")}
	forged, _ := other.Sign(id, time.Hour)
	missing, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": "dxos-demo", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(a.Key)
	wrongAlg, _ := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{"iss": "dxos-demo", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(a.Key)
	for _, tok := range []string{expired, forged, missing, wrongAlg, "garbage"} {
		if _, err := a.Verify(tok); err == nil {
			t.Fatal("accepted invalid token")
		}
	}
	for _, bad := range []Identity{{"", "ws-a", "employee"}, {"alice", "", "employee"}, {"alice", "ws-a", "admin"}} {
		if _, err := a.Sign(bad, time.Hour); err == nil {
			t.Fatal("signed invalid identity")
		}
	}
}
