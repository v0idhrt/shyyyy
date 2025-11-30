package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"api-gateway/internal/auth/handlers"
	"api-gateway/internal/auth/repository"
	"api-gateway/internal/auth/service"
	"api-gateway/internal/common/config"
	"api-gateway/internal/common/middleware"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// ============================================================
// Auth Service
// ============================================================

func main() {
	cfg := config.Load()
	if os.Getenv("PORT") == "" {
		cfg.Port = "3002"
	}

	dbPath := getenv("AUTH_DB_PATH", "data/db/auth.db")
	db, err := repository.OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := repository.New(db)
	if err := repo.Init(context.Background(), "migrations/001_init_auth.sql"); err != nil {
		log.Fatalf("init db: %v", err)
	}

	sessionManager := service.NewSessionManager()
	fileStorage := service.NewFileStorage("source")
	converterURL := getenv("CONVERTER_URL", "http://localhost:3001")
	authHandler := handlers.NewAuthHandler(repo, sessionManager, fileStorage, converterURL)

	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		AppName:      "Auth Service",
	})

	// ============================================================
	// Global Middleware
	// ============================================================

	app.Use(recover.New())
	app.Use(middleware.Logger())

	// ============================================================
	// Health Check Routes
	// ============================================================

	app.Get("/health/live", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "alive"})
	})

	app.Get("/health/ready", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// ============================================================
	// Auth Routes
	// ============================================================

	app.Post("/login", authHandler.Login)
	app.Get("/users/:id", authHandler.GetUser)

	// Internal routes (для межсервисного общения)
	app.Get("/internal/users/:id", authHandler.GetUserInternal)
	app.Get("/users/:id/svg", authHandler.GetSVG)
	app.Get("/users/:id/pdf", authHandler.GetPDF)
	app.Get("/users/:id/pdf-files", authHandler.GetPDFByName)
	app.Get("/users/:id/png", authHandler.GetPNG)
	app.Get("/users/:id/files", authHandler.ListFiles)
	app.Post("/users/:id/svg", authHandler.UploadSVG)
	app.Post("/users/:id/pdf", authHandler.UploadPDF)
	app.Post("/users/:id/png", authHandler.UploadPNG)
	app.Post("/users/:id/png-to-svg", authHandler.UploadPNGAndReturnSVG)
	app.Post("/users/:id/png-to-json", authHandler.UploadPNGToJSON)
	app.Post("/users/:id/svg-edited", authHandler.UploadEditedSVG)
	app.Get("/users/:id/svg-edited", authHandler.GetEditedSVG)
	app.Get("/users/:id/svg-json", authHandler.GetSVGAsJSON)
	app.Get("/users/:id/svg-edited-json", authHandler.GetEditedSVGAsJSON)
	app.Post("/users/:id/json", authHandler.UploadJSON)
	app.Get("/users/:id/json", authHandler.GetJSON)
	app.Post("/users/:id/json-edited", authHandler.UploadEditedJSON)
	app.Get("/users/:id/json-edited", authHandler.GetEditedJSON)
	app.Post("/users/:id/json-to-svg", authHandler.RenderEditedSVG)

	// ============================================================
	// Server Start
	// ============================================================

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Auth Service on %s (env: %s)", addr, cfg.Environment)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getenv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}
