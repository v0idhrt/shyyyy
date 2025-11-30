package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"api-gateway/internal/auth/models"
	"api-gateway/internal/auth/repository"
	"api-gateway/internal/auth/service"

	"github.com/gofiber/fiber/v3"
)

// ============================================================
// Auth Handler
// ============================================================

type AuthHandler struct {
	repo         *repository.Repository
	sessions     *service.SessionManager
	storage      *service.FileStorage
	converterURL string
}

func NewAuthHandler(repo *repository.Repository, sessions *service.SessionManager, storage *service.FileStorage, converterURL string) *AuthHandler {
	return &AuthHandler{
		repo:         repo,
		sessions:     sessions,
		storage:      storage,
		converterURL: converterURL,
	}
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string      `json:"token"`
	User  userPayload `json:"user"`
}

type userPayload struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	FIO       string `json:"fio"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	BirthDate string `json:"birth_date"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

// Login выдает простой токен по паре login/password.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	log.Printf("[AUTH] Login request")

	if len(c.Body()) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "empty body"})
	}

	var req loginRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}

	if req.Login == "" || req.Password == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "login and password required"})
	}

	user, err := h.repo.GetByCredentials(context.Background(), req.Login, req.Password)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	token := h.sessions.Issue(user.ID)

	return c.JSON(loginResponse{
		Token: token,
		User:  mapUser(user),
	})
}

// GetUser возвращает данные пользователя.
func (h *AuthHandler) GetUser(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	user, err := h.repo.GetByID(context.Background(), targetID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	return c.JSON(mapUser(user))
}

// GetUserInternal возвращает данные пользователя для межсервисного общения (без авторизации)
func (h *AuthHandler) GetUserInternal(c fiber.Ctx) error {
	targetID := c.Params("id")
	if targetID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "user id required"})
	}

	user, err := h.repo.GetByID(context.Background(), targetID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	return c.JSON(mapUser(user))
}

// GetSVG отдаёт svg файл пользователя из папки svg/.
func (h *AuthHandler) GetSVG(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.SVGDir(userID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendFile(path)
}

// GetPDF отдаёт pdf файл пользователя из папки pdf/.
func (h *AuthHandler) GetPDF(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.PDFDir(userID), c.Query("name"), ".pdf")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/pdf")
	return c.SendFile(path)
}

// GetPDFByName - alias для GetPDF (обратная совместимость).
func (h *AuthHandler) GetPDFByName(c fiber.Ctx) error {
	return h.GetPDF(c)
}

// GetPNG отдаёт png файл пользователя из папки png/.
func (h *AuthHandler) GetPNG(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.PNGDir(userID), c.Query("name"), ".png")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "image/png")
	return c.SendFile(path)
}

// GetJSON отдаёт json файл пользователя.
func (h *AuthHandler) GetJSON(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.JSONDir(userID), c.Query("name"), ".json")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/json")
	return c.SendFile(path)
}

// GetEditedJSON отдаёт измененный json.
func (h *AuthHandler) GetEditedJSON(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.EditedJSONDir(userID), c.Query("name"), ".json")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/json")
	return c.SendFile(path)
}

// UploadSVG сохраняет svg в папку svg/.
func (h *AuthHandler) UploadSVG(c fiber.Ctx) error {
	return h.saveFileToDir(c, h.storage.EnsureSVGDir, h.storage.SVGPath, ".svg")
}

// UploadPDF сохраняет pdf в папку pdf/.
func (h *AuthHandler) UploadPDF(c fiber.Ctx) error {
	return h.saveFileToDir(c, h.storage.EnsurePDFDir, h.storage.PDFPath, ".pdf")
}

// UploadPNG сохраняет png в папку png/.
func (h *AuthHandler) UploadPNG(c fiber.Ctx) error {
	return h.saveFileToDir(c, h.storage.EnsurePNGDir, h.storage.PNGPath, ".png", ".jpg", ".jpeg")
}

// UploadJSON сохраняет json файл в папке json/.
func (h *AuthHandler) UploadJSON(c fiber.Ctx) error {
	return h.saveFileToDir(c, h.storage.EnsureJSONDir, h.storage.JSONPath, ".json")
}

