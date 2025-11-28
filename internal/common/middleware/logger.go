package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

// ============================================================
// Logger Middleware
// ============================================================

// Logger возвращает настроенный middleware для логирования запросов
func Logger() fiber.Handler {
	return logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path} | Content-Type: ${reqHeader:Content-Type}\n",
		TimeFormat: "15:04:05",
		TimeZone:   "Local",
	})
}
