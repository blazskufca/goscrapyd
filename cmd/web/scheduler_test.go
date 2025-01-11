package main

import (
	"context"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestScheduler(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	ts.login(t)
	scheduler, err := gocron.NewScheduler(gocron.WithClock(clockwork.NewFakeClock()))
	assert.NilError(t, err)
	ta.scheduler = scheduler
	ta.scheduler.Start()
	testNode, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      "http://does_not_exist.example.com",
	})
	assert.NilError(t, err)
	testCases := []struct {
		name                string
		urlValues           url.Values
		expectedStatus      int
		expectedBody        []string
		afterRequestsChecks func(ta *application, t *testing.T)
	}{
		{
			name: "Valid",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"spider":      []string{"test_spider"},
				"task_name":   []string{"test_task"},
				"cron_input":  []string{"* * * * *"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   []string{`test_task`},
			afterRequestsChecks: func(ta *application, t *testing.T) {
				tasks, err := ta.DB.queries.GetTasks(context.Background())
				assert.NilError(t, err)
				assert.Equal(t, len(tasks), 1)
				assert.Equal(t, tasks[0].Project, "test_project")
				assert.Equal(t, tasks[0].Spider, "test_spider")
				assert.Equal(t, tasks[0].Name.String, "test_task")
				assert.Equal(t, tasks[0].CronString, "* * * * *")
				assert.Equal(t, tasks[0].SelectedNodes, testNode.Nodename)
				assert.Equal(t, len(ta.scheduler.Jobs()), 1)
			},
		},
		{
			name: "Invalid no nodes",
			urlValues: url.Values{
				"project":    []string{"test_project"},
				"spider":     []string{"test_spider"},
				"task_name":  []string{"test_task"},
				"cron_input": []string{"* * * * *"},

				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`<span>You must select at least one node</span>`},
		},
		{
			name: "Invalid no project",
			urlValues: url.Values{
				"spider":      []string{"test_spider"},
				"task_name":   []string{"test_task"},
				"cron_input":  []string{"* * * * *"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`<span>You must select at least one project</span>`},
		},
		{
			name: "Invalid no spider",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"task_name":   []string{"test_task"},
				"cron_input":  []string{"* * * * *"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`<span>You must select at least one spider</span>`},
		},
		{
			name: "Invalid empty cron input",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"spider":      []string{"test_spider"},
				"task_name":   []string{"test_task"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`<span class="font-medium">You must schedule spider</span>`},
		},
		{
			name: "Invalid cron input",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"spider":      []string{"test_spider"},
				"task_name":   []string{"test_task"},
				"cron_input":  []string{"not_cron"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`Not a valid/supported cron string. Please see https://en.wikipedia.org/wiki/Cron`},
		},
		{
			name: "Invalid empty task name",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"spider":      []string{"test_spider"},
				"cron_input":  []string{"* * * * *"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   []string{`<span>Task name can not be blank</span>`},
		},
		{
			name: "Valid with additional spider values",
			urlValues: url.Values{
				"project":     []string{"test_project"},
				"spider":      []string{"test_spider"},
				"task_name":   []string{"test_task"},
				"cron_input":  []string{"* * * * *"},
				"fireNode":    []string{testNode.Nodename},
				"immediately": []string{strconv.FormatBool(false)},
				"setting":     []string{"DOWNLOAD_DELAY=10", "API=https://someapi.com"},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   []string{`test_task`},
			afterRequestsChecks: func(ta *application, t *testing.T) {
				tasks, err := ta.DB.queries.GetTasks(context.Background())
				assert.NilError(t, err)
				assert.Equal(t, len(tasks), 2)
				assert.Equal(t, tasks[1].Project, "test_project")
				assert.Equal(t, tasks[1].Spider, "test_spider")
				assert.Equal(t, tasks[1].Name.String, "test_task")
				assert.Equal(t, tasks[1].CronString, "* * * * *")
				assert.Equal(t, tasks[1].SelectedNodes, testNode.Nodename)
				assert.Equal(t, len(ta.scheduler.Jobs()), 2)
				parsedValues, err := url.ParseQuery(tasks[1].SettingsArguments)
				assert.NilError(t, err)
				assert.Equal(t, parsedValues.Has("setting"), true)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, body := ts.get(t, "/add-task")
			assert.Equal(t, code, http.StatusOK)
			gotCSRFToken := extractCSRFToken(t, body)
			tc.urlValues.Set("csrf_token", gotCSRFToken)
			code, _, body = ts.postFormFollowRedirects(t, "/add-task", tc.urlValues)
			assert.Equal(t, code, tc.expectedStatus)
			for _, expectedBody := range tc.expectedBody {
				assert.StringContains(t, body, expectedBody)
			}
			if tc.afterRequestsChecks != nil {
				tc.afterRequestsChecks(ta, t)
			}
		})
	}
}

func TestListTasks(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	ts.login(t)
	scheduler, err := gocron.NewScheduler(gocron.WithClock(clockwork.NewFakeClock()))
	assert.NilError(t, err)
	ta.scheduler = scheduler
	ta.scheduler.Start()
	testNode, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      "http://does_not_exist.example.com",
	})
	assert.NilError(t, err)
	firstTaskName, secondTaskName := "first_task", "second_task"
	firstTask, err := ta.DB.queries.InsertTask(context.Background(), database.InsertTaskParams{
		ID:            uuid.New(),
		Name:          database.CreateSqlNullString(&firstTaskName),
		Project:       "test_project",
		Spider:        "test_spider",
		Jobid:         "test_job",
		SelectedNodes: testNode.Nodename,
		CronString:    "* * * * *",
		Paused:        false,
	})
	assert.NilError(t, err)
	secondTask, err := ta.DB.queries.InsertTask(context.Background(), database.InsertTaskParams{
		ID:            uuid.New(),
		Name:          database.CreateSqlNullString(&secondTaskName),
		Project:       "test_project",
		Spider:        "test_spider",
		Jobid:         "test_job",
		SelectedNodes: testNode.Nodename,
		CronString:    "* * * * *",
		Paused:        false,
	})
	assert.NilError(t, err)
	code, _, body := ts.get(t, "/list-tasks")
	assert.Equal(t, code, http.StatusOK)
	taskIdPlaceholder := `<td class="px-6 py-4 whitespace-nowrap text-center" data-collapse-toggle="task-%s-details">%s</td>`
	assert.StringContains(t, body, fmt.Sprintf(taskIdPlaceholder, firstTask.ID.String(), firstTask.ID.String()))
	assert.StringContains(t, body, fmt.Sprintf(taskIdPlaceholder, secondTask.ID.String(), secondTask.ID.String()))
}

func TestTaskFire(t *testing.T) {
	ta := newTestApplication(t)
	ts := newTestServer(t, ta.routes())
	ts.login(t)
	done := make(chan bool)
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/schedule.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := `{"node_name": "mynodename", "status": "ok", "jobid": "6487ec79947edab326d6db28a2d86511e8247444"}`
			_, err := w.Write([]byte(resp))
			if err != nil {
				t.Fatal(err)
			}
			done <- true
		}
	}))
	testNode, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: "test_node",
		Url:      fakeServer.URL,
	})
	assert.NilError(t, err)
	scheduler, err := gocron.NewScheduler(gocron.WithClock(clockwork.NewFakeClock()))
	assert.NilError(t, err)
	ta.scheduler = scheduler
	ta.scheduler.Start()
	databaseTask, err := ta.DB.queries.InsertTask(context.Background(), database.InsertTaskParams{
		ID:                uuid.New(),
		Project:           "project",
		Spider:            "spider",
		Jobid:             "jobid",
		SettingsArguments: "",
		SelectedNodes:     testNode.Nodename,
		CronString:        "* * * * *",
	})
	assert.NilError(t, err)
	task, err := newTask(false, &databaseTask.ID, ta.DB.queries, "test-task", "test-spider", "test-project",
		testNode.Nodename, ta.logger, url.Values{}, nil, ta.config.ScrapydEncryptSecret)
	assert.NilError(t, err)
	_, err = ta.scheduler.NewJob(task.newCronJob("* * * * *"))
	assert.NilError(t, err)
	code, _, _ := ts.postForm(t, "/fire-task/"+databaseTask.ID.String(), nil)
	assert.NilError(t, err)
	assert.Equal(t, code, http.StatusOK)
	assert.Equal(t, <-done, true)
	close(done)
}
