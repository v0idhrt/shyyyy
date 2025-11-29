package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"api-gateway/internal/common/config"
	"api-gateway/internal/common/middleware"
	"api-gateway/internal/gateway/handlers"
	"api-gateway/internal/gateway/proxy"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// ============================================================
// API Gateway
// ============================================================

func main() {
	cfg := config.Load()

	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		AppName:      "API Gateway",
	})

	// ============================================================
	// Global Middleware
	// ============================================================

	app.Use(recover.New())
	app.Use(middleware.Logger())

	// ============================================================
	// Health Check Routes
	// ============================================================

	app.Get("/health/live", handlers.LivenessProbe)
	app.Get("/health/ready", handlers.ReadinessProbe)
	app.Get("/health/startup", handlers.StartupProbe)

	// ============================================================
	// API Routes
	// ============================================================

	api := app.Group("/api/v1")

	api.Get("/", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "API Gateway v1",
			"status":  "ok",
		})
	})

	// ============================================================
	// Service Routes (Proxy)
	// ============================================================

	// Converter Service
	converterURL := getEnv("CONVERTER_URL", "http://localhost:3001")
	api.Post("/convert", proxy.ProxyTo(converterURL+"/convert"))
	api.Post("/render", proxy.ProxyTo(converterURL+"/render"))

	// Auth Service
	authURL := getEnv("AUTH_URL", "http://localhost:3002")
	api.Post("/login", proxy.ProxyTo(authURL+"/login"))
	api.Get("/users/:id", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s", authURL, c.Params("id")))
	})
	api.Get("/users/:id/svg", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg", authURL, c.Params("id")))
	})
	api.Get("/users/:id/pdf", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/pdf", authURL, c.Params("id")))
	})
	api.Get("/users/:id/png", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/png", authURL, c.Params("id")))
	})
	api.Get("/users/:id/files", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/files", authURL, c.Params("id")))
	})
	api.Post("/users/:id/svg", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg", authURL, c.Params("id")))
	})
	api.Post("/users/:id/pdf", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/pdf", authURL, c.Params("id")))
	})
	api.Post("/users/:id/png", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/png", authURL, c.Params("id")))
	})
	api.Post("/users/:id/png-to-svg", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/png-to-svg", authURL, c.Params("id")))
	})
	api.Post("/users/:id/png-to-json", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/png-to-json", authURL, c.Params("id")))
	})
	api.Post("/users/:id/svg-edited", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg-edited", authURL, c.Params("id")))
	})
	api.Get("/users/:id/svg-edited", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg-edited?%s", authURL, c.Params("id"), c.Request().URI().QueryString()))
	})
	api.Get("/users/:id/svg-json", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg-json?%s", authURL, c.Params("id"), c.Request().URI().QueryString()))
	})
	api.Get("/users/:id/svg-edited-json", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/svg-edited-json?%s", authURL, c.Params("id"), c.Request().URI().QueryString()))
	})
	api.Post("/users/:id/json", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/json", authURL, c.Params("id")))
	})
	api.Get("/users/:id/json", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/json?%s", authURL, c.Params("id"), c.Request().URI().QueryString()))
	})
	api.Post("/users/:id/json-edited", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/json-edited", authURL, c.Params("id")))
	})
	api.Get("/users/:id/json-edited", func(c fiber.Ctx) error {
		return proxy.Forward(c, fmt.Sprintf("%s/users/%s/json-edited?%s", authURL, c.Params("id"), c.Request().URI().QueryString()))
	})

	// ============================================================
	// Server Start
	// ============================================================

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting API Gateway on %s (env: %s)", addr, cfg.Environment)
	log.Printf("Proxying /convert to %s", converterURL)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}