// UploadEditedJSON сохраняет измененный json в json/edited/.
func (h *AuthHandler) UploadEditedJSON(c fiber.Ctx) error {
	userID := c.Params("id")
	
	// Читаем raw JSON из body
	jsonData := c.Body()
	if len(jsonData) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "empty body"})
	}
	
	// Валидируем JSON
	var tmp interface{}
	if err := json.Unmarshal(jsonData, &tmp); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	
	// Получаем имя файла из query параметра или генерируем
	filename := c.Query("name")
	if filename == "" {
		// Генерируем имя на основе timestamp
		filename = fmt.Sprintf("edited_%d.json", time.Now().Unix())
	} else if !strings.HasSuffix(filename, ".json") {
		filename += ".json"
	}
	
	// Создаем директорию если нужно
	if err := h.storage.EnsureEditedJSONDir(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare directory"})
	}
	
	// Сохраняем файл
	targetPath := h.storage.EditedJSONPath(userID, filename)
	if err := h.storage.SaveFile(userID, targetPath, jsonData); err != nil {
		log.Printf("[AUTH] save edited json error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}
	
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     targetPath,
		"filename": filename,
	})
}

// UploadEditedSVG сохраняет измененный svg в svg/edited/.
func (h *AuthHandler) UploadEditedSVG(c fiber.Ctx) error {
	return h.saveFileToDir(c, h.storage.EnsureEditedSVGDir, h.storage.EditedSVGPath, ".svg")
}

// GetEditedSVG отдает измененный svg из svg/edited/.
func (h *AuthHandler) GetEditedSVG(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.EditedSVGDir(userID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendFile(path)
}

// GetSVGAsJSON конвертирует пользовательский svg в JSON через Converter.
func (h *AuthHandler) GetSVGAsJSON(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.SVGDir(userID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	body, err := h.convertSVG(path)
	if err != nil {
		log.Printf("[AUTH] convert svg error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	if saved, err := h.saveJSONFile(userID, filepath.Base(path)+".json", body); err != nil {
		log.Printf("[AUTH] save json error: %v", err)
	} else {
		c.Set("X-Saved-JSON", saved)
	}

	c.Set("Content-Type", "application/json")
	return c.Send(body)
}

// GetEditedSVGAsJSON конвертирует edited svg в JSON через Converter.
func (h *AuthHandler) GetEditedSVGAsJSON(c fiber.Ctx) error {
	userID := c.Params("id")

	path, err := h.resolveFilePath(userID, h.storage.EditedSVGDir(userID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	body, err := h.convertSVG(path)
	if err != nil {
		log.Printf("[AUTH] convert edited svg error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	if saved, err := h.saveJSONFile(userID, filepath.Base(path)+".json", body); err != nil {
		log.Printf("[AUTH] save json error: %v", err)
	} else {
		c.Set("X-Saved-JSON", saved)
	}

	c.Set("Content-Type", "application/json")
	return c.Send(body)
}

// UploadPNGAndReturnSVG сохраняет png в png/ и возвращает одноименный svg из svg/, если есть.
func (h *AuthHandler) UploadPNGAndReturnSVG(c fiber.Ctx) error {
	userID := c.Params("id")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	name := fileHeader.Filename
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only png/jpg/jpeg allowed"})
	}
	base := strings.TrimSuffix(name, ext)

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}

	if err := h.storage.EnsurePNGDir(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare png dir"})
	}

	pngPath := h.storage.PNGPath(userID, name)
	if err := h.storage.SaveFile(userID, pngPath, data); err != nil {
		log.Printf("[AUTH] save png error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	svgPath := h.storage.SVGPath(userID, base+".svg")
	if _, err := os.Stat(svgPath); err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "matching svg not found",
			"png":   pngPath,
		})
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendFile(svgPath)
}

// UploadPNGToJSON сохраняет png в png/ и возвращает JSON, полученный из одноименного SVG в svg/ через Converter.
func (h *AuthHandler) UploadPNGToJSON(c fiber.Ctx) error {
	userID := c.Params("id")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	name := fileHeader.Filename
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only png/jpg/jpeg allowed"})
	}
	base := strings.TrimSuffix(name, ext)

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}

	if err := h.storage.EnsurePNGDir(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare png dir"})
	}

	pngPath := h.storage.PNGPath(userID, name)
	if err := h.storage.SaveFile(userID, pngPath, data); err != nil {
		log.Printf("[AUTH] save png error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	svgPath := h.storage.SVGPath(userID, base+".svg")
	if _, err := os.Stat(svgPath); err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "matching svg not found",
			"png":   pngPath,
		})
	}

	sceneJSON, err := h.convertSVG(svgPath)
	if err != nil {
		log.Printf("[AUTH] convert svg error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	if saved, err := h.saveJSONFile(userID, base+".json", sceneJSON); err != nil {
		log.Printf("[AUTH] save json error: %v", err)
	} else {
		c.Set("X-Saved-JSON", saved)
	}

	c.Set("Content-Type", "application/json")
	return c.Send(sceneJSON)
}

// RenderEditedSVG принимает scene JSON, рендерит через Converter и сохраняет в svg/edited/.
func (h *AuthHandler) RenderEditedSVG(c fiber.Ctx) error {
	userID := c.Params("id")

	name := c.Query("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "name required"})
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid name"})
	}

	scene := c.Body()
	if len(scene) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "empty body"})
	}

	svg, err := h.renderScene(scene)
	if err != nil {
		log.Printf("[AUTH] render scene error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	if err := h.storage.EnsureEditedSVGDir(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare edited svg dir"})
	}

	filename := base + ".svg"
	savePath := h.storage.EditedSVGPath(userID, filename)
	if err := h.storage.SaveFile(userID, savePath, svg); err != nil {
		log.Printf("[AUTH] save rendered svg error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     savePath,
		"filename": filename,
	})
}

