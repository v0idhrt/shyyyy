package handlers

import (
	"os"

	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Swagger Handlers
// ============================================================

// SwaggerSpec отдаёт OpenAPI YAML.
func SwaggerSpec(c fiber.Ctx) error {
	data, err := os.ReadFile("docs/api-gateway.openapi.yaml")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "spec not found"})
	}
	c.Type("yaml")
	return c.Send(data)
}

// SwaggerUI отдаёт страницу Swagger UI, читающую spec из /docs/openapi.yaml.
func SwaggerUI(c fiber.Ctx) error {
	page := `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>API Gateway Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '/docs/openapi.yaml',
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis],
    });
  };
</script>
</body>
</html>`

	c.Type("html")
	return c.SendString(page)
}
