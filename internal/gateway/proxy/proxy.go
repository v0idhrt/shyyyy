package proxy

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/textproto"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
)

// ============================================================
// Proxy Handler
// ============================================================

// ProxyTo прокси запрос к другому сервису
func ProxyTo(targetURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Printf("[PROXY] Request: %s %s", c.Method(), c.Path())
		log.Printf("[PROXY] Content-Type: %s", c.Get("Content-Type"))
		log.Printf("[PROXY] Content-Length: %d", len(c.Body()))
		log.Printf("[PROXY] Forwarding to: %s", targetURL)

		// Парсим multipart
		form, err := c.MultipartForm()
		if err != nil {
			log.Printf("[PROXY] Failed to parse multipart: %v", err)
			return c.Status(400).JSON(fiber.Map{"error": "invalid multipart data"})
		}

		// Пересоздаем multipart для отправки
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Копируем файлы
		for key, files := range form.File {
			for _, fileHeader := range files {
				file, err := fileHeader.Open()
				if err != nil {
					log.Printf("[PROXY] Failed to open file: %v", err)
					continue
				}

				// Создаем multipart field с правильными заголовками
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", `form-data; name="`+key+`"; filename="`+fileHeader.Filename+`"`)
				h.Set("Content-Type", fileHeader.Header.Get("Content-Type"))

				part, err := writer.CreatePart(h)
				if err != nil {
					file.Close()
					log.Printf("[PROXY] Failed to create part: %v", err)
					continue
				}

				io.Copy(part, file)
				file.Close()
			}
		}

		// Копируем обычные поля
		for key, values := range form.Value {
			for _, value := range values {
				writer.WriteField(key, value)
			}
		}

		writer.Close()

		// Отправляем запрос
		resp, err := client.Post(targetURL, client.Config{
			Header: map[string]string{
				"Content-Type": writer.FormDataContentType(),
			},
			Body: body.Bytes(),
		})

		if err != nil {
			log.Printf("[PROXY] Error: %v", err)
			return c.Status(502).JSON(fiber.Map{"error": "failed to reach upstream service"})
		}

		// Возвращаем ответ
		c.Status(resp.StatusCode())
		return c.Send(resp.Body())
	}
}
