package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"github.com/blazskufca/goscrapyd/internal/version"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/justinas/nosurf"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type scrapydDaemonStatusResponse struct {
	NodeName string `json:"node_name"`
	Status   string `json:"status"`
	Pending  int    `json:"pending"`
	Running  int    `json:"running"`
	Finished int    `json:"finished"`
}

func (app *application) newTemplateData(r *http.Request) map[string]any {
	data := map[string]any{
		"AuthenticatedUser": contextGetAuthenticatedUser(r),
		"Token":             nosurf.Token(r),
		"Version":           version.Get(),
	}

	return data
}

func (app *application) newEmailData() map[string]any {
	data := map[string]any{
		"BaseURL": app.config.baseURL,
	}

	return data
}

//func (app *application) backgroundTask(r *http.Request, fn func() error) {
//	app.wg.Add(1)
//
//	go func() {
//		defer app.wg.Done()
//
//		defer func() {
//			err := recover()
//			if err != nil {
//				app.reportServerError(r, fmt.Errorf("%s", err))
//			}
//		}()
//
//		err := fn()
//		if err != nil {
//			app.reportServerError(r, err)
//		}
//	}()
//}

func makeRequestToScrapyd(ctx context.Context, DB *database.Queries, method, nodeName string, setUrlParams func(url *url.URL) *url.URL, body io.Reader, presetHeaders *http.Header, secret string) (madeReq *http.Request, err error) {
	node, err := DB.GetNodeWithName(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	nodeUrl, err := url.Parse(node.Url)
	if err != nil {
		return nil, err
	}
	var urlString string
	switch {
	case nodeUrl != nil && setUrlParams != nil:
		urlString = setUrlParams(nodeUrl).String()
	case nodeUrl != nil && setUrlParams == nil:
		urlString = nodeUrl.String()
	default:
		return nil, errors.New("invalid node url")
	}
	madeReq, err = http.NewRequestWithContext(ctx, method, urlString, body)
	if err != nil {
		return nil, err
	}
	if presetHeaders != nil {
		madeReq.Header = *presetHeaders
	}
	if node.Username.Valid && validator.NotBlank(node.Username.String) && validator.NotBlank(secret) && node.Password != nil {
		// I'm very unsure about this... From security POV I think it makes sense to encrypt it at rest
		// AES-GCM is also fast enough on modern hardware with AES-NI and whatnot, I think they usually have throughput
		// of GB/s maybe even 10+GB/s on AES operations so a password string should not really be a problem. Performance
		//impact should be minimal, probably not noticeable at all however I still don't really like this is done on
		//every single request, ideally:
		// TODO: Add a password cache - On request hit cache first if not present yet hit the database, decrypt it and store it in the cache
		// Also make sure on node updates, deletes, whatever it's also reflected in cache so it's not out of sync
		password, err := decrypt(node.Password, secret)
		if err != nil {
			return nil, err
		}
		madeReq.SetBasicAuth(node.Username.String, password)
	}
	return madeReq, err
}

func requestJSONResourceFromScrapyd[T any](req *http.Request, logger *slog.Logger) (T, error) {
	var JSONResponse T
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return JSONResponse, err
	}
	defer func() {
		err := response.Body.Close()
		if err != nil && logger != nil {
			logger.Error("error when closing response body",
				slog.Any("function", "requestJSONResourceFromScrapyd"),
				slog.Any("request", req),
				slog.Any("err", err),
			)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return JSONResponse, fmt.Errorf("request returned status code %d", response.StatusCode)
	}
	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return JSONResponse, err
	}
	jsonIterType := jsoniter.ConfigFastest
	err = jsonIterType.NewDecoder(bytes.NewReader(rawBody)).Decode(&JSONResponse)
	if err != nil {
		return JSONResponse, fmt.Errorf("error when decoding response body:\n%w", errors.New(string(rawBody)))
	}
	return JSONResponse, nil
}

