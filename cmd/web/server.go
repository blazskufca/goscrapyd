package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultIdleTimeout    = time.Minute
	defaultReadTimeout    = time.Minute
	defaultWriteTimeout   = time.Minute
	defaultShutdownPeriod = 30 * time.Second
)

const (
	letsEncryptStagingCA    = "https://acme-staging-v02.api.letsencrypt.org/directory"
	letsEncryptProductionCA = "https://acme-v02.api.letsencrypt.org/directory"
)

func (app *application) serveHTTP() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.httpPort),
		Handler:      app.routes(),
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
		IdleTimeout:  defaultIdleTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	err := app.serve(srv)
	if err != nil {
		return err
	}

	app.wg.Wait()
	return nil
}

func (app *application) serveAutoHTTPS() error {
	if app.config.autoHTTPS.domain == "localhost" || strings.HasPrefix(app.config.autoHTTPS.domain, "localhost:") {
		return errors.New("auto HTTPS domain must be publicly accessible (not localhost)")
	}

	var directoryURL string

	if app.config.autoHTTPS.staging {
		directoryURL = letsEncryptStagingCA
	} else {
		directoryURL = letsEncryptProductionCA
	}

	certManager := autocert.Manager{
		Email:      app.config.autoHTTPS.email,
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache("./certs"),
		HostPolicy: autocert.HostWhitelist(app.config.autoHTTPS.domain),
		Client: &acme.Client{
			DirectoryURL: directoryURL,
		},
	}

	serverErrorChan := make(chan error)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		tlsConfig := certManager.TLSConfig()
		tlsConfig.MinVersion = tls.VersionTLS12
		tlsConfig.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256}

		srv := &http.Server{
			Addr:         ":443",
			Handler:      app.routes(),
			ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
			IdleTimeout:  defaultIdleTimeout,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			TLSConfig:    tlsConfig,
		}

		serverErrorChan <- app.serve(srv)
	}()

	go func() {
		defer wg.Done()

		srv := &http.Server{
			Addr:         ":80",
			Handler:      certManager.HTTPHandler(nil),
			ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
			IdleTimeout:  defaultIdleTimeout,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
		}

		serverErrorChan <- app.serve(srv)
	}()

	go func() {
		wg.Wait()
		close(serverErrorChan)
	}()

	for err := range serverErrorChan {
		if err != nil {
			return err
		}
	}

	app.wg.Wait()
	return nil
}

func (app *application) serve(srv *http.Server) error {
	shutdownErrorChan := make(chan error)

	go func() {
		quitChan := make(chan os.Signal, 1)
		signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)
		<-quitChan

		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownPeriod)
		defer cancel()

		shutdownErrorChan <- srv.Shutdown(ctx)
	}()

	app.logger.Info("starting server", slog.Group("server", "addr", srv.Addr))

	if srv.TLSConfig != nil {
		err := srv.ListenAndServeTLS("", "")
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	} else {
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	err := <-shutdownErrorChan
	if err != nil {
		return err
	}
	defer func() {
		err := app.DB.queries.Close()
		if err != nil {
			app.logger.Warn("failed to close database connection", "error", err)
		}
	}()
	err = app.scheduler.Shutdown()
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", slog.Group("server", "addr", srv.Addr))

	return nil
}
