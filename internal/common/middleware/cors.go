package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// CORS разрешает все источники и заголовки (dev).
func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"*"},
		AllowMethods: []string{"*"},
	})
}
