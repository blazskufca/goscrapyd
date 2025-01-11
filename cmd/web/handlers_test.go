package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/go-co-op/gocron/v2"
	"github.com/jonboulle/clockwork"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"
)

func TestInsertingNewScrapydNode(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	t.Run("GET request", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, "<h1 class=\"text-3xl font-extrabold dark:text-white\">Add a new node:</h1>")
	})
	t.Run("Valid POST request", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Add("nodeName", "TestNode")
		formValues.Add("url", "http://not-valid:6800")
		formValues.Add("username", "test")
		formValues.Add("password", "test")
		formValues.Add("csrf_token", gotCSRFToken)
		code, _, body = ts.postFormFollowRedirects(t, "/add-node", formValues)
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, `<h1 class="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white">Scrapyd Nodes</h1>`)
		assert.StringContains(t, body, `<a href="/TestNode/jobs" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">TestNode</a>`)
		node, err := ta.DB.queries.GetNodeWithName(context.Background(), "TestNode")
		assert.NilError(t, err)
		assert.Equal(t, node.Nodename, "TestNode")
		assert.Equal(t, node.Url, "http://not-valid:6800")
		assert.Equal(t, node.Username.String, "test")
		decryptedPassword, err := decrypt(node.Password, ta.config.ScrapydEncryptSecret)
		assert.NilError(t, err)
		assert.Equal(t, decryptedPassword, "test")
	})
	t.Run("Fails with duplicate node", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Add("nodeName", "TestNode")
		formValues.Add("url", "http://not-valid:6800")
		formValues.Add("username", "test")
		formValues.Add("password", "test")
		formValues.Add("csrf_token", gotCSRFToken)
		code, _, body = ts.postFormFollowRedirects(t, "/add-node", formValues)
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		assert.StringContains(t, body, `<h1 class="text-3xl font-extrabold dark:text-white">Add a new node:</h1>`)
		assert.StringContains(t, body, `<span class="font-medium">Node with name TestNode and URL http://not-valid:6800 already exists</span>`)
	})
	t.Run("Fails with blank name", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Add("url", "http://not-valid:6800")
		formValues.Add("username", "test")
		formValues.Add("password", "test")
		formValues.Add("csrf_token", gotCSRFToken)
		code, _, body = ts.postFormFollowRedirects(t, "/add-node", formValues)
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		assert.StringContains(t, body, `<h1 class="text-3xl font-extrabold dark:text-white">Add a new node:</h1>`)
		assert.StringContains(t, body, `<span>You must provide a name for this node</span>`)
	})
	t.Run("Fails with URL", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Add("nodeName", "TestNode")
		formValues.Add("username", "test")
		formValues.Add("password", "test")
		formValues.Add("csrf_token", gotCSRFToken)
		code, _, body = ts.postFormFollowRedirects(t, "/add-node", formValues)
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		assert.StringContains(t, body, `<h1 class="text-3xl font-extrabold dark:text-white">Add a new node:</h1>`)
		assert.StringContains(t, body, `<span>You must provide a URL for this node</span>`)
	})
	t.Run("Fails with invalid URL", func(t *testing.T) {
		code, _, body := ts.get(t, "/add-node")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Add("nodeName", "TestNode")
		formValues.Add("url", "not a URL")
		formValues.Add("username", "test")
		formValues.Add("password", "test")
		formValues.Add("csrf_token", gotCSRFToken)
		code, _, body = ts.postFormFollowRedirects(t, "/add-node", formValues)
		assert.Equal(t, code, http.StatusUnprocessableEntity)
		assert.StringContains(t, body, `<h1 class="text-3xl font-extrabold dark:text-white">Add a new node:</h1>`)
		assert.StringContains(t, body, `<span>Node URL must be a valid URL</span>`)
	})
}