// ListFiles возвращает наличие файлов пользователя.
func (h *AuthHandler) ListFiles(c fiber.Ctx) error {
	userID := c.Params("id")

	return c.JSON(fiber.Map{
		"svg":         listFilesWithExt(h.storage.SVGDir(userID), ".svg"),
		"png":         listFilesWithExt(h.storage.PNGDir(userID), ".png"),
		"pdf":         listFilesWithExt(h.storage.PDFDir(userID), ".pdf"),
		"json":        listFilesWithExt(h.storage.JSONDir(userID), ".json"),
		"edited_svg":  listFilesWithExt(h.storage.EditedSVGDir(userID), ".svg"),
		"edited_json": listFilesWithExt(h.storage.EditedJSONDir(userID), ".json"),
	})
}

// ============================================================
// Helpers
// ============================================================

func (h *AuthHandler) authorize(c fiber.Ctx) (string, bool) {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	userID, ok := h.sessions.Resolve(token)
	return userID, ok
}

// saveFileToDir сохраняет файл с оригинальным именем в указанную директорию.
func (h *AuthHandler) saveFileToDir(c fiber.Ctx, ensureDirFn func(string) error, pathFn func(string, string) string, allowedExt ...string) error {
	userID := c.Params("id")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	if len(allowedExt) > 0 {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		valid := false
		for _, a := range allowedExt {
			if ext == a {
				valid = true
				break
			}
		}
		if !valid {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid file type"})
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}

	if err := ensureDirFn(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare directory"})
	}

	targetPath := pathFn(userID, fileHeader.Filename)
	if err := h.storage.SaveFile(userID, targetPath, data); err != nil {
		log.Printf("[AUTH] save file error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     targetPath,
		"filename": fileHeader.Filename,
	})
}

func mapUser(u *models.User) userPayload {
	return userPayload{
		ID:        u.ID,
		Login:     u.Login,
		FIO:       u.FIO,
		Email:     u.Email,
		Phone:     u.Phone,
		BirthDate: u.BirthDate,
		Address:   u.Address,
		CreatedAt: u.CreatedAt,
	}
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func listFilesWithExt(dir, ext string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ext) {
			out = append(out, e.Name())
		}
	}
	return out
}

// convertSVG отправляет svg файл в Converter /convert и возвращает тело ответа.
func (h *AuthHandler) convertSVG(svgPath string) ([]byte, error) {
	if h.converterURL == "" {
		return nil, fmt.Errorf("converter url is empty")
	}

	svgData, err := os.ReadFile(svgPath)
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(svgPath))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(svgData); err != nil {
		return nil, err
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, h.converterURL+"/convert", bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("converter status %d", resp.StatusCode)
	}

	return data, nil
}

// renderScene отправляет JSON сцену в Converter /render и возвращает SVG.
func (h *AuthHandler) renderScene(scene []byte) ([]byte, error) {
	if h.converterURL == "" {
		return nil, fmt.Errorf("converter url is empty")
	}

	req, err := http.NewRequest(http.MethodPost, h.converterURL+"/render", bytes.NewReader(scene))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("converter status %d", resp.StatusCode)
	}

	return data, nil
}

// resolveFilePath выбирает файл по имени или, если имя пустое и файлов >1, возвращает 400/404.
func (h *AuthHandler) resolveFilePath(userID, dir, name, ext string) (string, error) {
	alts := listFilesWithExt(dir, ext)

	if name == "" {
		switch len(alts) {
		case 0:
			return "", fiber.NewError(http.StatusNotFound, "file not found")
		case 1:
			return filepath.Join(dir, alts[0]), nil
		default:
			return "", fiber.NewError(http.StatusBadRequest, "multiple files, specify name")
		}
	}

	if filepath.Ext(name) != ext {
		name += ext
	}

	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", fiber.NewError(http.StatusNotFound, "file not found")
	}

	return path, nil
}

func (h *AuthHandler) saveJSONFile(userID, filename string, data []byte) (string, error) {
	if err := h.storage.EnsureJSONDir(userID); err != nil {
		return "", err
	}
	path := h.storage.JSONPath(userID, filename)
	return path, h.storage.SaveFile(userID, path, data)
}
