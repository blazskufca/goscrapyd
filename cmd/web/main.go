package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"expvar"
	"flag"
	"fmt"
	"github.com/blazskufca/goscrapyd/assets"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/password"
	"github.com/blazskufca/goscrapyd/internal/smtp"
	"github.com/blazskufca/goscrapyd/internal/version"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"github.com/robfig/cron/v3"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

func init() {
	gob.Register(uuid.UUID{})
	gob.Register(deployCookieType{})
	expvar.NewString("buildFromCommit").Set(version.Get())
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}))

	err := run(logger)
	if err != nil {
		trace := string(debug.Stack())
		logger.Error(err.Error(), "trace", trace)
		os.Exit(1)
	}
}

type config struct {
	baseURL   string
	httpPort  int
	autoHTTPS struct {
		domain  string
		email   string
		staging bool
	}
	cookie struct {
		secretKey string
	}
	notifications struct {
		email string
	}
	session struct {
		secretKey    string
		oldSecretKey string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		from     string
	}
	db struct {
		dsn               string
		maxOpenConns      int
		maxIdleConns      int
		maxIdleTime       time.Duration
		autoMigrate       bool
		createDefaultUser bool
	}
	workerCount int
	pythonPath  string
	limiter     struct {
		rps     float64
		burst   int
		enabled bool
	}
	DefaultTimeout       time.Duration
	ScrapydEncryptSecret string
	autoUpdateNodes      string
}

type application struct {
	config       config
	logger       *slog.Logger
	mailer       *smtp.Mailer
	sessionStore *sessions.CookieStore
	wg           sync.WaitGroup
	DB           struct {
		queries *database.Queries
		dbConn  *sql.DB
	}
	scheduler     gocron.Scheduler
	reverseProxy  *httputil.ReverseProxy
	globalMu      sync.Mutex
	templateCache map[templateName]*template.Template
	eggBuildFunc  func(ctx context.Context, pythonPath, scrapyCfg string) ([]byte, error)
}

