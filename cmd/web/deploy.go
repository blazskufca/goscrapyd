package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/blazskufca/goscrapyd/assets"
	"github.com/blazskufca/goscrapyd/internal/cookies"
	"github.com/blazskufca/goscrapyd/internal/request"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/google/uuid"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strings"
)

type deployCookieType struct {
	Version         uuid.UUID
	ProjectName     string
	ProjectLocation string
	Nodes           []string
}

type addVersionResponse struct {
	NodeName       string `json:"node_name"`
	ActualNodeName string `json:"-"`
	Status         string `json:"status"`
	Error          error  `json:"-"`
	Spiders        int    `json:"spiders"`
}

func (app *application) deploy(w http.ResponseWriter, r *http.Request) {
	ctxwt, cancelFunc := context.WithTimeout(context.Background(), app.config.DefaultTimeout)
	defer cancelFunc()
	formData := struct {
		ProjectName   string              `form:"project_name"`
		ProjectPath   string              `form:"project_location"`
		NodesToDeploy []string            `form:"nodes_to_deploy"`
		Validator     validator.Validator `form:"-"`
	}{}
	nodes, err := app.DB.queries.ListScrapydNodes(ctxwt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if hasSettings, err := app.DB.queries.CheckSettingsExist(ctxwt); err != nil {
			app.serverError(w, r, err)
			return
		} else if hasSettings == 1 {
			settings, err := app.DB.queries.GetSettings(ctxwt)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			if settings.DefaultProjectName.Valid {
				formData.ProjectName = settings.DefaultProjectName.String
			}
			if settings.DefaultProjectPath.Valid {
				formData.ProjectPath = settings.DefaultProjectPath.String
			}
		}
		templateData := app.newTemplateData(r)
		templateData["Nodes"] = nodes
		templateData["Form"] = formData
		app.render(w, r, http.StatusOK, deployPage, nil, templateData)
	case http.MethodPost:
		err := request.DecodePostForm(r, &formData)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		cleanPath, sanitizeErr := sanitizePath(formData.ProjectPath)
		if sanitizeErr != nil {
			formData.Validator.CheckField(sanitizeErr == nil, "project_location", sanitizeErr.Error())
		}
		formData.Validator.CheckField(validator.NotBlank(formData.ProjectName), "project_name", "You must provide a project name")
		formData.Validator.CheckField(validator.NotBlank(formData.ProjectName), "project_location", "You must provide a project location")
		formData.Validator.CheckField(len(formData.NodesToDeploy) != 0, "nodes_to_deploy", "You must provide at least one node to deploy")
		if formData.Validator.HasErrors() {
			data := app.newTemplateData(r)
			data["Form"] = formData
			data["Nodes"] = nodes
			app.render(w, r, http.StatusUnprocessableEntity, deployPage, nil, data)
			return
		}

		versionUUID, err := uuid.NewUUID()
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		var buf bytes.Buffer
		deployResult := deployCookieType{
			ProjectName:     formData.ProjectName,
			Version:         versionUUID,
			Nodes:           formData.NodesToDeploy,
			ProjectLocation: cleanPath,
		}
		err = gob.NewEncoder(&buf).Encode(&deployResult)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		err = cookies.WriteEncrypted(w, http.Cookie{
			Name:     "deploy-session",
			Value:    buf.String(),
			Path:     "/deploy-sse",
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			MaxAge:   0,
		}, app.config.cookie.secretKey)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		templatedata := app.newTemplateData(r)
		templatedata["Result"] = deployResult
		app.render(w, r, http.StatusOK, deployInProgressPage, nil, templatedata)
	}
}

