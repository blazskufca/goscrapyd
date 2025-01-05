package main

import (
	"context"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/password"
	"github.com/google/uuid"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestUserLogin(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	tests := []struct {
		name         string
		username     string
		password     string
		wantCode     int
		wantLocation string
		wantCookie   string
		wantBody     string
	}{
		{
			name:         "Valid Credentials",
			username:     "admin",
			password:     "admin",
			wantCode:     303,
			wantLocation: "/list-nodes",
			wantCookie:   "session=",
		},
		{
			name:     "Invalid Username",
			username: "wronguser",
			password: "password",
			wantCode: 422,
			wantBody: "User by that username could not be found",
		},
		{
			name:     "Invalid Password",
			username: "admin",
			password: "wrongpassword",
			wantCode: 422,
		},
		{
			name:     "Empty Credentials",
			username: "",
			password: "",
			wantCode: 422,
		},
		{
			name:     "SQL Injection Attempt",
			username: "admin' OR '1'='1",
			password: "' OR '1'='1",
			wantCode: 422,
		},
		{
			name:     "Very Long Credentials",
			username: strings.Repeat("a", 1000),
			password: strings.Repeat("b", 1000),
			wantCode: 422,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			ts.Client().Jar = jar

			code, _, body := ts.get(t, "/login")
			if code != 200 {
				t.Fatalf("got status %d for GET /login; want 200", code)
			}

			csrfToken := extractCSRFToken(t, body)
			if csrfToken == "" {
				t.Fatal("CSRF token not found in response")
			}

			form := url.Values{}
			form.Add("csrf_token", csrfToken)
			form.Add("username", tt.username)
			form.Add("password", tt.password)

			code, headers, body := ts.postForm(t, "/login", form)

			if code != tt.wantCode {
				t.Errorf("got status %d; want %d", code, tt.wantCode)
			}

			if tt.wantLocation != "" {
				location := headers.Get("Location")
				if location != tt.wantLocation {
					t.Errorf("got location %s; want %s", location, tt.wantLocation)
				}
			}
			if tt.wantCookie != "" {
				cookie := headers.Get("Set-Cookie")
				if !strings.Contains(cookie, tt.wantCookie) {
					t.Errorf("cookie %q doesn't contain %q", cookie, tt.wantCookie)
				}
			}

			if tt.wantBody != "" && !strings.Contains(body, tt.wantBody) {
				t.Errorf("response body doesn't contain %q", tt.wantBody)
			}
		})
	}
}

func TestRequireAuthenticatedUser(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	tests := []struct {
		name         string
		path         string
		wantCode     int
		wantLocation string
	}{
		{
			name:     "Visit list nodes",
			path:     "/list-nodes",
			wantCode: http.StatusOK,
		},
		{
			name:     "Visit Tasks",
			wantCode: http.StatusOK,
			path:     "/list-tasks",
		},
		{
			name:     "Visit Single Fire Page",
			wantCode: http.StatusOK,
			path:     "/fire-spider",
		},
		{
			name:     "Visit Deploy spider",
			wantCode: http.StatusOK,
			path:     "/deploy-project",
		},
		{
			name:     "Visit Versions",
			wantCode: http.StatusOK,
			path:     "/versions",
		},
		{
			name:         "Visit login again",
			path:         "/login",
			wantCode:     http.StatusSeeOther,
			wantLocation: "/list-nodes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, headers, _ := ts.get(t, tt.path)
			if code != tt.wantCode {
				t.Fatalf("got status %d; want %d", code, tt.wantCode)
			}
			if tt.wantLocation != "" {
				location := headers.Get("Location")
				if location != tt.wantLocation {
					t.Fatalf("got location %s; want %s", location, tt.wantLocation)
				}
			}
		})
	}
}