func run(logger *slog.Logger) error {
	var cfg config
	flag.StringVar(&cfg.baseURL, "base-url", "http://localhost:4444", "base URL for the application")
	flag.IntVar(&cfg.httpPort, "http-port", 4444, "port to listen on for HTTP requests")
	flag.StringVar(&cfg.autoHTTPS.domain, "auto-https-domain", "", "domain to enable automatic HTTPS for")
	flag.StringVar(&cfg.autoHTTPS.email, "auto-https-email", "admin@example.com", "contact email address for problems with LetsEncrypt certificates")
	flag.BoolVar(&cfg.autoHTTPS.staging, "auto-https-staging", false, "use LetsEncrypt staging environment")
	flag.StringVar(&cfg.cookie.secretKey, "cookie-secret-key", "cpoga3pwmoq5s6wfxmhj5tplt6uusyy5", "secret key for cookie authentication/encryption")
	flag.StringVar(&cfg.notifications.email, "notifications-email", "", "contact email address for error notifications")
	flag.StringVar(&cfg.session.secretKey, "session-secret-key", "ueo5gngxtoh37od5dwvez55cyne6afav", "secret key for session cookie authentication")
	flag.StringVar(&cfg.session.oldSecretKey, "session-old-secret-key", "", "previous secret key for session cookie authentication")
	flag.StringVar(&cfg.smtp.host, "smtp-host", "example.smtp.host", "smtp host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "smtp port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "example_username", "smtp username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "pa55word", "smtp password")
	flag.StringVar(&cfg.smtp.from, "smtp-from", "Example Name <no-reply@example.org>", "smtp sender")
	flag.StringVar(&cfg.db.dsn, "db-dsn", "database.db", "Database DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "Max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "Max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 30*time.Minute, "Max idle connections")
	flag.IntVar(&cfg.workerCount, "worker-count", 4, "Number of workers")
	flag.StringVar(&cfg.pythonPath, "python-path", "", "Python interpreter path")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.DurationVar(&cfg.DefaultTimeout, "default-timeout", 30*time.Second, "Default timeout")
	flag.StringVar(&cfg.ScrapydEncryptSecret, "scrapyd-secret", "cpoga3pwmoq5s6wfxmhj5tplt6uusyy5", "Used to encrypt your scrapyd credentials in the database")
	flag.BoolVar(&cfg.db.autoMigrate, "auto-migrate", true, "Automatically migrate the database")
	flag.BoolVar(&cfg.db.createDefaultUser, "create-default-user", false, "Create admin:admin user on startup (useful for first startup so you can login. Don't forget to create legit users afterwards and delete this insecure one)")
	flag.StringVar(&cfg.autoUpdateNodes, "auto-update-interval", "*/10 * * * *", "Updates jobs info for all the nodes in the background on a given schedule. Expects CRON string.")
	showVersion := flag.Bool("version", false, "display version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("version: %s\n", version.Get())
		return nil
	}

	_, err := cron.ParseStandard(cfg.autoUpdateNodes)
	if err != nil {
		log.Fatalln(err)
	}

	mailer, err := smtp.NewMailer(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.from)
	if err != nil {
		return err
	}

	keyPairs := [][]byte{[]byte(cfg.session.secretKey), nil}
	if cfg.session.oldSecretKey != "" {
		keyPairs = append(keyPairs, []byte(cfg.session.oldSecretKey), nil)
	}

	sessionStore := sessions.NewCookieStore(keyPairs...)
	sessionStore.Options = &sessions.Options{
		HttpOnly: true,
		MaxAge:   86400 * 7,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}
	databaseQueries, databaseConnection, err := openDB(cfg)
	if err != nil {
		log.Fatalln(err)
	}
	templateCache, err := newTemplateCache()
	if err != nil {
		log.Fatalln(err)
	}
	expvar.Publish("database", expvar.Func(func() any {
		return databaseConnection.Stats()
	}))
	app := &application{
		config:       cfg,
		logger:       logger,
		mailer:       mailer,
		sessionStore: sessionStore,
		globalMu:     sync.Mutex{},
		DB: struct {
			queries *database.Queries
			dbConn  *sql.DB
		}{queries: databaseQueries, dbConn: databaseConnection},
		templateCache: templateCache,
		eggBuildFunc:  buildEggInternal,
	}
	app.reverseProxy = &httputil.ReverseProxy{
		Rewrite:       proxyRewriter,
		FlushInterval: -1,
		ErrorLog:      slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
		ErrorHandler:  app.reverseProxyErrHandler,
	}
	s, err := gocron.NewScheduler(gocron.WithLogger(app.logger))
	if err != nil {
		log.Fatalln(err)
	}
	app.scheduler = s
	app.scheduler.Start()
	err = app.loadTasksOnStart()
	if err != nil {
		log.Fatalln(err)
	}
	job, err := app.scheduler.NewJob(gocron.CronJob(app.config.autoUpdateNodes, false), gocron.NewTask(app.updateAllNodesSchedule),
		gocron.WithSingletonMode(gocron.LimitModeReschedule), gocron.WithEventListeners(gocron.AfterJobRunsWithError(func(jobID uuid.UUID, jobName string, err error) {
			log.Println("ERROR IN updateAllNodesSchedule", "jobID:", jobID, "jobName:", jobName, "err:", err)
		}), gocron.AfterJobRunsWithPanic(func(jobID uuid.UUID, jobName string, recoverData any) {
			log.Println("PANIC IN updateAllNodesSchedule:", "jobID:", jobID, "jobName:", jobName, "recoverData:", recoverData)
		})))
	if err != nil {
		log.Fatalln(err)
	}
	err = job.RunNow()
	if err != nil {
		log.Fatalln(err)
	}
	if cfg.autoHTTPS.domain != "" {
		return app.serveAutoHTTPS()
	}
	return app.serveHTTP()
}

func openDB(cfg config) (*database.Queries, *sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.db.dsn)
	if err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous = NORMAL;`); err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if cfg.db.autoMigrate {
		goose.SetBaseFS(assets.EmbeddedFiles)

		if err := goose.SetDialect("sqlite3"); err != nil {
			_ = db.Close()
			return nil, nil, err
		}

		if err := goose.Up(db, "migrations"); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}
	preparedDb, err := database.Prepare(context.Background(), db)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	if cfg.db.createDefaultUser {
		username := "admin"
		userUUID, err := uuid.NewRandom()
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		passwordHash, err := password.Hash("admin")
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		_, err = preparedDb.CreateNewUser(context.Background(), database.CreateNewUserParams{
			ID:                 userUUID,
			Username:           username,
			HashedPassword:     passwordHash,
			HasAdminPrivileges: true,
		})
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		log.Println("Created default user", userUUID, "with username", username)
	}
	return preparedDb, db, nil
}