func TestDeletingNode(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "node_to_delete",
		Url:      "http://does_not_exist:6800",
	})
	assert.NilError(t, err)
	t.Run("Delete node", func(t *testing.T) {
		code, _, body := ts.get(t, "/list-nodes")
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/jobs" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Nodename))
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/scrapyd-backend/" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Url))
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/delete-node/"+node.Nodename, nil)
		assert.NilError(t, err)
		response, err := ts.Client().Do(req)
		assert.NilError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		_, err = ta.DB.queries.GetNodeWithName(context.Background(), node.Nodename)
		assert.Equal(t, err, sql.ErrNoRows)
		code, _, body = ts.get(t, "/list-nodes")
		assert.Equal(t, http.StatusOK, code)
		assert.StringDoesNotContain(t, body, fmt.Sprintf(`<a href="/%s/jobs" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Nodename))
		assert.StringDoesNotContain(t, body, fmt.Sprintf(`<a href="/%s/scrapyd-backend/" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Url))
	})
}

func TestListingNodes(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "node1",
		Url:      "http://does_not_exist:6800",
	})
	assert.NilError(t, err)
	node2, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "node2",
		Url:      "http://does_not_exist:6801",
	})
	assert.NilError(t, err)
	t.Run("Listing Nodes", func(t *testing.T) {
		code, _, body := ts.get(t, "/list-nodes")
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/jobs" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Nodename))
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/scrapyd-backend/" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node.Nodename, node.Url))
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/jobs" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node2.Nodename, node2.Nodename))
		assert.StringContains(t, body, fmt.Sprintf(`<a href="/%s/scrapyd-backend/" target="_blank" class="hover:text-blue-600 dark:hover:text-blue-500">%s</a>`, node2.Nodename, node2.Url))
	})
}

