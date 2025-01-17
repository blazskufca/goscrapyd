package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/request"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type tasksBulkForm struct {
	Action        string   `form:"action"`
	SelectedTasks []string `form:"selected_tasks"`
}

type taskEditAddFormData struct {
	Project     string              `form:"project"`
	Spider      string              `form:"spider"`
	TaskName    string              `form:"task_name"`
	CronTab     string              `form:"cron_input"`
	FireNodes   []string            `form:"fireNode"`
	Immediately *bool               `form:"immediately"`
	Validator   validator.Validator `form:"-"`
}

func (app *application) createNewTask(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var formData taskEditAddFormData
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	preconfiguredSettings, err := app.getPreconfiguredSettings(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		templateData := app.newTemplateData(r)
		templateData["PreconfiguredSettings"] = preconfiguredSettings
		templateData["Nodes"] = nodes
		app.render(w, r, http.StatusOK, addTaskPage, nil, templateData)
	case http.MethodPost:
		err := request.DecodePostForm(r, &formData)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		// Try to catch invalid/unknown cron format before it reaches gocron and throws a server error there!
		_, cronParseError := cron.ParseStandard(formData.CronTab)
		if cronParseError != nil {
			app.logger.ErrorContext(ctxwt, "got invalid/unknown cron schedule according to parse standard", slog.Any("cronString", formData.CronTab), slog.Any("err", cronParseError))
		}
		formData.Validator.CheckField(len(formData.FireNodes) != 0, "fireNodes", "You must select at least one node")
		formData.Validator.CheckField(validator.NotBlank(formData.Project), "project", "You must select at least one project")
		formData.Validator.CheckField(validator.NotBlank(formData.Spider), "spider", "You must select at least one spider")
		formData.Validator.CheckField(validator.NotBlank(formData.CronTab), "cron_input", "You must schedule spider")
		formData.Validator.CheckField(cronParseError == nil, "cron_input", "Not a valid/supported cron string. Please see https://en.wikipedia.org/wiki/Cron")
		formData.Validator.CheckField(validator.NotBlank(formData.TaskName), "task_name", "Task name can not be blank")
		if formData.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = formData
			data["Nodes"] = nodes
			data["PreconfiguredSettings"] = preconfiguredSettings
			app.render(w, r, http.StatusUnprocessableEntity, addTaskPage, nil, data)
			return
		}
		// Cleanup form data, remove the metadata
		cleanForm := cleanUrlValues(r.PostForm, "fireNode", "csrf_token", "cron_input", "task_name", "immediately")
		var result []gocron.Job
		for _, node := range formData.FireNodes {
			createdTask, err := app.newTask(false, nil, formData.TaskName, formData.Spider, formData.Project, node, cleanForm, nil)
			if app.checkCreateTaskError(w, r, createdTask, err) {
				return
			}
			cronJob, err := createdTask.newCronJob(formData.CronTab)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			if formData.Immediately != nil && *formData.Immediately {
				err = cronJob.RunNow()
				if err != nil {
					app.serverError(w, r, err)
					return
				}
			}
			result = append(result, cronJob)
			queryParams := database.InsertTaskParams{
				ID:                cronJob.ID(),
				Name:              database.CreateSqlNullString(&formData.TaskName),
				Project:           formData.Project,
				Spider:            formData.Spider,
				Jobid:             formData.TaskName,
				SettingsArguments: cleanForm.Encode(),
				SelectedNodes:     node,
				CronString:        formData.CronTab,
				Paused:            false,
			}
			if user := contextGetAuthenticatedUser(r); user != nil {
				queryParams.CreatedBy = user.ID
			}
			_, err = app.DB.queries.InsertTask(ctxwt, queryParams)
			if err != nil {
				app.serverError(w, r, err)
				return
			}

		}
		templateData := app.newTemplateData(r)
		templateData["Result"] = result
		app.render(w, r, http.StatusOK, addedTaskPage, nil, templateData)
	}
}

