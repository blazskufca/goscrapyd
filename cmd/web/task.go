package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Task struct {
	ID           uuid.UUID
	JobID        string
	Project      string
	Spider       string
	TaskName     string
	NodeName     string
	Secret       string
	SpiderValues url.Values
	DB           *database.Queries
	Logger       *slog.Logger
	User         *database.User
	OneTimeJob   bool
}

type scrapydScheduleResponse struct {
	NodeName string `json:"node_name"`
	Status   string `json:"status"`
	Jobid    string `json:"jobid"`
}

func newTask(oneTimeJob bool, taskID *uuid.UUID, db *database.Queries, taskName, spider, project, nodeName string,
	logger *slog.Logger, spiderValues url.Values, user *database.User, secret string) (*Task, error) {

	if db == nil {
		return nil, errors.New("database is required")
	}

	t := &Task{
		DB:           db,
		Logger:       logger,
		Project:      project,
		Spider:       spider,
		NodeName:     nodeName,
		SpiderValues: make(url.Values, len(spiderValues)),
		TaskName:     taskName,
		OneTimeJob:   oneTimeJob,
		User:         user,
		Secret:       secret,
	}

	if taskID == nil {
		newID, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		t.ID = newID
	} else {
		t.ID = *taskID
	}
	maps.Copy(t.SpiderValues, spiderValues)
	return t, nil
}

func (t *Task) fireFunc() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := t.insertJobIntoDB(ctx); err != nil {
		return err
	}

	req, err := t.createScrapydRequest(ctx)
	if err != nil {
		return err
	}

	if err := t.scheduleSpider(req); err != nil {
		return err
	}

	return nil
}

func (t *Task) insertJobIntoDB(ctx context.Context) error {
	insertParam := database.InsertJobParams{
		Project:    t.Project,
		Spider:     t.Spider,
		Job:        t.JobID,
		Status:     "scheduled",
		Deleted:    false,
		CreateTime: time.Now(),
		UpdateTime: time.Time{},
		Node:       t.NodeName,
	}
	if !t.OneTimeJob {
		insertParam.TaskID = t.ID
	}
	if t.User != nil && t.OneTimeJob {
		insertParam.StartedBy = t.User.ID
	}
	_, err := t.DB.InsertJob(ctx, insertParam)
	return err
}

func (t *Task) createScrapydRequest(ctx context.Context) (*http.Request, error) {
	return makeRequestToScrapyd(ctx, t.DB, http.MethodPost, t.NodeName, func(url *url.URL) *url.URL {
		url.Path = path.Join(url.Path, scrapydScheduleSpider)
		url.RawQuery = t.SpiderValues.Encode()
		return url
	}, nil, &http.Header{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	}, t.Secret)
}

func (t *Task) scheduleSpider(req *http.Request) error {
	scheduleResp, err := requestJSONResourceFromScrapyd[scrapydScheduleResponse](req, t.Logger)
	if err != nil {
		return err
	}

	if strings.ToLower(strings.TrimSpace(scheduleResp.Status)) != "ok" {
		return errors.New(scheduleResp.Status)
	}

	return nil
}

func (t *Task) beforeJobRuns(jobID uuid.UUID, jobName string) {
	if t.OneTimeJob {
		t.JobID = fmt.Sprintf("one_time_job_%s_%s_%s", t.Spider, t.NodeName, time.Now().Format("2006-01-02T15_04_05"))
	} else {
		t.JobID = fmt.Sprintf("task_%s_%s_%s", t.Spider, t.NodeName, time.Now().Format("2006-01-02T15_04_05"))
	}
	if !t.SpiderValues.Has("jobid") {
		t.SpiderValues.Set("jobid", t.JobID)
	} else {
		t.SpiderValues.Set("jobid", t.JobID)
	}
}

func (t *Task) afterTaskRunsWithSuccess(jobID uuid.UUID, jobName string) {
	t.Logger.Debug("task started successfully", slog.Any("jobID", jobID), slog.Any("jobName", jobName))
}

func (t *Task) afterTaskRunsWithError(jobID uuid.UUID, jobName string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			t.Logger.Error("panic in afterTaskRunsWithError", slog.Any("jobID", jobID), slog.Any("jobName", jobName), slog.Any("recoverData", r))
		}
	}()
	t.Logger.ErrorContext(ctx, "error in task", slog.Any("task", jobID), slog.Any("jobName", jobName), slog.Any("err", err))
	if !errors.Is(err, sql.ErrNoRows) {
		errAsString := base64.StdEncoding.EncodeToString([]byte(err.Error()))
		if dbErr := t.DB.SetErrorWhereJobId(ctx, database.SetErrorWhereJobIdParams{
			Error:   database.CreateSqlNullString(&errAsString),
			JobID:   t.JobID,
			Project: t.Project,
			Node:    t.NodeName,
		}); dbErr != nil {
			t.Logger.ErrorContext(ctx, "error saving error for task into database", slog.Any("jobID", jobID), slog.Any("jobName", jobName), slog.Any("err", dbErr))
		}
	} else {
		t.Logger.Error("no row in database, insert failed?", slog.Any("jobID", jobID), slog.Any("jobName", jobName))
	}
}

func (t *Task) afterTaskPanics(jobID uuid.UUID, jobName string, recoverData any) {
	t.Logger.Error("PANIC IN TASK", slog.Any("jobID", jobID), slog.Any("jobName", jobName), "recoverData", recoverData)
}

func (t *Task) newEventListener() gocron.JobOption {
	return gocron.WithEventListeners(
		gocron.BeforeJobRuns(t.beforeJobRuns),
		gocron.AfterJobRuns(t.afterTaskRunsWithSuccess),
		gocron.AfterJobRunsWithError(t.afterTaskRunsWithError),
		gocron.AfterJobRunsWithPanic(t.afterTaskPanics),
	)
}

func (t *Task) newOneTimeJob() (gocron.JobDefinition, gocron.Task, gocron.JobOption, gocron.JobOption, gocron.JobOption) {
	return gocron.OneTimeJob(gocron.OneTimeJobStartImmediately()),
		gocron.NewTask(t.fireFunc), gocron.WithName(t.TaskName), gocron.WithIdentifier(t.ID),
		t.newEventListener()
}

func (t *Task) newCronJob(schedule string) (gocron.JobDefinition, gocron.Task, gocron.JobOption, gocron.JobOption, gocron.JobOption) {
	return gocron.CronJob(schedule, false), gocron.NewTask(t.fireFunc),
		gocron.WithName(t.TaskName), gocron.WithIdentifier(t.ID), t.newEventListener()
}
