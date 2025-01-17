package main

import (
	"context"
	"encoding/base64"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTask_Integration(t *testing.T) {
	app := newTestApplication(t)
	fakeClock := clockwork.NewFakeClock()
	scheduler, err := gocron.NewScheduler(gocron.WithClock(fakeClock))
	assert.NilError(t, err)
	scheduler.Start()
	app.scheduler = scheduler

	t.Run("One time job", func(t *testing.T) {
		mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, http.MethodPost)
			assert.Equal(t, r.URL.Path, "/schedule.json")
			query := r.URL.Query()
			assert.Equal(t, query.Has("jobid"), true)
			assert.StringContains(t, query.Get("jobid"), "test_node")
			assert.StringContains(t, query.Get("jobid"), "test_spider")
			assert.StringContains(t, query.Get("jobid"), "one_time_job")
			assert.Equal(t, query.Has("test_param"), true)
			assert.Equal(t, query.Get("test_param"), "test_value")
			assert.Equal(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
			baUsername, baPassword, ok := r.BasicAuth()
			assert.Equal(t, ok, false)
			assert.Equal(t, baUsername, "")
			assert.Equal(t, baPassword, "")
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"status":"ok", "jobid":"test-job-123", "node_name":"test-node"}`))
			assert.NilError(t, err)
		}))
		defer mockScrapyd.Close()
		spiderValues := make(url.Values)
		spiderValues.Set("test_param", "test_value")

		_, err := app.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
			Nodename: "test_node",
			Url:      mockScrapyd.URL,
		})
		assert.NilError(t, err)
		taskID := uuid.New()
		createdTask, err := app.newTask(
			true,
			&taskID,
			"test_integration_task",
			"test_spider",
			"test_project",
			"test_node",
			spiderValues,
			nil,
		)
		assert.NilError(t, err)
		_, err = createdTask.newOneTimeJob()
		assert.NilError(t, err)
		// README: There some kind of a race between goose bringing the database UP and task trying to write into table
		// It tries to write into not yet existent table raising various errors, despite goose saying migrations are done (??)
		// I'm unsure how to solve this in goose (well could mock the database in the worst case), but for sleep does it for now
		// Far from ideal, hopefully it does not create flaky tests, fingers crossed
		time.Sleep(100 * time.Millisecond)
		jobs, err := app.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   "test_node",
			Limit:  100,
			Offset: 0,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobs), 1)

		assert.Equal(t, jobs[0].Project, createdTask.Project)
		assert.Equal(t, jobs[0].Spider, createdTask.Spider)
		assert.Equal(t, jobs[0].Node, createdTask.NodeName)
		assert.Equal(t, jobs[0].Status, "scheduled")
	})
	t.Run("One time job with error", func(t *testing.T) {
		spiderValues := make(url.Values)
		spiderValues.Set("test_param", "error_case")

		_, err := app.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
			Nodename: "error_node",
			Url:      "",
		})
		assert.NilError(t, err)

		taskID := uuid.New()
		createdTask, err := app.newTask(
			true,
			&taskID,
			"test_error_task",
			"test_spider",
			"test_project_error",
			"error_node",
			spiderValues,
			nil,
		)
		assert.NilError(t, err)

		_, err = createdTask.newOneTimeJob()
		assert.NilError(t, err)

		// Wait for execution (or rather goose to migrate)
		time.Sleep(100 * time.Millisecond)
		jobs, err := app.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:  "error_node",
			Limit: 100,
		})
		assert.NilError(t, err)
		assert.Equal(t, jobs[0].Status, "error")
		strErr, err := base64.StdEncoding.DecodeString(jobs[0].Error.String)
		assert.NilError(t, err)
		assert.StringContains(t, string(strErr), `Post "schedule.json?jobid=one_time_job_test_spider_error_node`)
	})
	t.Run("One Time job with basic auth", func(t *testing.T) {
		app.config.ScrapydEncryptSecret = "thisis16bytes123"
		username := "testUser"
		mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, http.MethodPost)
			assert.Equal(t, r.URL.Path, "/schedule.json")
			query := r.URL.Query()
			assert.Equal(t, query.Has("jobid"), true)
			assert.StringContains(t, query.Get("jobid"), "test_node_with_basic_auth")
			assert.StringContains(t, query.Get("jobid"), "test_spider")
			assert.StringContains(t, query.Get("jobid"), "one_time_job")
			assert.Equal(t, query.Has("test_param"), true)
			assert.Equal(t, query.Get("test_param"), "test_value")
			assert.Equal(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
			baUsername, baPassword, ok := r.BasicAuth()
			assert.Equal(t, ok, true)
			assert.Equal(t, baUsername, username)
			assert.Equal(t, baPassword, "nodePassword")
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"status":"ok", "jobid":"test-job-123", "node_name":"test-node"}`))
			assert.NilError(t, err)
		}))
		defer mockScrapyd.Close()
		spiderValues := make(url.Values)
		spiderValues.Set("test_param", "test_value")

		nodePassword, err := encrypt("nodePassword", app.config.ScrapydEncryptSecret)
		assert.NilError(t, err)

		_, err = app.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
			Nodename: "test_node_with_basic_auth",
			Url:      mockScrapyd.URL,
			Username: database.CreateSqlNullString(&username),
			Password: nodePassword,
		})
		assert.NilError(t, err)
		taskID := uuid.New()
		createdTask, err := app.newTask(
			true,
			&taskID,
			"test_integration_task",
			"test_spider",
			"test_project",
			"test_node_with_basic_auth",
			spiderValues,
			nil,
		)
		assert.NilError(t, err)

		_, err = createdTask.newOneTimeJob()
		assert.NilError(t, err)
		time.Sleep(100 * time.Millisecond)
		jobs, err := app.DB.queries.GetJobsForNode(context.Background(), database.GetJobsForNodeParams{
			Node:   "test_node_with_basic_auth",
			Limit:  100,
			Offset: 0,
		})
		assert.NilError(t, err)
		assert.Equal(t, len(jobs), 1)

		assert.Equal(t, jobs[0].Project, createdTask.Project)
		assert.Equal(t, jobs[0].Spider, createdTask.Spider)
		assert.Equal(t, jobs[0].Node, createdTask.NodeName)
		assert.Equal(t, jobs[0].Status, "scheduled")
	})
}