func TestAddNewUser(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	tests := []struct {
		name            string
		username        string
		password        string
		passwordConfirm string
		isAdmin         bool
		wantCode        int
		wantBody        string
	}{
		{
			name:            "Test Add New User",
			username:        "TestUser",
			password:        "ThisIsAVerySecurePasswordA$$word",
			passwordConfirm: "ThisIsAVerySecurePasswordA$$word",
			isAdmin:         false,
			wantCode:        http.StatusOK,
			wantBody:        "TestUser",
		},
		{
			name:            "Test Add New User Admin",
			username:        "AdminUser",
			password:        "ThisIsAVerySecurePasswordA$$word",
			passwordConfirm: "ThisIsAVerySecurePasswordA$$word",
			isAdmin:         true,
			wantCode:        http.StatusOK,
			wantBody:        "Yes",
		},
		{
			name:            "Test No Password",
			username:        "NoPassword",
			password:        "",
			passwordConfirm: "",
			isAdmin:         false,
			wantCode:        http.StatusUnprocessableEntity,
			wantBody:        "Password is required",
		},
		{
			name:            "Test Wrong Password Confirm",
			username:        "TestUser",
			password:        "ThisIsAVerySecurePasswordA$$word",
			passwordConfirm: "This is not",
			isAdmin:         false,
			wantCode:        http.StatusUnprocessableEntity,
			wantBody:        "Passwords do not match",
		},
		{
			name:            "Test No Username",
			username:        "",
			password:        "ThisIsAVerySecurePasswordA$$word",
			passwordConfirm: "ThisIsAVerySecurePasswordA$$word",
			isAdmin:         false,
			wantCode:        http.StatusUnprocessableEntity,
			wantBody:        "Username is required",
		},
		{
			name:            "Test Insecure password",
			username:        "TestUser",
			password:        "password",
			passwordConfirm: "password",
			isAdmin:         false,
			wantCode:        http.StatusUnprocessableEntity,
			wantBody:        "Password is too common",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, body := ts.get(t, "/add-user")
			if code != http.StatusOK {
				t.Fatalf("got status %d for GET /add-user; want 200", code)
			}
			csrfToken := extractCSRFToken(t, body)
			if csrfToken == "" {
				t.Fatal("CSRF token not found in response")
			}
			form := url.Values{}
			form.Add("username", tt.username)
			form.Add("password", tt.password)
			form.Add("csrf_token", csrfToken)
			form.Add("password_confirm", tt.passwordConfirm)
			form.Add("grant_admin", strconv.FormatBool(tt.isAdmin))
			status, _, body := ts.postFormFollowRedirects(t, "/add-user", form)
			if status != tt.wantCode {
				t.Fatalf("got status %d; want %d", status, tt.wantCode)
			}
			if tt.wantBody != "" {
				assert.StringContains(t, body, tt.wantBody)
			}

		})
	}
}

func TestEditUser(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	var user database.User
	t.Run("Create User", func(t *testing.T) {
		userUUID := uuid.New()
		hashedPassword, err := password.Hash("ThisIsAVerySecurePasswordA$$word")
		if err != nil {
			t.Fatal(err)
		}
		user, err = app.DB.queries.CreateNewUser(context.Background(), database.CreateNewUserParams{
			ID:                 userUUID,
			Username:           "TestUser",
			HashedPassword:     hashedPassword,
			HasAdminPrivileges: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Edit User", func(t *testing.T) {
		code, _, body := ts.get(t, "/user/edit/"+user.ID.String())
		if code != http.StatusOK {
			t.Fatalf("got status %d for GET /add-user; want 200", code)
		}
		csrfToken := extractCSRFToken(t, body)
		if csrfToken == "" {
			t.Fatal("CSRF token not found in response")
		}
		form := url.Values{}
		form.Add("username", "ThisIsEditedUser")
		form.Add("password", "")
		form.Add("csrf_token", csrfToken)
		form.Add("password_confirm", "")
		form.Add("grant_admin", strconv.FormatBool(user.HasAdminPrivileges))
		status, _, body := ts.postFormFollowRedirects(t, "/user/edit/"+user.ID.String(), form)
		if status != http.StatusOK {
			t.Fatalf("got status %d; want %d", status, http.StatusOK)
		}
		assert.StringContains(t, body, "ThisIsEditedUser")
		assert.StringDoesNotContain(t, body, "TestUser")
	})
}

func TestDeleteUser(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	var user database.User
	t.Run("Create User", func(t *testing.T) {
		userUUID := uuid.New()
		hashedPassword, err := password.Hash("ThisIsAVerySecurePasswordA$$word")
		if err != nil {
			t.Fatal(err)
		}
		user, err = app.DB.queries.CreateNewUser(context.Background(), database.CreateNewUserParams{
			ID:                 userUUID,
			Username:           "TestUser",
			HashedPassword:     hashedPassword,
			HasAdminPrivileges: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Delete Existent User", func(t *testing.T) {
		code, _, _ := ts.delete(t, "/user/delete/"+user.ID.String())
		if code != http.StatusOK {
			t.Fatalf("got status %d; want %d", code, http.StatusOK)
		}
	})
	t.Run("Deleted users does no exist anymore", func(t *testing.T) {
		code, _, body := ts.get(t, "/list-users")
		if code != http.StatusOK {
			t.Fatalf("got status %d; want %d", code, http.StatusOK)
		}
		assert.StringDoesNotContain(t, body, "TestUser")
	})
	t.Run("Delete non-existent user", func(t *testing.T) {
		fakeUUID, err := uuid.NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		code, _, _ := ts.delete(t, "/user/delete/"+fakeUUID.String())
		if code != http.StatusOK {
			t.Fatalf("got status %d; want %d", code, http.StatusOK)
		}
	})
	t.Run("Delete non-UUID", func(t *testing.T) {
		code, _, _ := ts.delete(t, "/user/delete/not-a-uuid")
		if code != http.StatusBadRequest {
			t.Fatalf("got status %d; want %d", code, http.StatusBadRequest)
		}
	})
}
