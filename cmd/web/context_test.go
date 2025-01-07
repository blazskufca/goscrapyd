package main

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/google/uuid"
	"net/http"
	"testing"
	"time"
)

func TestContext(t *testing.T) {
	t.Run("Context set/get user", func(t *testing.T) {
		user := database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "testUser",
			HashedPassword:     "hashHere",
			HasAdminPrivileges: true,
		}
		req, err := http.NewRequest(http.MethodGet, "/users/"+user.Username, nil)
		if err != nil {
			t.Fatal(err)
		}
		req = contextSetAuthenticatedUser(req, &user)
		gotUser := contextGetAuthenticatedUser(req)
		assert.Equal(t, user.Username, gotUser.Username)
		assert.Equal(t, user.HashedPassword, gotUser.HashedPassword)
		assert.Equal(t, user.HasAdminPrivileges, gotUser.HasAdminPrivileges)
		assert.Equal(t, user.ID, gotUser.ID)
		assert.Equal(t, user.CreatedAt, gotUser.CreatedAt)
	})
	t.Run("Context set/get nil user", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/users/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req = contextSetAuthenticatedUser(req, nil)
		gotUser := contextGetAuthenticatedUser(req)
		assert.Equal(t, gotUser, nil)
	})
	t.Run("Context set/get user with nil request", func(t *testing.T) {
		user := database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "testUser",
			HashedPassword:     "hashHere",
			HasAdminPrivileges: true,
		}
		req := contextSetAuthenticatedUser(nil, &user)
		assert.Equal(t, req, nil)
	})
}
