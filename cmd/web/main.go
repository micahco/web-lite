package main

import (
	"database/sql"
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"
	"github.com/lmittmann/tint"
	"github.com/micahco/web-lite/internal/models"
)

type config struct {
	port int
	dev  bool
	db   struct {
		dsn string
	}
}

type application struct {
	config         config
	logger         *slog.Logger
	models         models.Models
	sessionManager *scs.SessionManager
	templateCache  map[string]*template.Template
	formDecoder    *form.Decoder
	validate       *validator.Validate
}

func main() {
	var cfg config

	// Default flag values for production
	flag.IntVar(&cfg.port, "port", 8080, "API server port")
	flag.BoolVar(&cfg.dev, "dev", false, "Development mode")
	flag.StringVar(&cfg.db.dsn, "db-dsn", "pricetag.db", "SQLite DSN")
	flag.Parse()

	// Logger
	h := newSlogHandler(cfg.dev)
	logger := slog.New(h)
	// Create error log for http.Server
	errLog := slog.NewLogLogger(h, slog.LevelError)

	// Database
	db, err := initDB(cfg.db.dsn)
	if err != nil {
		logger.Error("unable to initialize db", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	// Session manager
	sm := scs.New()
	sm.Store = sqlite3store.New(db)
	sm.Lifetime = 12 * time.Hour
	gob.Register(FlashMessage{})
	gob.Register(FormErrors{})

	// Template cache
	tc, err := newTemplateCache()
	if err != nil {
		logger.Error("unable to create template cache", slog.Any("err", err))
		os.Exit(1)
	}

	app := &application{
		config:         cfg,
		logger:         logger,
		models:         models.New(db),
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

func newSlogHandler(dev bool) slog.Handler {
	if dev {
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

func initDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	query := `
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			expiry REAL NOT NULL
		);

		CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);
		
		CREATE TABLE IF NOT EXISTS User (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL
		);
		
		CREATE TABLE IF NOT EXISTS Permission (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS UserPermission (
			user_id INTEGER NOT NULL,
			permission_id INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES User (id) ON DELETE CASCADE,
			FOREIGN KEY (permission_id) REFERENCES Permissions (id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, permission_id)
		);

		INSERT INTO Permission (name)
		VALUES
			('admin'),
			('services'),
			('tags'),
			('forwarding'),
			('logs')
		ON CONFLICT (name) DO NOTHING;`

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return db, nil
}
