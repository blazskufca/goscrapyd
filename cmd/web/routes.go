package main

import (
	"expvar"
	"github.com/blazskufca/goscrapyd/assets"
	"github.com/justinas/alice"
	"net/http"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	appMiddleware := alice.New(app.authenticate, app.rateLimit, app.logAccess)
	reverseProxyMiddleware := alice.New(app.authenticate, app.logAccess)
	// Authenticated, access logged, CSRF protected routes
	mux.Handle("GET /add-task", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.createNewTask))
	mux.Handle("POST /add-task", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.createNewTask))
	mux.Handle("GET /add-user", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.addNewUser))
	mux.Handle("POST /add-user", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.addNewUser))
	mux.Handle("POST /bulk-update-tasks", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.doBulkAction))
	mux.Handle("GET /deploy-project", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.deploy))
	mux.Handle("POST /deploy-project", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.deploy))
	mux.Handle("GET /fire-spider", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.fireSpider))
	mux.Handle("POST /fire-spider", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.fireSpider))
	mux.Handle("GET /task/edit/{taskUUID}", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.editTask))
	mux.Handle("POST /task/edit/{taskUUID}", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.editTask))
	mux.Handle("GET /list-tasks", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.listTasks))
	// Authenticated, access logged, but not CSRF protected
	mux.Handle("GET /htmx-list-online-nodes", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.htmxListOnlineNodes))
	mux.Handle("GET /list-nodes", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.listScrapydNodes))
	mux.Handle("GET /{node}/jobs", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.nodeJobs))
	mux.Handle("DELETE /delete-job/{jobId}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.deleteJob))
	mux.Handle("POST /fire-task/{jobUUID}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.fireTask))
	mux.Handle("DELETE /stop-task/{taskUUID}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.stopTask))
	mux.Handle("POST /restart-task/{taskUUID}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.restartTask))
	mux.Handle("DELETE /delete-task/{taskUUID}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.deleteTask))
	mux.Handle("POST /task/search", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.searchTasksTable))
	mux.Handle("GET /job/view-logs/{jobId}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.viewJobLogs))
	mux.Handle("GET /deploy-sse", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser).ThenFunc(app.buildAndDeployEggSSE))
	mux.Handle("GET /logout", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.logout))
	mux.Handle("GET /htmx-fire-form", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.htmxFireForm))
	mux.Handle("DELETE /{node}/stop-job/{project}/{job}", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.stopJob))
	mux.Handle("GET /{node}/scrapyd-backend/", reverseProxyMiddleware.Append(app.requireAuthenticatedUser, app.reverseProxyMiddleware).Then(app.reverseProxy))
	mux.Handle("POST /{node}/scrapyd-backend/", reverseProxyMiddleware.Append(app.requireAuthenticatedUser, app.reverseProxyMiddleware).Then(app.reverseProxy))
	mux.Handle("POST /{node}/job/search", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.searchJobs))
	mux.Handle("GET /versions", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.listVersions))
	mux.Handle("GET /versions-htmx", appMiddleware.Append(app.requireAuthenticatedUser).ThenFunc(app.listVersionsHTMX))
	// This should come last because it's the general root route.
	mux.Handle("GET /", appMiddleware.Append(app.requireAuthenticatedUser).Then(http.RedirectHandler("/list-nodes", http.StatusMovedPermanently)))
	// Admin only routes
	mux.Handle("GET /add-node", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.insertNewScrapydNode))
	mux.Handle("POST /add-node", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.insertNewScrapydNode))
	mux.Handle("GET /edit-settings", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.settingPage))
	mux.Handle("POST /edit-settings", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.settingPage))
	mux.Handle("GET /list-users", appMiddleware.Append(app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.listsUsers))
	mux.Handle("DELETE /user/delete/{userID}", appMiddleware.Append(app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.deleteUser))
	mux.Handle("GET /user/edit/{userID}", appMiddleware.Append(app.preventCSRF, app.requirePermission, app.requirePermission).ThenFunc(app.updateUser))
	mux.Handle("POST /user/edit/{userID}", appMiddleware.Append(app.preventCSRF, app.requirePermission, app.requirePermission).ThenFunc(app.updateUser))
	mux.Handle("DELETE /delete-node/{node}", appMiddleware.Append(app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.deleteScrapydNode))
	mux.Handle("GET /node/edit/{node}", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.editNode))
	mux.Handle("POST /node/edit/{node}", appMiddleware.Append(app.preventCSRF, app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.editNode))
	mux.Handle("GET /metrics", appMiddleware.Append(app.requireAuthenticatedUser, app.requirePermission).ThenFunc(app.metricsHandler))
	mux.Handle("GET /metrics/json", appMiddleware.Append(app.requireAuthenticatedUser, app.requirePermission).Then(expvar.Handler()))
	// Anonymous user routes
	mux.Handle("GET /login", appMiddleware.Append(app.preventCSRF, app.requireAnonymousUser).ThenFunc(app.login))
	mux.Handle("POST /login", appMiddleware.Append(app.preventCSRF, app.requireAnonymousUser).ThenFunc(app.login))
	fileServer := http.FileServer(http.FS(assets.EmbeddedFiles))
	mux.Handle("GET /ui/static/", http.StripPrefix("/ui", fileServer))
	defaultMiddleware := alice.New(app.metricsMiddleware, app.recoverPanic, app.securityHeaders)
	return defaultMiddleware.Then(mux)
}
