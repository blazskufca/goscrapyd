package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/validator"
	jsoniter "github.com/json-iterator/go"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// This is very inefficient...On every page load tasks which are still in scrapyd and are not finished yet finished are updated
// even if there are no changes no checks are done in regard to that (database does coalescing on nullable columns and update time
// must be greater than previous one or write is rejected)... Not great really but for now whatever (I think that scrapyd
// keeps ~50 to ~100 most recent jobs in the worst case if you have less of them it's less)

type scrapydListJobsResponse struct {
	NodeName string           `json:"node_name,omitempty"`
	Status   string           `json:"status,omitempty"`
	Pending  []scrapydJobType `json:"pending,omitempty"`
	Running  []scrapydJobType `json:"running,omitempty"`
	Finished []scrapydJobType `json:"finished,omitempty"`
}

type scrapydJobType struct {
	Id        string             `json:"id,omitempty"`
	Project   string             `json:"project,omitempty"`
	Spider    string             `json:"spider,omitempty"`
	Version   *string            `json:"version,omitempty"`
	StartTime logParserTimestamp `json:"start_time,omitempty"`
	EndTime   logParserTimestamp `json:"end_time,omitempty"`
	Pid       *int               `json:"pid,omitempty"`
	LogUrl    *string            `json:"log_url,omitempty"`
	ItemsUrl  *string            `json:"items_url,omitempty"`
	Settings  *map[string]string `json:"settings,omitempty"`
	Args      *map[string]string `json:"args,omitempty"`
}

type logParserStat struct {
	Status              string                                               `json:"status"`
	SettingsPy          string                                               `json:"settings_py"`
	LogparserVersion    string                                               `json:"logparser_version"`
	LastUpdateTimestamp logParserTimeStampMilis                              `json:"last_update_timestamp"`
	LastUpdateTime      logParserTimestamp                                   `json:"last_update_time"`
	Datas               map[string]map[string]map[string]spiderLogParserStat `json:"datas"`
	Settings            struct {
		ScrapydServer                    string   `json:"scrapyd_server"`
		ScrapydLogsDir                   string   `json:"scrapyd_logs_dir"`
		ParseRoundInterval               int      `json:"parse_round_interval"`
		EnableTelnet                     bool     `json:"enable_telnet"`
		OverrideTelnetConsoleHost        string   `json:"override_telnet_console_host"`
		LogEncoding                      string   `json:"log_encoding"`
		LogExtensions                    []string `json:"log_extensions"`
		LogHeadLines                     int      `json:"log_head_lines"`
		LogTailLines                     int      `json:"log_tail_lines"`
		LogCategoriesLimit               int      `json:"log_categories_limit"`
		JobsToKeep                       int      `json:"jobs_to_keep"`
		ChunkSize                        int      `json:"chunk_size"`
		DeleteExistingJsonFilesAtStartup bool     `json:"delete_existing_json_files_at_startup"`
		KeepDataInMemory                 bool     `json:"keep_data_in_memory"`
		Verbose                          bool     `json:"verbose"`
		MainPid                          int      `json:"main_pid"`
	} `json:"settings"`
}

type spiderLogParserStat struct {
	LogPath        string              `json:"log_path"`
	JsonPath       string              `json:"json_path"`
	JsonUrl        string              `json:"json_url"`
	Status         string              `json:"status"`
	ShutdownReason string              `json:"shutdown_reason"`
	FinishReason   string              `json:"finish_reason"`
	Size           int                 `json:"size"`
	Position       int                 `json:"position"`
	LastUpdateTime logParserTimestamp  `json:"last_update_time"`
	Pages          *int                `json:"pages"`
	Items          *int                `json:"items"`
	FirstLogTime   *logParserTimestamp `json:"first_log_time"`
	LatestLogTime  *logParserTimestamp `json:"latest_log_time"`
	Runtime        *string             `json:"runtime"`
}

type logParserTimestamp time.Time

func (t *logParserTimestamp) UnmarshalJSON(data []byte) error {
	var timeStr string
	jsonIterType := jsoniter.ConfigFastest
	err := jsonIterType.Unmarshal(data, &timeStr)
	if err != nil {
		return err
	}

	if timeStr == "N/A" || timeStr == "" {
		*t = logParserTimestamp(time.Time{})
		return nil
	}
	parsedTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return err
	}
	*t = logParserTimestamp(parsedTime)
	return nil
}

type logParserTimeStampMilis time.Time

func (tms *logParserTimeStampMilis) UnmarshalJSON(data []byte) error {
	var timestamp int64
	jsonIterType := jsoniter.ConfigFastest
	err := jsonIterType.Unmarshal(data, &timestamp)
	if err != nil {
		return err
	}

	parsedTime := time.Unix(timestamp, 0)
	*tms = logParserTimeStampMilis(parsedTime)
	return nil
}

func (app *application) requestScrapydWorkInfo(ctx context.Context, node string) error {
	// Get the log parser stat JSON
	req, err := makeRequestToScrapyd(ctx, app.DB.queries, http.MethodGet, node, func(url *url.URL) *url.URL {
		url.Path = path.Join(url.Path, ScrapydLogStatsReq)
		return url
	}, nil, nil, app.config.ScrapydEncryptSecret)
	if err != nil {
		return err
	}
	logParserStatResponse, err := requestJSONResourceFromScrapyd[logParserStat](req, app.logger)
	if err != nil {
		return err
	}
	req, err = makeRequestToScrapyd(ctx, app.DB.queries, http.MethodGet, node, func(url *url.URL) *url.URL {
		url.Path = path.Join(url.Path, ScrapydListJobsReq)
		return url
	}, nil, nil, app.config.ScrapydEncryptSecret)
	if err != nil {
		return err
	}
	response, err := requestJSONResourceFromScrapyd[scrapydListJobsResponse](req, app.logger)
	if err != nil {
		return err
	}
	app.handleListOfSpiders(ctx, "pending", node, &logParserStatResponse, response.Pending)
	app.handleListOfSpiders(ctx, "running", node, &logParserStatResponse, response.Running)
	app.handleListOfSpiders(ctx, "finished", node, &logParserStatResponse, response.Finished)
	return nil
}

