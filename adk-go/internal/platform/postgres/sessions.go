package postgres

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/session/database"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const eventsBySessionIndex = `CREATE INDEX IF NOT EXISTS events_session_timestamp_idx
ON events (app_name, user_id, session_id, timestamp)`

// NewSessionService stores ADK sessions in PostgreSQL so a paused workflow can
// be resumed by any worker replica, not only by the process that started it.
// The returned *sql.DB must be closed by the caller. migrate creates the
// session tables; only one process (the api) should do it.
func NewSessionService(cfg Config, migrate bool) (session.Service, *sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("open session database: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)

	service, err := database.NewSessionService(
		gormpostgres.New(gormpostgres.Config{Conn: db}),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create session service: %w", err)
	}
	if !migrate {
		return service, db, nil
	}
	if err := database.AutoMigrate(service); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("migrate session tables: %w", err)
	}
	// ADK's events primary key starts with the event id, so loading a
	// session's events (by app, user and session) would scan the whole table.
	if _, err := db.Exec(eventsBySessionIndex); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create session events index: %w", err)
	}
	return service, db, nil
}
