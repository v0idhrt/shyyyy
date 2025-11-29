package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"api-gateway/internal/auth/models"
)

// ============================================================
// SQLite Repository
// ============================================================

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Init запускает миграции и убеждается в наличии admin.
func (r *Repository) Init(ctx context.Context, migrationsPath string) error {
	if err := r.runMigrations(migrationsPath); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return r.ensureAdmin(ctx)
}

func (r *Repository) GetByCredentials(ctx context.Context, login, password string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, login, password, fio, email, phone, birth_date, address, created_at
        FROM users
        WHERE login = ? AND password = ?
    `, login, password)

	var u models.User
	if err := row.Scan(&u.ID, &u.Login, &u.Password, &u.FIO, &u.Email, &u.Phone, &u.BirthDate, &u.Address, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, login, password, fio, email, phone, birth_date, address, created_at
        FROM users
        WHERE id = ?
    `, id)

	var u models.User
	if err := row.Scan(&u.ID, &u.Login, &u.Password, &u.FIO, &u.Email, &u.Phone, &u.BirthDate, &u.Address, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}
	return &u, nil
}

// ============================================================
// Migrations & Seeding
// ============================================================

func (r *Repository) runMigrations(migrationsPath string) error {
	data, err := os.ReadFile(migrationsPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	sqlText := string(data)
	_, err = r.db.Exec(sqlText)
	if err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}

func (r *Repository) ensureAdmin(ctx context.Context) error {
	_, err := r.GetByCredentials(ctx, "admin", "admin")
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "not found") {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
        INSERT INTO users (id, login, password, fio, email, phone, birth_date, address)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `,
		"11111111-1111-1111-1111-111111111111",
		"admin",
		"admin",
		"Admin User",
		"admin@example.com",
		"+10000000000",
		"1970-01-01",
		"Admin Street 1",
	)
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	return nil
}

// OpenSQLite открывает sqlite по указанному пути.
func OpenSQLite(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_pragma=busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
