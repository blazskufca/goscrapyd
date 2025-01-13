package main

import (
	"context"
	"database/sql"
	"errors"
	"expvar"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/response"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/google/uuid"
	"github.com/justinas/nosurf"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	totalRequestsReceived           = expvar.NewInt("total_requests_received")
	totalResponsesSent              = expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
	totalResponsesSentByStatus      = expvar.NewMap("total_responses_sent_by_status")
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				app.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		next.ServeHTTP(w, r)
	})
}

func (app *application) logAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := response.NewMetricsResponseWriter(w)
		next.ServeHTTP(mw, r)

		var (
			ip         = realip.FromRequest(r)
			method     = r.Method
			requestUrl = r.URL.String()
			proto      = r.Proto
		)

		userAttrs := slog.Group("user", "ip", ip)
		requestAttrs := slog.Group("request", "method", method, "url", requestUrl, "proto", proto)
		responseAttrs := slog.Group("repsonse", "status", mw.StatusCode, "size", mw.BytesCount)

		app.logger.Info("access", userAttrs, requestAttrs, responseAttrs)
	})
}

func (app *application) reverseProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
		defer cancel()
		node, err := app.DB.queries.GetNodeWithName(ctxwt, r.PathValue("node"))
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		parsedURL, err := url.Parse(node.Url)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if node.Username.Valid && validator.NotBlank(node.Username.String) && validator.NotBlank(app.config.ScrapydEncryptSecret) && node.Password != nil {
			decryptedPassword, err := decrypt(node.Password, app.config.ScrapydEncryptSecret)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			r.SetBasicAuth(node.Username.String, decryptedPassword)
		}
		expectedPrefix := fmt.Sprintf("/%s/scrapyd-backend", r.PathValue("node"))
		r.URL.Path = strings.TrimPrefix(r.URL.Path, expectedPrefix)
		r = r.WithContext(context.WithValue(r.Context(), backendUrl, parsedURL))
		r = r.WithContext(context.WithValue(r.Context(), xForwardedForPrefix, expectedPrefix))
		next.ServeHTTP(w, r)
	})
}

func (app *application) preventCSRF(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)

	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})

	return csrfHandler
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := app.sessionStore.Get(r, "session")
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		var found bool
		userID, ok := session.Values["userID"].(uuid.UUID)
		if ok {
			user, err := app.DB.queries.GetUserWithID(context.Background(), userID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					found = false
				} else {
					app.serverError(w, r, err)
					return
				}
			} else {
				found = true
			}
			if found {
				r = contextSetAuthenticatedUser(r, &user)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedUser := contextGetAuthenticatedUser(r)

		if authenticatedUser == nil {
			session, err := app.sessionStore.Get(r, "session")
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			session.Values["redirectPathAfterLogin"] = r.URL.Path

			err = session.Save(r, w)
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		w.Header().Add("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAnonymousUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedUser := contextGetAuthenticatedUser(r)

		if authenticatedUser != nil {
			http.Redirect(w, r, "/list-nodes", http.StatusSeeOther)
			return

		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)
	go func() {
		defer func() {
			err := recover()
			if err != nil {
				app.logger.Error("rateLimit panicked", slog.Any("recover", err))
			}
		}()
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.limiter.enabled {
			ip := realip.FromRequest(r)
			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}
			clients[ip].lastSeen = time.Now()
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.logger.Info("rate limit exceeded", slog.Any("ip", ip))
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requirePermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := contextGetAuthenticatedUser(r)
		if !user.HasAdminPrivileges {
			app.notPermittedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		totalRequestsReceived.Add(1)
		mw := response.NewMetricsResponseWriter(w)
		next.ServeHTTP(mw, r)
		totalResponsesSent.Add(1)
		totalResponsesSentByStatus.Add(strconv.Itoa(mw.StatusCode), 1)
		duration := time.Since(start).Microseconds()
		totalProcessingTimeMicroseconds.Add(duration)
	})
}
