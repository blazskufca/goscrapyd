package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func proxyRewriter(pr *httputil.ProxyRequest) {
	backendURL := pr.In.Context().Value(backendUrl).(*url.URL)
	pr.Out.URL.Host = backendURL.Host
	pr.Out.URL.Scheme = backendURL.Scheme
	pr.Out.Host = backendURL.Host
	pr.Out.Header.Set("x-forwarded-prefix", pr.In.Context().Value(xForwardedForPrefix).(string))
}

func (app *application) reverseProxyErrHandler(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		app.serverError(w, r, err)
	}
}