func TestNodeJobs(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	logParserDataMock := `{
	"status": "ok",
	"datas": {
	"testProject": {
	  "testSpider": {
		"task_testProject_testSpider_2022-03-14T07_00_00": {
		  "log_path": "/some/path/task_testProject_testSpider_2022-03-14T07_00_00.log",
		  "json_path": "/some/path/task_testProject_testSpider_2022-03-14T07_00_00.json",
		  "json_url": "http://does_not_exists:6800/some/path/task_testProject_testSpider_2022-03-14T07_00_00.json",
		  "size": 6197838,
		  "position": 6197838,
		  "status": "ok",
		  "pages": 447182,
		  "items": 427335,
		  "first_log_time": "2022-03-14 07:01:26",
		  "latest_log_time": "2022-03-21 10:02:10",
		  "runtime": "7 days, 3:00:44",
		  "shutdown_reason": "N/A",
		  "finish_reason": "finished",
		  "last_update_time": "2022-03-21 10:02:18"
		}
	  }
	}
	},
	"settings_py": "/usr/local/lib/python3.8/site-packages/logparser/settings.py",
	"settings": {
	"scrapyd_server": "0.0.0.0:80",
	"scrapyd_logs_dir": "/scrapyd_data/logs",
	"parse_round_interval": 10,
	"enable_telnet": true,
	"override_telnet_console_host": "",
	"log_encoding": "utf-8",
	"log_extensions": [
	  ".log",
	  ".txt"
	],
	"log_head_lines": 100,
	"log_tail_lines": 200,
	"log_categories_limit": 10,
	"jobs_to_keep": 100,
	"chunk_size": 10000000,
	"delete_existing_json_files_at_startup": false,
	"keep_data_in_memory": false,
	"verbose": false,
	"main_pid": 0
	},
	"last_update_timestamp": 1736584994,
	"last_update_time": "2025-01-11 09:43:14",
	"logparser_version": "0.8.2"
	}`

	scrapydJobsMockData := `{
	"node_name": "mynodename",
	"status": "ok",
	"pending": [],
	"running": [
		{
			"id": "task_testProject_testSpider_2022-03-14T07_00_00",
			"project": "testProject",
			"spider": "testSpider",
			"pid": 93956,
			"start_time": "2012-09-12 10:14:03.594664",
			"log_url": "/some/path/task_testProject_testSpider_2022-03-14T07_00_00.log",
			"items_url": "/some/path/task_testProject_testSpider_2022-03-14T07_00_00.jl"
		}
	],
	"finished": []
	}`
	mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/logs/stats.json":
			_, err := w.Write([]byte(logParserDataMock))
			assert.NilError(t, err)
		case "/listjobs.json":
			_, err := w.Write([]byte(scrapydJobsMockData))
			assert.NilError(t, err)
		}
	}))
	defer mockScrapyd.Close()
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "node1",
		Url:      mockScrapyd.URL,
	})
	assert.NilError(t, err)
	t.Run("Listing Nodes", func(t *testing.T) {
		code, _, body := ts.get(t, fmt.Sprintf("/%s/jobs", node.Nodename))
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">testProject</td>`)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">testSpider</td>`)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">task_testProject_testSpider_2022-03-14T07_00_00</td>`)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">447182</td>`)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">427335</td>`)
		jobs, err := ta.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   "node1",
			Limit:  1000,
			Offset: 0,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobs), 1)
		assert.Equal(t, jobs[0].Project, "testProject")
		assert.Equal(t, jobs[0].Spider, "testSpider")
		assert.Equal(t, jobs[0].Job, "task_testProject_testSpider_2022-03-14T07_00_00")
		assert.Equal(t, jobs[0].Status, "running")
	})
}

func TestDeletingJobs(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      "http://does_not_exists:6800",
	})
	assert.NilError(t, err)
	insertedJob, err := ta.DB.queries.InsertJob(context.Background(), database.InsertJobParams{
		Project:    "testProject",
		Spider:     "testSpider",
		Job:        "test_job",
		Status:     "finished",
		Deleted:    false,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
		Node:       node.Nodename,
	})
	assert.NilError(t, err)
	t.Run("Deleting job", func(t *testing.T) {
		jobsForNode, err := ta.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   node.Nodename,
			Limit:  1000,
			Offset: 0,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobsForNode), 1)
		assert.Equal(t, jobsForNode[0].Project, insertedJob.Project)
		assert.Equal(t, jobsForNode[0].Spider, insertedJob.Spider)
		assert.Equal(t, jobsForNode[0].Job, insertedJob.Job)
		assert.Equal(t, jobsForNode[0].Status, insertedJob.Status)
		assert.Equal(t, jobsForNode[0].Deleted, insertedJob.Deleted)
		assert.Equal(t, jobsForNode[0].Deleted, false)
		req, err := http.NewRequest(http.MethodDelete, ts.URL+fmt.Sprintf("/delete-job/%s", insertedJob.Job), nil)
		assert.NilError(t, err)
		response, err := ts.Client().Do(req)
		assert.NilError(t, err)
		assert.Equal(t, response.StatusCode, http.StatusOK)
		jobsForNode, err = ta.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   node.Nodename,
			Limit:  1000,
			Offset: 0,
		})
		assert.NilError(t, err)
		// Job should be deleted now
		// It's a soft deleted but SELECT does not pick up soft deleted jobs
		assert.Equal(t, len(jobsForNode), 0)
	})
}

func TestFireSpiderPage(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	scheduler, err := gocron.NewScheduler(gocron.WithClock(clockwork.NewFakeClock()))
	assert.NilError(t, err)
	ta.scheduler = scheduler
	ta.scheduler.Start()
	mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/schedule.json":
			query := r.URL.Query()
			assert.Equal(t, query.Has("jobid"), true)
			assert.Equal(t, query.Has("project"), true)
			assert.Equal(t, query.Get("project"), "testProject")
			assert.Equal(t, query.Has("spider"), true)
			assert.Equal(t, query.Get("spider"), "test_spider")
			_, err := w.Write([]byte(fmt.Sprintf(`{"node_name": "test_node", "status": "ok", "jobid": "%s"}`, query.Get("jobid"))))
			assert.NilError(t, err)
		}
	}))
	defer mockScrapyd.Close()
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      mockScrapyd.URL,
	})
	assert.NilError(t, err)
	t.Run("Fire spider page", func(t *testing.T) {
		code, _, body := ts.get(t, "/fire-spider")
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Set("csrf_token", gotCSRFToken)
		formValues.Set("spider", "test_spider")
		formValues.Set("project", "testProject")
		formValues.Set("fireNode", node.Nodename)
		code, _, body = ts.postFormFollowRedirects(t, "/fire-spider", formValues)
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, `One time job for spider test_spider on node test_node`)
		assert.Equal(t, len(ta.scheduler.Jobs()), 1)
		jobs, err := ta.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:  node.Nodename,
			Limit: 1000,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobs), 1)
		assert.Equal(t, jobs[0].Project, "testProject")
		assert.Equal(t, jobs[0].Spider, "test_spider")
		assert.Equal(t, jobs[0].Status, "scheduled")
	})
}

func TestHtmxFireForm(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/listprojects.json":
			_, err := w.Write([]byte(`{"node_name": "mynodename", "status": "ok", "projects": ["myproject", "otherproject"]}`))
			assert.NilError(t, err)
		case "/listspiders.json":
			query := r.URL.Query()
			assert.Equal(t, query.Has("project"), true)
			_, err := w.Write([]byte(`{"node_name": "mynodename", "status": "ok", "spiders": ["spider1", "spider2", "spider3"]}`))
			assert.NilError(t, err)
		}
	}))
	defer mockScrapyd.Close()
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      mockScrapyd.URL,
	})
	assert.NilError(t, err)
	t.Run("test listing projects", func(t *testing.T) {
		code, _, body := ts.get(t, "/htmx-fire-form?node="+node.Nodename)
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, `<option value="">Select a Project</option>`)
		assert.StringContains(t, body, `<option value="myproject">myproject</option>`)
		assert.StringContains(t, body, `<option value="otherproject">otherproject</option>`)
	})
	t.Run("test listing spiders", func(t *testing.T) {
		code, _, body := ts.get(t, "/htmx-fire-form?node="+node.Nodename+"&project=someProject")
		assert.Equal(t, http.StatusOK, code)
		assert.StringContains(t, body, `<option value="">Select a Spider</option>`)
		assert.StringContains(t, body, `<option value="spider1">spider1</option>`)
		assert.StringContains(t, body, `<option value="spider2">spider2</option>`)
		assert.StringContains(t, body, `<option value="spider3">spider3</option>`)
	})
}

func TestNodeEdit(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	defer ts.Close()
	ts.login(t)
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      "http://none_url:6800",
	})
	assert.NilError(t, err)
	t.Run("test editing node", func(t *testing.T) {
		dbNode, err := ta.DB.queries.GetNodeWithName(context.Background(), "test_node")
		assert.NilError(t, err)
		assert.Equal(t, dbNode.Nodename, "test_node")
		assert.Equal(t, dbNode.Url, "http://none_url:6800")
		assert.Equal(t, dbNode.Username.Valid, false)
		if dbNode.Password != nil {
			t.Fatal("password should be empty before edit")
		}
		code, _, body := ts.get(t, "/node/edit/"+node.Nodename)
		assert.Equal(t, http.StatusOK, code)
		gotCSRFToken := extractCSRFToken(t, body)
		formValues := url.Values{}
		formValues.Set("csrf_token", gotCSRFToken)
		formValues.Set("url", "http://new-fake-url.com")
		formValues.Set("username", "myuser")
		formValues.Set("password", "ThisIsAVerySecurePassword$")
		formValues.Set("nodeName", "newNodeName")
		code, _, _ = ts.postFormFollowRedirects(t, "/node/edit/"+node.Nodename, formValues)
		assert.Equal(t, http.StatusOK, code)
		dbNode, err = ta.DB.queries.GetNodeWithName(context.Background(), "newNodeName")
		assert.NilError(t, err)
		assert.Equal(t, dbNode.Username.Valid, true)
		if dbNode.Password == nil {
			t.Fatal("password should not be empty after edit")
		}
		assert.Equal(t, dbNode.Username.String, "myuser")
		assert.Equal(t, dbNode.Url, "http://new-fake-url.com")
		assert.Equal(t, dbNode.Nodename, "newNodeName")
		decryptedPassword, err := decrypt(dbNode.Password, ta.config.ScrapydEncryptSecret)
		assert.NilError(t, err)
		assert.Equal(t, decryptedPassword, "ThisIsAVerySecurePassword$")
	})
}

func TestStopJob(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cancel.json":
			assert.Equal(t, http.MethodPost, r.Method)
			query := r.URL.Query()
			assert.Equal(t, query.Has("job"), true)
			assert.Equal(t, query.Get("job"), "test_job")
			assert.Equal(t, query.Has("project"), true)
			assert.Equal(t, query.Get("project"), "testProject")
			_, err := w.Write([]byte(`{"node_name": "mynodename", "status": "ok", "prevstate": "running"}`))
			assert.NilError(t, err)
		}
	}))
	defer mockScrapyd.Close()
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      mockScrapyd.URL,
	})
	assert.NilError(t, err)
	job, err := ta.DB.queries.InsertJob(context.Background(), database.InsertJobParams{
		Project:    "testProject",
		Spider:     "testSpider",
		Job:        "test_job",
		Status:     "running",
		Deleted:    false,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
		Node:       node.Nodename,
	})
	assert.NilError(t, err)
	t.Run("test stopping job", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+fmt.Sprintf("/%s/stop-job/%s/%s", node.Nodename, job.Project, job.Job), nil)
		assert.NilError(t, err)
		response, err := ts.Client().Do(req)
		assert.NilError(t, err)
		assert.Equal(t, response.StatusCode, http.StatusOK)
		jobs, err := ta.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   node.Nodename,
			Limit:  100,
			Offset: 0,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobs), 1)
		assert.Equal(t, jobs[0].Project, job.Project)
		assert.Equal(t, jobs[0].Job, job.Job)
		assert.Equal(t, jobs[0].StoppedByUsername.String, "admin")
	})
}

func TestJobsSearching(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      "http://new-fake-url.com",
	})
	assert.NilError(t, err)
	var insertedJobs []database.Job
	for i := 0; i < 5; i++ {
		job, err := ta.DB.queries.InsertJob(context.Background(), database.InsertJobParams{
			Project:    "testProject",
			Spider:     "testSpider",
			Job:        fmt.Sprintf("test_job_%d", i),
			Status:     "running",
			Deleted:    false,
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
			Node:       node.Nodename,
		})
		assert.NilError(t, err)
		insertedJobs = append(insertedJobs, job)
	}
	t.Run("test searching jobs", func(t *testing.T) {
		formValues := url.Values{}

		for _, currentJob := range insertedJobs {
			formValues.Set("searchTerm", currentJob.Job)
			code, _, body := ts.postForm(t, fmt.Sprintf("/%s/job/search", node.Nodename), formValues)
			assert.Equal(t, code, http.StatusOK)
			jobPattern := `<td class="px-6 py-4 whitespace-nowrap text-center">(test_job_\d+)</td>`
			re := regexp.MustCompile(jobPattern)
			matches := re.FindAllStringSubmatch(body, -1)
			assert.Equal(t, len(matches), 1)
			assert.Equal(t, matches[0][1], currentJob.Job)
		}
	})
}

func TestListVersions(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	defer ts.Close()
	ts.login(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	project := "testProject"
	mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/listversions.json":
			assert.Equal(t, r.Method, http.MethodGet)
			query := r.URL.Query()
			assert.Equal(t, query.Has("project"), true)
			assert.Equal(t, query.Get("project"), project)
			_, err := w.Write([]byte(`{"node_name": "mynodename", "status": "ok", "versions": ["r99", "r156"]}`))
			assert.NilError(t, err)
		}
	}))
	defer mockScrapyd.Close()
	node, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      mockScrapyd.URL,
	})
	assert.NilError(t, err)
	t.Run("test list versions", func(t *testing.T) {
		formValues := url.Values{}
		formValues.Set("project", project)
		code, _, body := ts.get(t, fmt.Sprintf("/versions-htmx?node=%s&project=%s", node.Nodename, project))
		assert.Equal(t, code, http.StatusOK)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">r156</td>`)
		assert.StringContains(t, body, `<td class="px-6 py-4 whitespace-nowrap text-center">r99</td>`)
	})
}