func (app *application) buildAndDeployEggSSE(w http.ResponseWriter, r *http.Request) {
	ctxwc, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	flusher, ok := w.(http.Flusher)
	if !ok {
		app.serverError(w, r, fmt.Errorf("streaming not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	managedToLock := app.globalMu.TryLock()
	if !managedToLock {
		app.writeSSEResponse(w, r, flusher, nil, deployAlreadyLockedSSE, "locked_for_deploy", "sse:lockedForDeploy")
		app.globalMu.Lock()
	}
	defer app.globalMu.Unlock()

	gobEncodedValue, err := cookies.ReadEncrypted(r, "deploy-session", app.config.cookie.secretKey)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	var cookieData deployCookieType
	reader := strings.NewReader(gobEncodedValue)
	if err := gob.NewDecoder(reader).Decode(&cookieData); err != nil {
		app.serverError(w, r, err)
		return
	}

	egg, err := app.buildEggInternal(ctxwc, cookieData.ProjectLocation)
	if err != nil {
		app.reportServerError(r, err)
		app.writeSSEResponse(w, r, flusher, err, buildFailedSSE, "build_error", "sse:BuildFailed")
		return
	}

	numJobs := len(cookieData.Nodes)
	jobs := make(chan string, numJobs)
	results := make(chan addVersionResponse, numJobs)
	for w := 0; w < app.config.workerCount; w++ {
		go app.deployToNodeWorker(ctxwc, cookieData.ProjectName, cookieData.Version, egg, jobs, results)
	}

	for _, node := range cookieData.Nodes {
		jobs <- node
	}
	close(jobs)

	for i := 0; i < numJobs; i++ {
		res := <-results
		if res.Error != nil {
			app.reportServerError(r, res.Error)
			app.writeSSEResponse(w, r, flusher, res.Error, deployErrorSSE, "status_"+res.ActualNodeName, "sse:deployError")
		} else {
			app.writeSSEResponse(w, r, flusher, res.Status, justTemplateDataSSE, "status_"+res.ActualNodeName, "sse:justTemplateData")
			app.writeSSEResponse(w, r, flusher, res.Spiders, justTemplateDataSSE, "spider_"+res.ActualNodeName, "sse:justTemplateData")
		}
	}
	close(results)
	app.writeSSEResponse(w, r, flusher, nil, deploymentDoneSSE, "deployment-complete", "sse:DeployDone")
}

func (app *application) deployToNodeWorker(ctx context.Context, project string, version uuid.UUID, egg []byte, jobs <-chan string, results chan<- addVersionResponse) {
	defer func() {
		err := recover()
		if err != nil {
			app.logger.Error("panic in deployToNodeWorker", "recoverData", err)
		}
	}()
	for job := range jobs {
		result := addVersionResponse{ActualNodeName: job}
		form := new(bytes.Buffer)
		writer := multipart.NewWriter(form)
		fieldsToWrite := map[string]string{
			"project": project,
			"version": version.String(),
		}

		for fieldName, fieldValue := range fieldsToWrite {
			formField, err := writer.CreateFormField(fieldName)
			if err != nil {
				result.Error = fmt.Errorf("error creating form field: %v", err)
				results <- result
				continue
			}
			if _, err := formField.Write([]byte(fieldValue)); err != nil {
				result.Error = fmt.Errorf("error writing form field: %v", err)
				results <- result
				continue
			}
		}
		formFile, err := writer.CreateFormFile("egg", fmt.Sprintf("goscrapyd-%s/%s.egg", job, project))
		if err != nil {
			result.Error = fmt.Errorf("error creating form file: %v", err)
			results <- result
			continue
		}
		if _, err := io.Copy(formFile, bytes.NewBuffer(egg)); err != nil {
			result.Error = fmt.Errorf("error copying egg data: %v", err)
			results <- result
			continue
		}

		if err := writer.Close(); err != nil {
			result.Error = fmt.Errorf("error closing writer: %v", err)
			results <- result
			continue
		}
		headers := &http.Header{
			"Content-Type": []string{writer.FormDataContentType()},
		}
		req, err := makeRequestToScrapyd(ctx, app.DB.queries, http.MethodPost, job,
			func(url *url.URL) *url.URL {
				url.Path = path.Join(url.Path, ScrapydAddVersion)
				return url
			}, form, headers, app.config.ScrapydEncryptSecret)
		if err != nil {
			result.Error = fmt.Errorf("error creating request: %v", err)
			results <- result
			continue
		}
		deployedInfo, err := requestJSONResourceFromScrapyd[addVersionResponse](req, app.logger)
		if err != nil {
			result.Error = fmt.Errorf("error getting response from Scrapyd: %v", err)
			results <- result
			continue
		}
		result.Status = deployedInfo.Status
		result.Spiders = deployedInfo.Spiders
		results <- result
	}
}

func (app *application) buildEggInternal(ctx context.Context, scrapyCfg string) ([]byte, error) {
	pythonScript, err := assets.EmbeddedFiles.ReadFile("build_egg.py")
	if err != nil {
		return nil, fmt.Errorf("error reading embedded Python script: %v", err)
	}
	// TODO: scary thing, but it will do for now. There should be some other way however or at the very, very least, paths and commands should be thoroughly checked, verified, whatever
	// I think that this should be mostly fine now - Python is passed in through environment variable not a form
	// The project path is expended to absolute path, checked for any weired escape sequences, dir enumeration attempts, etc
	// before it's passed to this execute
	// Probably keeping this a to do indefinitely because it is a probable command injection spot nonetheless and todos make it stand
	// out more in a lot of editors

	// I think this is again a gosec false positive, scrapyCfg is verified with helpers.sanitizePath on form submission
	cmd := exec.CommandContext(ctx, app.config.pythonPath, "-c", string(pythonScript), scrapyCfg) // #nosec G204
	out := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error executing Python script: %v\nStderr: %s", err, stderr.String())
	}
	return out.Bytes(), nil
}