func (app *application) listTasks(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	databaseTasks, err := app.DB.queries.GetTasksWithLatestJobMetadata(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	updatedTasks, err := app.checkAndUpdateRunningTasks(databaseTasks)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	data := app.newTemplateData(r)
	data["Tasks"] = updatedTasks
	app.render(w, r, http.StatusOK, allTasksPage, nil, data)
}

func (app *application) fireTask(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("jobUUID") == "" {
		app.serverError(w, r, fmt.Errorf("job uuid is empty"))
		return
	}
	juuid, err := uuid.Parse(r.PathValue("jobUUID"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if exists, task := app.isTaskRunning(juuid); exists {
		err := task.RunNow()
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		data := app.newTemplateData(r)
		data["ParagraphText"] = fmt.Sprintf("Started task with UUUID %v (job name: %v)", task.ID(), task.Name())
		app.renderHTMX(w, r, http.StatusOK, htmxParagraph, nil, "htmx:Paragraph", data)
	} else {
		data := app.newTemplateData(r)
		data["ParagraphText"] = fmt.Sprintf("task with UUUID %v (job name: %v) not found (This is an error!)", task.ID(), task.Name())
		app.renderHTMX(w, r, http.StatusOK, htmxParagraph, nil, "htmx:Paragraph", data)
	}
}

func (app *application) stopTask(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	if r.PathValue("taskUUID") == "" {
		app.badRequest(w, r, fmt.Errorf("task uuid is empty"))
		return
	}
	err := app.deleteTaskFromScheduler(ctxwt, r.PathValue("taskUUID"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	data := app.newTemplateData(r)
	data["ParagraphText"] = fmt.Sprintf("Stopped Task with UUID %v", r.PathValue("taskUUID"))
	app.renderHTMX(w, r, http.StatusOK, htmxParagraph, http.Header{
		"HX-Refresh": []string{"true"},
	}, "htmx:Paragraph", data)
}

func (app *application) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	if r.PathValue("taskUUID") == "" {
		app.badRequest(w, r, fmt.Errorf("task uuid is empty"))
		return
	}
	taskUUID, err := uuid.Parse(r.PathValue("taskUUID"))

	if err != nil {
		app.serverError(w, r, err)
		return
	}
	err = app.deleteTaskFromScheduler(ctxwt, r.PathValue("taskUUID"))
	if err != nil && !errors.Is(err, gocron.ErrJobNotFound) {
		app.serverError(w, r, err)
		return
	}
	err = app.DB.queries.DeleteTaskWhereUUID(ctxwt, taskUUID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *application) doBulkAction(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var formData tasksBulkForm
	var operationResults []string
	err := request.DecodeForm(r, &formData)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	uuidList, err := stringListToUUIDList(formData.SelectedTasks)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch requestedAction := strings.TrimSpace(strings.ToLower(formData.Action)); requestedAction {
	case "fire":
		for _, taskUUID := range uuidList {
			if exists, task := app.isTaskRunning(taskUUID); exists {
				err := task.RunNow()
				if err != nil {
					app.serverError(w, r, err)
					return
				}
				operationResults = append(operationResults, fmt.Sprintf("Fired Task With UUID %v (Task name: %v)", taskUUID.String(), task.Name()))
			} else {
				operationResults = append(operationResults, fmt.Sprintf("Task With UUUID %v was not found in the scheduler, is it stopped?", taskUUID.String()))
			}
		}
	case "stop":
		for _, taskUUID := range uuidList {
			if exists, task := app.isTaskRunning(taskUUID); exists {
				err = app.deleteTaskFromScheduler(ctxwt, taskUUID.String())
				if err != nil {
					app.serverError(w, r, err)
					return
				}
				operationResults = append(operationResults, fmt.Sprintf("Stopped Task with UUID %v (Task name: %v)", taskUUID.String(), task.Name()))
			} else {
				operationResults = append(operationResults, fmt.Sprintf("Can not stop task with UUID %v", taskUUID.String()))
			}
		}
	case "delete":
		for _, taskUUID := range uuidList {
			if exists, _ := app.isTaskRunning(taskUUID); exists {
				err = app.deleteTaskFromScheduler(ctxwt, taskUUID.String())
				if err != nil {
					app.serverError(w, r, err)
					return
				}
			}
			err = app.DB.queries.DeleteTaskWhereUUID(ctxwt, taskUUID)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		}
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	data := app.newTemplateData(r)
	data["Values"] = operationResults
	data["Placeholder"] = fmt.Sprintf("Results of operation %v", formData.Action)
	app.renderHTMX(w, r, http.StatusOK, htmxListOfItems, nil, "htmx:listOfItems", data)
}

func (app *application) editTask(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	taskAsUUID, err := uuid.Parse(r.PathValue("taskUUID"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		taskDb, err := app.DB.queries.GetTaskWithUUID(ctxwt, taskAsUUID)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		taskSettings, err := url.ParseQuery(taskDb.SettingsArguments)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		taskSettings = cleanUrlValues(taskSettings, "spider", "project", "version", "csrf_token")
		templateData := app.newTemplateData(r)
		templateData["Task"] = taskDb
		templateData["Nodes"] = nodes
		templateData["Settings"] = taskSettings
		app.render(w, r, http.StatusOK, editTaskPage, nil, templateData)
	case http.MethodPost:
		var formData taskEditAddFormData
		var isPaused bool
		err := request.DecodePostForm(r, &formData)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		// Try to catch invalid/unknown cron format before it reaches gocron and throws a server error there!
		_, cronParseError := cron.ParseStandard(formData.CronTab)
		if cronParseError != nil {
			app.logger.ErrorContext(ctxwt, "got invalid/unknown cron schedule according to parse standard", slog.Any("cronString", formData.CronTab), slog.Any("err", cronParseError))
		}
		formData.Validator.CheckField(len(formData.FireNodes) != 0, "fireNodes", "You must select at least one node")
		formData.Validator.CheckField(validator.NotBlank(formData.Project), "project", "You must select at least one project")
		formData.Validator.CheckField(validator.NotBlank(formData.Spider), "spider", "You must select at least one spider")
		formData.Validator.CheckField(validator.NotBlank(formData.CronTab), "cron_input", "You must schedule spider")
		formData.Validator.CheckField(cronParseError == nil, "cron_input", "Not a valid/supported cron string. Please see https://en.wikipedia.org/wiki/Cron")
		formData.Validator.CheckField(validator.NotBlank(formData.TaskName), "task_name", "Task name can not be blank")
		if formData.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = formData
			data["Nodes"] = nodes
			app.render(w, r, http.StatusUnprocessableEntity, editTaskPage, nil, data)
			return
		}
		cleanForm := cleanUrlValues(r.PostForm, "fireNode", "csrf_token", "cron_input", "task_name", "immediately")
		if exists, _ := app.isTaskRunning(taskAsUUID); exists {
			isPaused = false
			replacedTask, err := app.newTask(false, &taskAsUUID, formData.TaskName, formData.Spider, formData.Project, formData.FireNodes[0], cleanForm, nil)
			if app.checkCreateTaskError(w, r, replacedTask, err) {
				return
			}
			_, err = replacedTask.updatesResource(taskAsUUID, formData.CronTab)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		} else {
			isPaused = true
		}
		queryParams := database.UpdateTaskParams{
			Name:              database.CreateSqlNullString(&formData.TaskName),
			Project:           formData.Project,
			Spider:            formData.Spider,
			Jobid:             formData.TaskName,
			SettingsArguments: cleanForm.Encode(),
			SelectedNodes:     formData.FireNodes[0],
			CronString:        formData.CronTab,
			Paused:            isPaused,
			ID:                taskAsUUID,
		}
		if user := contextGetAuthenticatedUser(r); user != nil {
			queryParams.ModifiedBy = user.ID
		}
		err = app.DB.queries.UpdateTask(ctxwt, queryParams)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		http.Redirect(w, r, "/list-tasks", http.StatusSeeOther)
	}
}

func (app *application) searchTasksTable(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var formData tasksSearchForm
	err := request.DecodePostForm(r, &formData)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	tasks, err := app.DB.queries.SearchTasksTable(ctxwt, formData.SearchTerm)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	templateData := app.newTemplateData(r)
	templateData["Tasks"] = tasks
	app.renderHTMX(w, r, http.StatusOK, htmxTaskTable, nil, "htmx:TaskTable", templateData)
}

func (app *application) restartTask(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancel := context.WithTimeout(r.Context(), app.config.DefaultTimeout)
	defer cancel()
	var taskName string
	if r.PathValue("taskUUID") == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	taskUUID, err := uuid.Parse(r.PathValue("taskUUID"))
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	taskDb, err := app.DB.queries.GetTaskWithUUID(ctxwt, taskUUID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if taskDb.Name.Valid {
		taskName = taskDb.Name.String
	}
	values, err := url.ParseQuery(taskDb.SettingsArguments)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	restartedTask, err := app.newTask(false, &taskUUID, taskName, taskDb.Spider, taskDb.Project, taskDb.SelectedNodes, values, nil)
	if app.checkCreateTaskError(w, r, restartedTask, err) {
		return
	}
	cronJob, err := restartedTask.newCronJob(taskDb.CronString)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	err = app.DB.queries.UpdateTaskPaused(ctxwt, database.UpdateTaskPausedParams{
		Paused: false,
		ID:     taskDb.ID,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	data := app.newTemplateData(r)
	data["ParagraphText"] = fmt.Sprintf("Started task with UUUID %v (job name: %v)", cronJob.ID(), cronJob.Name())
	app.renderHTMX(w, r, http.StatusOK, htmxParagraph, http.Header{
		"HX-Refresh": []string{"true"},
	}, "htmx:Paragraph", data)
}
