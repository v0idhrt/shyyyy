package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Proxy Handler
// ============================================================

// ProxyTo прокси запрос к другому сервису
func ProxyTo(targetURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return forwardRequest(c, targetURL)
	}
}

// Forward проксирует запрос по переданному URL (для динамических путей).
func Forward(c fiber.Ctx, targetURL string) error {
	return forwardRequest(c, targetURL)
}

// forwardRequest проксирует любой метод с учетом multipart/raw.
func forwardRequest(c fiber.Ctx, targetURL string) error {
	log.Printf("[PROXY] Request: %s %s", c.Method(), c.Path())
	log.Printf("[PROXY] Content-Type: %s", c.Get("Content-Type"))
	log.Printf("[PROXY] Content-Length: %d", len(c.Body()))
	log.Printf("[PROXY] Forwarding to: %s", targetURL)

	contentType := c.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return sendRaw(c, targetURL, contentType)
	}

	return sendMultipart(c, targetURL)
}

func sendRaw(c fiber.Ctx, targetURL, contentType string) error {
	body := bytes.NewReader(c.Body())
	req, err := http.NewRequest(c.Method(), targetURL, body)
	if err != nil {
		log.Printf("[PROXY] build request error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "proxy failed"})
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth := c.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[PROXY] Error: %v", err)
		return c.Status(502).JSON(fiber.Map{"error": "failed to reach upstream service"})
	}
	defer resp.Body.Close()

	return copyResponse(c, resp)
}

func sendMultipart(c fiber.Ctx, targetURL string) error {
	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("[PROXY] Failed to parse multipart: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "invalid multipart data"})
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, files := range form.File {
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("[PROXY] Failed to open file: %v", err)
				continue
			}

			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, key, fileHeader.Filename))
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

	for key, values := range form.Value {
		for _, value := range values {
			writer.WriteField(key, value)
		}
	}

	writer.Close()

	req, err := http.NewRequest(c.Method(), targetURL, bytes.NewReader(body.Bytes()))
	if err != nil {
		log.Printf("[PROXY] build multipart request error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "proxy failed"})
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if auth := c.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[PROXY] Error: %v", err)
		return c.Status(502).JSON(fiber.Map{"error": "failed to reach upstream service"})
	}
	defer resp.Body.Close()

	return copyResponse(c, resp)
}

func copyResponse(c fiber.Ctx, resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[PROXY] Read response error: %v", err)
		return c.Status(502).JSON(fiber.Map{"error": "invalid upstream response"})
	}

	for key, values := range resp.Header {
		if len(values) > 0 {
			c.Set(key, values[0])
		}
	}

	c.Status(resp.StatusCode)
	return c.Send(data)
}