func (app *application) handleListOfSpiders(ctx context.Context, status, node string, stat *logParserStat, spiders []scrapydJobType) {
	for _, spider := range spiders {
		project, ok := stat.Datas[spider.Project]
		if !ok {
			app.logger.ErrorContext(ctx, "project not found in logparser stat", slog.Any("project", spider.Project))
			app.doPartialUpdate(ctx, node, status, spider)
			continue
		}
		logParserSpider, ok := project[spider.Spider]
		if !ok {
			app.logger.ErrorContext(ctx, "project not found in logparser stat", slog.Any("project", spider.Project))
			app.doPartialUpdate(ctx, node, status, spider)
			continue
		}
		job, ok := logParserSpider[spider.Id]
		if !ok {
			app.logger.ErrorContext(ctx, "project not found in logparser stat", slog.Any("project", spider.Project))
			app.doPartialUpdate(ctx, node, status, spider)
			continue
		}
		queryParams := database.InsertJobParams{
			Project:    spider.Project,
			Spider:     spider.Spider,
			Job:        spider.Id,
			Status:     status,
			Deleted:    false,
			CreateTime: time.Time(spider.StartTime),
			UpdateTime: time.Time(job.LastUpdateTime),
			Pages:      database.CreateSqlNullInt64FromInt(job.Pages),
			Items:      database.CreateSqlNullInt64FromInt(job.Items),
			Pid:        database.CreateSqlNullInt64FromInt(spider.Pid),
			Start:      database.CreateCreateSqlNullTimeNonPtr(time.Time(spider.StartTime)),
			Runtime:    database.CreateSqlNullString(job.Runtime),
			Finish:     database.CreateCreateSqlNullTimeNonPtr(time.Time(spider.EndTime)),
			Node:       node,
		}
		if spider.LogUrl != nil && validator.NotBlank(*spider.LogUrl) {
			logUrl := "/" + node + "/scrapyd-backend" + *spider.LogUrl
			queryParams.HrefLog = database.CreateSqlNullString(&logUrl)
		} else if validator.NotBlank(job.LogPath) {
			logUrl := strings.Replace(job.LogPath, "/root/", fmt.Sprintf("/%s/scrapyd-backend/", node), 1)
			queryParams.HrefLog = database.CreateSqlNullString(&logUrl)
		}
		if spider.ItemsUrl != nil && validator.NotBlank(*spider.ItemsUrl) {
			itemsUrl := "/" + node + "/scrapyd-backend" + *spider.ItemsUrl
			queryParams.HrefItems = database.CreateSqlNullString(&itemsUrl)
		}
		_, err := app.DB.queries.InsertJob(ctx, queryParams)
		switch {
		case err != nil && errors.Is(err, sql.ErrNoRows):
			app.logger.DebugContext(ctx, "insert rejected with sql.ErrNoRows", slog.Any("project", spider.Project), slog.Any("job", spider.Id), slog.Any("spider", spider.Spider))
		case err != nil:
			app.logger.ErrorContext(ctx, "error inserting job", slog.Any("project", spider.Project), slog.Any("job", spider.Id), slog.Any("spider", spider.Spider), slog.Any("err", err))
			continue
		}
	}
}

func (app *application) doPartialUpdate(ctx context.Context, node, status string, spider scrapydJobType) {
	// On missing logparser data insert whatever we have
	queryParams := database.InsertJobParams{
		Project:    spider.Project,
		Spider:     spider.Spider,
		Job:        spider.Id,
		Status:     status,
		Deleted:    false,
		CreateTime: time.Time(spider.StartTime),
		Pid:        database.CreateSqlNullInt64FromInt(spider.Pid),
		Start:      database.CreateCreateSqlNullTimeNonPtr(time.Time(spider.StartTime)),
		Finish:     database.CreateCreateSqlNullTimeNonPtr(time.Time(spider.EndTime)),
		Node:       node,
	}
	if spider.LogUrl != nil && validator.NotBlank(*spider.LogUrl) {
		logUrl := "/" + node + "/scrapyd-backend" + *spider.LogUrl
		queryParams.HrefLog = database.CreateSqlNullString(&logUrl)
	}
	if spider.ItemsUrl != nil && validator.NotBlank(*spider.ItemsUrl) {
		itemsUrl := "/" + node + "/scrapyd-backend" + *spider.ItemsUrl
		queryParams.HrefItems = database.CreateSqlNullString(&itemsUrl)
	}
	_, err := app.DB.queries.InsertJob(ctx, queryParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.DebugContext(ctx, "insert rejected with sql.ErrNoRows", slog.Any("job", spider.Id), slog.Any("spider", spider.Spider), slog.Any("job", spider.Id))
		} else {
			app.logger.ErrorContext(ctx, "error inserting job", slog.Any("project", spider.Project), slog.Any("job", spider.Id), slog.Any("spider", spider.Spider), slog.Any("err", err))
		}
	}
}
