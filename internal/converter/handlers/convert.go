package handlers

import (
	"bytes"
	"io"
	"log"

	"api-gateway/internal/converter/mapper"

	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Convert Handler
// ============================================================

// ConvertSVG конвертирует SVG в react-planner JSON
func ConvertSVG(c fiber.Ctx) error {
	log.Printf("[CONVERTER] Received request")
	log.Printf("[CONVERTER] Content-Type: %s", c.Get("Content-Type"))
	log.Printf("[CONVERTER] Content-Length: %d", len(c.Body()))

	// Получаем файл из multipart/form-data
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("[CONVERTER] FormFile error: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "file required in multipart/form-data",
		})
	}

	log.Printf("[CONVERTER] File received: %s, size: %d", file.Filename, file.Size)

	// Открываем файл
	f, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to open file",
		})
	}
	defer f.Close()

	// Читаем содержимое
	data, err := io.ReadAll(f)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "failed to read file",
		})
	}

	// Конвертируем
	log.Printf("[CONVERTER] Starting conversion, data size: %d bytes", len(data))
	converter := mapper.New()
	scene, err := converter.Convert(bytes.NewReader(data))
	if err != nil {
		log.Printf("[CONVERTER] Conversion error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	log.Printf("[CONVERTER] Conversion successful")
	return c.JSON(scene)
}