func (app *application) listScrapydNodesWorkerFunc(ctx context.Context, r *http.Request, jobs <-chan database.ScrapydNode, resultChan chan<- listScrapydNodesType) {
	defer func() {
		err := recover()
		if err != nil {
			app.reportServerError(r, fmt.Errorf("%s", err))
		}
	}()
	for node := range jobs {
		var workResult listScrapydNodesType
		workResult.Name = node.Nodename
		workResult.URL = node.Url
		workResult.Id = node.ID
		req, err := makeRequestToScrapyd(ctx, app.DB.queries, http.MethodGet, node.Nodename, func(url *url.URL) *url.URL {
			url.Path = path.Join(url.Path, scrapydDaemonStatusReq)
			return url
		}, nil, nil, app.config.ScrapydEncryptSecret)
		if err != nil {
			workResult.Error = fmt.Errorf("failed to create request: %w", err)
			app.reportServerError(r, err)
			resultChan <- workResult
			continue
		}
		scrapydDaemonStatus, err := requestJSONResourceFromScrapyd[scrapydDaemonStatusResponse](req, app.logger)
		if err != nil {
			workResult.Error = fmt.Errorf("request failed: %w", err)
			app.reportServerError(r, err)
			resultChan <- workResult
			continue
		}
		workResult.Finished = scrapydDaemonStatus.Finished
		workResult.Pending = scrapydDaemonStatus.Pending
		workResult.Running = scrapydDaemonStatus.Running
		workResult.Status = scrapydDaemonStatus.Status
		resultChan <- workResult
	}
}

func cleanUrlValues(urlValues url.Values, keysToRemove ...string) url.Values {
	for _, key := range keysToRemove {
		if urlValues.Has(key) {
			urlValues.Del(key)
		}
	}
	return urlValues
}

func (app *application) isTaskRunning(taskUUID uuid.UUID) (bool, gocron.Job) {
	for _, task := range app.scheduler.Jobs() {
		if task.ID() == taskUUID {
			return true, task
		}
	}
	return false, nil
}

func (app *application) checkAndUpdateRunningTasks(tasks []database.GetTasksWithLatestJobMetadataRow) ([]database.GetTasksWithLatestJobMetadataRow, error) {
	var UpdatedTasks []database.GetTasksWithLatestJobMetadataRow
	for _, task := range tasks {
		if !task.Paused { // The task should be running according to database
			if exists, _ := app.isTaskRunning(task.TaskID); !exists {
				task.Paused = true
			}
		} else if task.Paused {
			if exists, _ := app.isTaskRunning(task.TaskID); exists {
				task.Paused = false
			}
		}
		UpdatedTasks = append(UpdatedTasks, task)
	}
	return UpdatedTasks, nil
}

func (app *application) deleteTaskFromScheduler(ctx context.Context, uuidString string) error {
	uuidStringAsUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return err
	}
	err = app.scheduler.RemoveJob(uuidStringAsUUID)
	if err != nil {
		return err
	}
	return app.DB.queries.UpdateTaskPaused(ctx, database.UpdateTaskPausedParams{
		Paused: true,
		ID:     uuidStringAsUUID,
	})
}

func (app *application) loadTasksOnStart() error {
	tasks, err := app.DB.queries.GetTasks(context.Background())
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Paused {
			continue
		}
		values, err := url.ParseQuery(task.SettingsArguments)
		if err != nil {
			app.logger.Error("Error parsing query:", slog.Any("err", err))
			return err
		}
		var nameStr string
		if task.Name.Valid {
			nameStr = task.Name.String
		}
		createdTask, err := app.newTask(false, &task.ID, nameStr, task.Spider, task.Project, task.SelectedNodes, values, nil)
		if err != nil {
			return err
		} else if createdTask == nil {
			return errors.New("failed to load task")
		}
		cronJob, err := createdTask.newCronJob(task.CronString)
		if err != nil {
			return err
		}
		app.logger.Info("loaded task", slog.Any("name", cronJob.Name()), slog.Any("id", cronJob.ID()))
	}
	return nil
}

func stringListToUUIDList(list []string) ([]uuid.UUID, error) {
	var uuidList []uuid.UUID
	for _, v := range list {
		uuidUnit, err := uuid.Parse(v)
		if err != nil {
			return nil, err
		}
		uuidList = append(uuidList, uuidUnit)
	}
	return uuidList, nil
}

func sanitizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		return "", fmt.Errorf("invalid URL encoding in path")
	}
	path = decodedPath
	if strings.HasPrefix(path, `\\`) {
		return "", fmt.Errorf("path contains dangerous sequences (UNC path detected)")
	}
	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains directory traversal ('..')")
	}
	forbiddenChars := []string{";", "&", "|", ">", "<", "`", "$", "(", ")", "{", "}", "[", "]", "!", "#"}
	for _, char := range forbiddenChars {
		if strings.Contains(path, char) {
			return "", fmt.Errorf("path contains forbidden character: %v", char)
		}
	}
	re, err := regexp.Compile(`(?i)(\.\./|\.\.\\|/\.\./|/\\\.\.|\\..\\)`)
	if err != nil {
		return "", err
	}
	if re.MatchString(path) {
		return "", fmt.Errorf("path contains dangerous sequences like '..' or '\\'")
	}
	for _, r := range path {
		if r > unicode.MaxASCII {
			return "", fmt.Errorf("path contains non-ASCII characters")
		}
	}
	if filepath.Ext(path) != ".cfg" {
		return "", fmt.Errorf("you should provide a path which points to a scrapy.cfg file")
	}
	_, err = os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return "", err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	return absPath, nil
}

func (app *application) getPreconfiguredSettings(ctx context.Context) (url.Values, error) {
	if hasSettings, err := app.DB.queries.CheckSettingsExist(ctx); err != nil {
		return nil, err
	} else if hasSettings == 1 {
		settings, err := app.DB.queries.GetSettings(ctx)
		if err != nil {
			return nil, err
		}
		if settings.PersistedSpiderSettings.Valid {
			persistedSettings, err := url.ParseQuery(settings.PersistedSpiderSettings.String)
			if err != nil {
				return nil, err
			}
			return cleanUrlValues(persistedSettings, "spider", "project", "version", "csrf_token"), nil
		}
	}
	return url.Values{}, nil
}

func encrypt(value, secret string) ([]byte, error) {
	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, []byte(value), nil), nil
}

func decrypt(value []byte, secret string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("value is nil")
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()

	if len(value) < nonceSize {
		return "", fmt.Errorf("encrypted value (%d) is shorter then AES-GCM nonce (%d)", len(value), nonceSize)
	}

	nonce := value[:nonceSize]
	ciphertext := value[nonceSize:]

	// False positive gosec warning about using hardcoded nonce, for now it has to be ignored
	plaintext, err := aesGCM.Open(nil, []byte(nonce), []byte(ciphertext), nil) // #nosec G407
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func sortScrapydNodes(nodes []listScrapydNodesType) []listScrapydNodesType {
	slices.SortFunc(nodes, func(a, b listScrapydNodesType) int {
		if a.Id < b.Id {
			return -1
		} else if a.Id > b.Id {
			return 1
		}
		return 0
	})
	return nodes
}

func parseCSV(r *http.Request, fileName string, maxMemory int64) ([]map[string]string, error) {
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to parse form: %v", err)
	}
	file, formHeader, err := r.FormFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %v", err)
	}
	defer file.Close()
	if formHeader.Header.Get("Content-Type") != "text/csv" {
		return nil, fmt.Errorf("invalid file type: expected csv, got %s", formHeader.Header.Get("Content-Type"))
	}
	reader := csv.NewReader(file)

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %v", err)
	}

	header := records[0]

	var result []map[string]string
	for _, row := range records[1:] {
		rowMap := make(map[string]string)
		for i, field := range row {
			rowMap[header[i]] = field
		}
		result = append(result, rowMap)
	}

	return result, nil
}

func hasKeys(m map[string]string, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func addToURLValues(values url.Values, key string, value any) {
	strValue := ""
	switch v := value.(type) {
	case string:
		strValue = v
	case float64:
		strValue = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		strValue = strconv.FormatBool(v)
	case []any:
		for _, item := range v {
			addToURLValues(values, key, item)
		}
		return
	default:
		// Don't know what this would be, hopefully nothing ends up here
		strValue = fmt.Sprintf("%v", v)
	}

	if existing, exists := values[key]; exists {
		values[key] = append(existing, strValue)
	} else {
		values.Set(key, strValue)
	}
}
