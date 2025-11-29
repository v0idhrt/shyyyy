package main

import (
	"fmt"
	"log"
	"time"

	"api-gateway/internal/common/config"
	"api-gateway/internal/common/middleware"
	"api-gateway/internal/converter/handlers"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// ============================================================
// Converter Service
// ============================================================

func main() {
	cfg := config.Load()

	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		AppName:      "Converter Service",
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
	// Converter Routes
	// ============================================================

	app.Post("/convert", handlers.ConvertSVG)
	app.Post("/render", handlers.RenderSVG)

	// ============================================================
	// Server Start
	// ============================================================

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting Converter Service on %s (env: %s)", addr, cfg.Environment)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
