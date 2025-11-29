package handlers

import (
	"encoding/json"
	"log"

	"api-gateway/internal/converter/mapper"
	"api-gateway/internal/converter/models"

	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Render Handler
// ============================================================

// RenderSVG конвертирует react-planner JSON обратно в SVG
func RenderSVG(c fiber.Ctx) error {
	log.Printf("[RENDER] Received request")
	log.Printf("[RENDER] Content-Type: %s", c.Get("Content-Type"))
	log.Printf("[RENDER] Content-Length: %d", len(c.Body()))

	if len(c.Body()) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "body required",
		})
	}

	var scene models.Scene
	if err := json.Unmarshal(c.Body(), &scene); err != nil {
		log.Printf("[RENDER] Decode error: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid JSON payload",
		})
	}

	renderer := mapper.NewRenderer()
	svg, err := renderer.Render(&scene)
	if err != nil {
		log.Printf("[RENDER] Render error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendString(svg)
}
