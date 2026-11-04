// SPDX-License-Identifier: Apache-2.0
package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

const Namespace = "https://hasura.io/jwt/claims"

type Identity struct{ UserID, WorkspaceID, Role string }
type Auth struct{ Key []byte }

func DemoIdentity(name string) (Identity, error) {
	ids := map[string]Identity{"alice": {"alice", "ws-a", "employee"}, "bob": {"bob", "ws-b", "employee"}, "manager-a": {"manager-a", "ws-a", "manager"}, "manager-b": {"manager-b", "ws-b", "manager"}}
	id, ok := ids[name]
	if !ok {
		return id, errors.New("unknown demo identity")
	}
	return id, nil
}
func valid(i Identity) bool {
	return i.UserID != "" && (i.WorkspaceID == "ws-a" || i.WorkspaceID == "ws-b") && (i.Role == "employee" || i.Role == "manager")
}
func (a Auth) Sign(i Identity, ttl time.Duration) (string, error) {
	if !valid(i) {
		return "", errors.New("invalid identity")
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": "dxos-demo", "sub": i.UserID, "exp": time.Now().Add(ttl).Unix(), Namespace: map[string]any{"x-hasura-default-role": i.Role, "x-hasura-allowed-roles": []string{i.Role}, "x-hasura-user-id": i.UserID, "x-hasura-workspace-id": i.WorkspaceID}}).SignedString(a.Key)
}
func (a Auth) Verify(raw string) (Identity, error) {
	var i Identity
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) { return a.Key, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("dxos-demo"), jwt.WithExpirationRequired())
	if err != nil {
		return i, err
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return i, errors.New("invalid claims")
	}
	h, ok := claims[Namespace].(map[string]any)
	if !ok {
		return i, errors.New("missing hasura claims")
	}
	i.UserID, _ = h["x-hasura-user-id"].(string)
	i.WorkspaceID, _ = h["x-hasura-workspace-id"].(string)
	i.Role, _ = h["x-hasura-default-role"].(string)
	sub, _ := claims.GetSubject()
	roles, ok := h["x-hasura-allowed-roles"].([]any)
	if !valid(i) || sub != i.UserID || !ok || len(roles) != 1 || roles[0] != i.Role {
		return Identity{}, errors.New("invalid identity claims")
	}
	return i, nil
}
