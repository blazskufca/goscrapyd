package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/request"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/go-co-op/gocron/v2"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
)

type templateName string

// HTML pages
const (
	addNodePage            templateName = "add_node.tmpl"
	listNodesPage          templateName = "list_nodes.tmpl"
	listNodeJobs           templateName = "list_node_jobs.tmpl"
	fireSpiderPage         templateName = "fire_spider_one_time.tmpl"
	addTaskPage            templateName = "add_tasks.tmpl"
	addedTaskPage          templateName = "added_task.tmpl"
	allTasksPage           templateName = "all_tasks.tmpl"
	editTaskPage           templateName = "task_edit.tmpl"
	htmxTaskTable          templateName = "htmx_task_table.tmpl"
	htmxListOfItems        templateName = "htmx_list_of_items.tmpl"
	htmxParagraph          templateName = "htmx_just_paragraph.tmpl"
	firedSpiderResultPage  templateName = "fired_spider_one_time.tmpl"
	htmxJobsTable          templateName = "htmx_jobs_table.tmpl"
	jobLogsPage            templateName = "job_logs.tmpl"
	nodeEditPage           templateName = "edit_node.tmpl"
	deployPage             templateName = "deploy.tmpl"
	deployInProgressPage   templateName = "deploying_nodes.tmpl"
	buildFailedSSE         templateName = "build_failed.tmpl"
	deploymentDoneSSE      templateName = "deploy_done.tmpl"
	justTemplateDataSSE    templateName = "template_data.tmpl"
	deployErrorSSE         templateName = "deploy_error.tmpl"
	deployAlreadyLockedSSE templateName = "locked_for_deploy.tmpl"
	usersListPage          templateName = "users.tmpl"
	addUserFormPage        templateName = "add_user_form.tmpl"
	loginPage              templateName = "login.tmpl"
	settingsPage           templateName = "settings_page.tmpl"
	editUserPage           templateName = "edit_user_form.tmpl"
	htmxListNodes          templateName = "htmx_nodes_list.tmpl"
	versionsPage           templateName = "versions.tmpl"
	versionsPageHtmx       templateName = "htmx_versions.tmpl"
)

// Other various misc strings
const (
	scrapydUniqueConstraintErr string = "Node with name %s and URL %s already exists"
)

// Scrapyd endpoints/paths
const (
	ScrapydDaemonStatusReq string = "daemonstatus.json"
	ScrapydListProjectsReq string = "listprojects.json"
	ScrapydListSpidersReq  string = "listspiders.json"
	ScrapydListJobsReq     string = "listjobs.json"
	ScrapydLogStatsReq     string = "/logs/stats.json"
	ScrapydScheduleSpider  string = "schedule.json"
	ScrapydAddVersion      string = "addversion.json"
	ScrapydStopSpider      string = "cancel.json"
	ScrapydListVersions    string = "listversions.json"
)

var (
	errScrapydTableUniqueConstraint error = errors.New("UNIQUE constraint failed: scrapyd_nodes.nodeName, scrapyd_nodes.URL")
)

type listScrapydNodesType struct {
	Id       int64
	Pending  int
	Running  int
	Finished int
	Name     string
	URL      string
	Status   string
	Error    error
}

type scrapydListProjects struct {
	NodeName string   `json:"node_name"`
	Status   string   `json:"status"`
	Projects []string `json:"projects"`
}

type scrapydListSpidersResponse struct {
	NodeName string   `json:"node_name"`
	Status   string   `json:"status"`
	Spiders  []string `json:"spiders"`
}

type tasksSearchForm struct {
	SearchTerm string `form:"searchTerm"`
}

type editAddScrapydNode struct {
	NodeName  string              `form:"nodeName"`
	URL       string              `form:"url"`
	Username  *string             `form:"username"`
	Password  *string             `form:"password"`
	Validator validator.Validator `form:"-"`
}

