package handlers

import (
	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Health Check Handlers
// ============================================================

// LivenessProbe проверяет, что приложение работает
func LivenessProbe(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "alive",
	})
}

// ReadinessProbe проверяет готовность приложения обрабатывать запросы
func ReadinessProbe(c fiber.Ctx) error {
	// Здесь можно добавить проверки подключения к БД, внешним сервисам и т.д.
	return c.JSON(fiber.Map{
		"status": "ready",
	})
}

// StartupProbe проверяет, что приложение успешно запустилось
func StartupProbe(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "started",
	})
}
