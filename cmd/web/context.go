package main

import (
	"context"
	"github.com/blazskufca/goscrapyd/internal/database"
	"net/http"
)

type contextKey string

const (
	authenticatedUserContextKey = contextKey("authenticatedUser")
	backendUrl                  = contextKey("backendURL")
	xForwardedForPrefix         = contextKey("xForwardedForPrefix")
)

func contextSetAuthenticatedUser(r *http.Request, user *database.User) *http.Request {
	if r == nil {
		return nil
	}
	ctx := context.WithValue(r.Context(), authenticatedUserContextKey, user)
	return r.WithContext(ctx)
}

func contextGetAuthenticatedUser(r *http.Request) *database.User {
	user, ok := r.Context().Value(authenticatedUserContextKey).(*database.User)
	if !ok {
		return nil
	}

	return user
}