func (app *application) insertNewScrapydNode(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		templateData := app.newTemplateData(r)
		app.render(w, r, http.StatusOK, addNodePage, nil, templateData)
	case http.MethodPost:
		var fd editAddScrapydNode
		err := request.DecodePostForm(r, &fd)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		fd.Validator.CheckField(validator.NotBlank(fd.NodeName), "nodeName", "You must provide a name for this node")
		fd.Validator.CheckField(validator.NotBlank(fd.URL), "URL", "You must provide a URL for this node")
		fd.Validator.CheckField(validator.IsURL(fd.URL), "URL", "Node URL must be a valid URL")
		if fd.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = fd
			app.render(w, r, http.StatusUnprocessableEntity, addNodePage, nil, data)
			return
		}
		cleanUrl, err := url.ParseRequestURI(fd.URL)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		cleanUrl.Path = "" // Remove any paths that might have been left in
		fd.URL = cleanUrl.String()
		dbQueryParams := database.NewScrapydNodeParams{
			Nodename: fd.NodeName,
			Url:      fd.URL,
			Username: database.CreateSqlNullString(fd.Username),
		}
		if fd.Username != nil && validator.NotBlank(*fd.Username) && fd.Password != nil {
			encryptedPassword, err := encrypt(*fd.Password, app.config.ScrapydEncryptSecret)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			dbQueryParams.Password = encryptedPassword
		}
		_, err = app.DB.queries.NewScrapydNode(ctxwt, dbQueryParams)
		if err != nil {
			if errors.As(err, &errScrapydTableUniqueConstraint) {
				data := app.newTemplateData(r)
				data["Form"] = fd
				data["UniqueViolation"] = fmt.Sprintf(scrapydUniqueConstraintErr, fd.NodeName, fd.URL)
				app.render(w, r, http.StatusUnprocessableEntity, addNodePage, nil, data)
			} else {
				app.serverError(w, r, err)
			}
			return
		}
		http.Redirect(w, r, "/list-nodes", http.StatusSeeOther)
	}
}

