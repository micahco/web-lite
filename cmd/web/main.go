package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/micahco/web/internal/mailer"
	"github.com/micahco/web/internal/models"
	"github.com/micahco/web/ui"
)

type config struct {
	port int
	dev  bool
	db   struct {
		dsn string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
}

type application struct {
	baseURL        *url.URL
	config         config
	logger         *slog.Logger
	mailer         *mailer.Mailer
	models         models.Models
	sessionManager *scs.SessionManager
	templateCache  map[string]*template.Template
	formDecoder    *form.Decoder
	validate       *validator.Validate
}

func main() {
	var cfg config
	var urlstr string

	// Default flag values for production
	flag.IntVar(&cfg.port, "port", 8080, "API server port")
	flag.BoolVar(&cfg.dev, "dev", false, "Development mode")
	flag.StringVar(&urlstr, "url", "", "Base URL")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-user", "", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-pass", "", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-addr", "", "SMTP sender address")

	flag.Parse()

	// Logger
	h := newSlogHandler(cfg)
	logger := slog.New(h)
	// Create error log for http.Server
	errLog := slog.NewLogLogger(h, slog.LevelError)

	// Base URL
	baseURL, err := url.Parse(urlstr)
	if err != nil {
		logger.Error("unable to parse url", slog.Any("err", err))
		os.Exit(1)
	}

	// PostgreSQL
	pool, err := openPool(cfg)
	if err != nil {
		logger.Error("unable to open pgpool", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Mailer
	sender := &mail.Address{
		Name:    "Do Not Reply",
		Address: cfg.smtp.sender,
	}
	logger.Debug("dialing SMTP server...")
	mailer, err := mailer.New(
		cfg.smtp.host,
		cfg.smtp.port,
		cfg.smtp.username,
		cfg.smtp.password,
		sender,
		ui.Files,
		"mail/*.tmpl",
	)
	if err != nil {
		logger.Error("unable to create mailer", slog.Any("err", err))
		os.Exit(1)
	}

	// Session manager
	sm := scs.New()
	sm.Store = pgxstore.New(pool)
	sm.Lifetime = 12 * time.Hour
	gob.Register(uuid.UUID{})
	gob.Register(FlashMessage{})
	gob.Register(FormErrors{})

	// Template cache
	tc, err := newTemplateCache()
	if err != nil {
		logger.Error("unable to create template cache", slog.Any("err", err))
		os.Exit(1)
	}

	app := &application{
		baseURL:        baseURL,
		config:         cfg,
		logger:         logger,
		mailer:         mailer,
		models:         models.New(pool),
		sessionManager: sm,
		templateCache:  tc,
		formDecoder:    form.NewDecoder(),
		validate:       validator.New(),
	}

	srv := &http.Server{
		Addr:     fmt.Sprintf(":%d", cfg.port),
		Handler:  app.routes(),
		ErrorLog: errLog,
	}

	logger.Info("starting server", "addr", srv.Addr)
	err = srv.ListenAndServe()
	logger.Error(err.Error())
}

func openPool(cfg config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	err = dbpool.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return dbpool, err
}

func newSlogHandler(cfg config) slog.Handler {
	if cfg.dev {
		// Development text hanlder
		return tint.NewHandler(os.Stdout, &tint.Options{
			AddSource:  true,
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		})
	}

	// Production use JSON handler with default opts
	return slog.NewJSONHandler(os.Stdout, nil)
}

func (app *application) background(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error("background", slog.Any("err", err))
			}
		}()
		fn()
	}()
}