func (app *application) deleteScrapydNode(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	err := app.DB.queries.DeleteScrapydNodes(ctxwt, r.PathValue("node"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}

func (app *application) listScrapydNodes(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	var workerResults []listScrapydNodesType
	numJobs := len(nodes)
	jobs := make(chan database.ScrapydNode, numJobs)
	results := make(chan listScrapydNodesType, numJobs)
	for w := 0; w <= app.config.workerCount; w++ {
		go app.listScrapydNodesWorkerFunc(ctxwt, r, jobs, results)
	}
	for _, job := range nodes {
		jobs <- job
	}
	close(jobs)
	for range numJobs {
		workerResults = append(workerResults, <-results)
	}
	close(results)
	data := app.newTemplateData(r)
	// Resort the result of async workers
	// I don't know if it even makes sense to requests async and then do addition work sorting it back
	// I guess it almost certainly does since requests are way more expensive but I don't really like I'm doing extra work after the fact
	data["Nodes"] = sortScrapydNodes(workerResults)
	app.render(w, r, http.StatusOK, listNodesPage, nil, data)
}

func (app *application) nodeJobs(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	const pageSize = 1000
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	if page <= 0 {
		page = 1
	}
	if page == 1 {
		err := app.requestScrapydWorkInfo(ctxwt, r.PathValue("node"))
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}
	offset := (page - 1) * pageSize
	jobs, err := app.DB.queries.GetJobsForNode(ctxwt, database.GetJobsForNodeParams{
		Node:   r.PathValue("node"),
		Limit:  int64(pageSize),
		Offset: int64(offset),
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	totalNumberOfJobs, err := app.DB.queries.GetTotalJobCountForNode(ctxwt, r.PathValue("node"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	totalPages := int(math.Ceil(float64(totalNumberOfJobs) / float64(pageSize)))
	var errored, pending, running, finished []database.GetJobsForNodeRow
	for _, job := range jobs {
		switch job.Status {
		case "error":
			errored = append(errored, job)
		case "pending":
			pending = append(pending, job)
		case "running":
			running = append(running, job)
		case "finished":
			finished = append(finished, job)
		}
	}
	paginationPages := make([]int, totalPages)
	for i := 1; i <= totalPages; i++ {
		paginationPages[i-1] = i
	}
	data := app.newTemplateData(r)
	data["ErrorJobs"] = errored
	data["PendingJobs"] = pending
	data["RunningJobs"] = running
	data["FinishedJobs"] = finished
	data["NodeName"] = r.PathValue("node")
	data["CurrentPage"] = page
	data["TotalPages"] = totalPages
	data["NextPage"] = page + 1
	data["PrevPage"] = page - 1
	data["PaginationPages"] = paginationPages

	if data["NextPage"].(int) > totalPages {
		data["NextPage"] = nil
	}
	if data["PrevPage"].(int) < 1 {
		data["PrevPage"] = nil
	}
	app.render(w, r, http.StatusOK, listNodeJobs, nil, data)
}

func (app *application) deleteJob(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	err := app.DB.queries.SoftDeleteJob(ctxwt, database.SoftDeleteJobParams{
		Deleted: true,
		Job:     r.PathValue("jobId"),
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}

func (app *application) fireSpider(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.logger.ErrorContext(ctxwt, "failed to list nodes", slog.Any("error", err))
		return
	}
	preconfiguredSettings, err := app.getPreconfiguredSettings(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		tempData := app.newTemplateData(r)
		tempData["PreconfiguredSettings"] = preconfiguredSettings
		tempData["Nodes"] = nodes
		app.render(w, r, http.StatusOK, fireSpiderPage, nil, tempData)
	case http.MethodPost:
		fullQuery := struct {
			Project   string              `form:"project"`
			Spider    string              `form:"spider"`
			Version   string              `form:"_version"`
			Node      []string            `form:"fireNode"`
			Validator validator.Validator `form:"-"`
		}{}
		err := request.DecodePostForm(r, &fullQuery)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		fullQuery.Validator.CheckField(validator.NotBlank(fullQuery.Project), "project", "project can not be blank")
		fullQuery.Validator.CheckField(validator.NotBlank(fullQuery.Spider), "spider", "spider can not be blank")
		fullQuery.Validator.CheckField(len(fullQuery.Node) != 0, "node", "Select at least one node")
		if fullQuery.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = fullQuery
			data["Nodes"] = nodes
			data["PreconfiguredSettings"] = preconfiguredSettings
			app.render(w, r, http.StatusUnprocessableEntity, fireSpiderPage, nil, data)
			return
		}
		cleanForm := cleanUrlValues(r.Form, "fireNode", "csrf_token")
		type OneTimeFireResult struct {
			gocron.Job
			Node  string
			Error error
		}
		var results []OneTimeFireResult
		for _, node := range fullQuery.Node {
			jobResult := OneTimeFireResult{Node: node}
			currentTask, err := newTask(true, nil, app.DB.queries, fmt.Sprintf("One time job for spider %s on node %s",
				fullQuery.Spider, node), fullQuery.Spider, fullQuery.Project, node, app.logger, cleanForm, contextGetAuthenticatedUser(r), app.config.ScrapydEncryptSecret)
			if app.checkCreateTaskError(w, r, currentTask, err) {
				return
			}
			cronJob, err := app.scheduler.NewJob(currentTask.newOneTimeJob())
			if err != nil {
				jobResult.Error = err
			} else {
				jobResult.Job = cronJob
			}
			results = append(results, jobResult)
		}
		templateData := app.newTemplateData(r)
		templateData["Result"] = results
		app.render(w, r, http.StatusOK, firedSpiderResultPage, nil, templateData)
	}
}

func (app *application) htmxFireForm(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	q := r.URL.Query()
	if !q.Has("node") {
		app.badRequest(w, r, errors.New("node id not supplied"))
		return
	}
	tempData := app.newTemplateData(r)
	switch {
	case q.Has("node") && !q.Has("project"):
		req, err := makeRequestToScrapyd(ctxwt, app.DB.queries, http.MethodGet, q.Get("node"), func(url *url.URL) *url.URL {
			url.Path = path.Join(url.Path, ScrapydListProjectsReq)
			return url
		}, nil, nil, app.config.ScrapydEncryptSecret)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		scrapydProjects, err := requestJSONResourceFromScrapyd[scrapydListProjects](req, app.logger)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		tempData["Values"] = scrapydProjects.Projects
		tempData["Placeholder"] = "Select a Project"
	case q.Has("node") && q.Has("project"):
		req, err := makeRequestToScrapyd(ctxwt, app.DB.queries, http.MethodGet, q.Get("node"), func(url *url.URL) *url.URL {
			url.Path = path.Join(url.Path, ScrapydListSpidersReq)
			query := url.Query()
			query.Add("project", q.Get("project"))
			query.Add("_version", q.Get("version"))
			url.RawQuery = query.Encode()
			return url
		}, nil, nil, app.config.ScrapydEncryptSecret)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		scrapydSpiders, err := requestJSONResourceFromScrapyd[scrapydListSpidersResponse](req, app.logger)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		tempData["Values"] = scrapydSpiders.Spiders
		tempData["Placeholder"] = "Select a Spider"
	}
	app.renderHTMX(w, r, http.StatusOK, htmxListOfItems, nil, "htmx:listOfItems", tempData)
}

func (app *application) viewJobLogs(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	row, err := app.DB.queries.StartFinishRuntimeLogsItemsForJobWithJobID(ctxwt, r.PathValue("jobId"))
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	templateData := app.newTemplateData(r)
	templateData["RunData"] = row
	app.render(w, r, http.StatusOK, jobLogsPage, nil, templateData)
}

func (app *application) editNode(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var form editAddScrapydNode
	node, err := app.DB.queries.GetNodeWithName(ctxwt, r.PathValue("node"))
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		templateData := app.newTemplateData(r)
		form.URL = node.Url
		if node.Username.Valid && validator.NotBlank(node.Username.String) && node.Password != nil {
			decryptedPassword, err := decrypt(node.Password, app.config.ScrapydEncryptSecret)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			form.Password = &decryptedPassword
		}
		form.Username = database.ReadSqlNullString(node.Username)
		form.NodeName = node.Nodename
		templateData["Form"] = form
		app.render(w, r, http.StatusOK, nodeEditPage, nil, templateData)
	case http.MethodPost:
		err := request.DecodePostForm(r, &form)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}
		form.Validator.CheckField(validator.NotBlank(form.NodeName), "nodeName", "You must provide a name for this node")
		form.Validator.CheckField(validator.NotBlank(form.URL), "URL", "You must provide a URL for this node")
		form.Validator.CheckField(validator.IsURL(form.URL), "URL", "Node URL must be a valid URL")
		if form.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = form
			app.render(w, r, http.StatusUnprocessableEntity, nodeEditPage, nil, data)
			return
		}
		cleanUrl, err := url.ParseRequestURI(form.URL)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		cleanUrl.Path = ""
		form.URL = cleanUrl.String()
		updateQuery := database.UpdateNodeWhereNameParams{
			NewNodeName: form.NodeName,
			NewURL:      form.URL,
			NewUsername: database.CreateSqlNullString(form.Username),
			OldNodeName: r.PathValue("node"),
		}
		if form.Username != nil && validator.NotBlank(*form.Username) && form.Password != nil && validator.NotBlank(*form.Password) {
			encryptedPassword, err := encrypt(*form.Password, app.config.ScrapydEncryptSecret)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			updateQuery.NewPassword = encryptedPassword
		}
		err = app.DB.queries.UpdateNodeWhereName(ctxwt, updateQuery)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, "/list-nodes", http.StatusSeeOther)
	}
}

func (app *application) htmxListOnlineNodes(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	var workerResults []listScrapydNodesType
	numJobs := len(nodes)
	jobs := make(chan database.ScrapydNode, numJobs)
	results := make(chan listScrapydNodesType, numJobs)
	for w := 0; w <= app.config.workerCount; w++ {
		go app.listScrapydNodesWorkerFunc(ctxwt, r, jobs, results)
	}
	for _, job := range nodes {
		jobs <- job
	}
	close(jobs)
	for range numJobs {
		if workResult := <-results; workResult.Error == nil {
			workerResults = append(workerResults, workResult)
		} else {
			app.logger.Debug("Not sending node to jobs dropdown", slog.Any("node", workResult.Name), slog.Any("because it has the following error", workResult.Error))
		}

	}
	close(results)
	templateData := app.newTemplateData(r)
	templateData["Nodes"] = sortScrapydNodes(workerResults)
	app.renderHTMX(w, r, http.StatusOK, htmxListNodes, nil, "htmx:list_of_nodes", templateData)
}

func (app *application) stopJob(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	type CancelResponse struct {
		NodeName  string `json:"node_name"`
		Status    string `json:"status"`
		Prevstate string `json:"prevstate"`
	}
	req, err := makeRequestToScrapyd(ctxwt, app.DB.queries, http.MethodPost, r.PathValue("node"), func(blankUlr *url.URL) *url.URL {
		query := url.Values{}
		query.Add("project", r.PathValue("project"))
		query.Add("job", r.PathValue("job"))
		blankUlr.Path = path.Join(ScrapydStopSpider)
		blankUlr.RawQuery = query.Encode()
		return blankUlr
	}, nil, nil, app.config.ScrapydEncryptSecret)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	response, err := requestJSONResourceFromScrapyd[CancelResponse](req, app.logger)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.ToLower(strings.TrimSpace(response.Status)) != "ok" {
		app.reportServerError(r, errors.New(response.Status))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	queryParams := database.SetStoppedByOnJobParams{
		Job:     r.PathValue("job"),
		Project: r.PathValue("project"),
		Node:    r.PathValue("node"),
	}
	if user := contextGetAuthenticatedUser(r); user != nil {
		queryParams.StoppedBy = user.ID
	}

	err = app.DB.queries.SetStoppedByOnJob(ctxwt, queryParams)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *application) searchJobs(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var searchForm tasksSearchForm
	err := request.DecodePostForm(r, &searchForm)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	searchResults, err := app.DB.queries.SearchNodeJobs(ctxwt, database.SearchNodeJobsParams{
		SearchTerm: searchForm.SearchTerm,
		Node:       r.PathValue("node"),
	})
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var errored, pending, running, finished []database.SearchNodeJobsRow
	for _, job := range searchResults {
		switch job.Status {
		case "error":
			errored = append(errored, job)
		case "pending":
			pending = append(pending, job)
		case "running":
			running = append(running, job)
		case "finished":
			finished = append(finished, job)
		}
	}
	data := app.newTemplateData(r)
	data["ErrorJobs"] = errored
	data["PendingJobs"] = pending
	data["RunningJobs"] = running
	data["FinishedJobs"] = finished
	data["NodeName"] = r.PathValue("node")
	app.renderHTMX(w, r, http.StatusOK, htmxJobsTable, nil, "htmx:jobsTable", data)
}

func (app *application) listVersions(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	templateData := app.newTemplateData(r)
	templateData["Nodes"] = nodes
	app.render(w, r, http.StatusOK, versionsPage, nil, templateData)
}

func (app *application) listVersionsHTMX(w http.ResponseWriter, r *http.Request) {
	type versionsResponse struct {
		NodeName string   `json:"node_name"`
		Status   string   `json:"status"`
		Versions []string `json:"versions"`
	}
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	node, project := r.URL.Query().Get("node"), r.URL.Query().Get("project")
	if node == "" || project == "" {
		app.reportServerError(r, errors.New("no node or project specified"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req, err := makeRequestToScrapyd(ctxwt, app.DB.queries, http.MethodGet, node, func(url *url.URL) *url.URL {
		url.Path = path.Join(url.Path, ScrapydListVersions)
		query := url.Query()
		query.Add("project", project)
		url.RawQuery = query.Encode()
		return url
	}, nil, nil, app.config.ScrapydEncryptSecret)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp, err := requestJSONResourceFromScrapyd[versionsResponse](req, app.logger)
	if err != nil {
		app.reportServerError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "ok" {
		app.reportServerError(r, errors.New(resp.Status))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	slices.Reverse(resp.Versions)
	templateData := app.newTemplateData(r)
	templateData["Versions"] = resp.Versions
	templateData["Node"] = node
	templateData["Project"] = project
	app.renderHTMX(w, r, http.StatusOK, versionsPageHtmx, nil, "htmx:scrapyd_versions", templateData)
}
