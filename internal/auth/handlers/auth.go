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

// GetSVG отдаёт svg файл пользователя.
func (h *AuthHandler) GetSVG(c fiber.Ctx) error {
	return h.getFileNamed(c, ".svg", "image/svg+xml")
}

// GetPDF отдаёт pdf файл пользователя.
func (h *AuthHandler) GetPDF(c fiber.Ctx) error {
	return h.getFile(c, h.storage.PDFPath, "application/pdf")
}

// GetPNG отдаёт png файл пользователя.
func (h *AuthHandler) GetPNG(c fiber.Ctx) error {
	return h.getFileNamed(c, ".png", "image/png")
}

// GetJSON отдаёт json файл пользователя.
func (h *AuthHandler) GetJSON(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	path, err := h.resolveFilePath(targetID, h.storage.JSONDir(targetID), c.Query("name"), ".json")
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/json")
	return c.SendFile(path)
}

// UploadSVG сохраняет svg в файловой системе.
func (h *AuthHandler) UploadSVG(c fiber.Ctx) error {
	return h.saveFileWithOriginal(c, ".svg")
}

// UploadPDF сохраняет pdf в файловой системе.
func (h *AuthHandler) UploadPDF(c fiber.Ctx) error {
	return h.saveFile(c, h.storage.PDFPath)
}

// UploadPNG сохраняет png в файловой системе.
func (h *AuthHandler) UploadPNG(c fiber.Ctx) error {
	return h.saveFileWithOriginal(c, ".png")
}

// UploadJSON сохраняет json файл в папке json/.
func (h *AuthHandler) UploadJSON(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}
	if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != ".json" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only json allowed"})
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

	if err := h.storage.EnsureJSONDir(targetID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare json dir"})
	}

	path := h.storage.JSONPath(targetID, fileHeader.Filename)
	if err := h.storage.SaveFile(targetID, path, data); err != nil {
		log.Printf("[AUTH] save json error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     path,
		"filename": fileHeader.Filename,
	})
}

// UploadEditedSVG сохраняет измененный svg с тем же именем в /edited/.
func (h *AuthHandler) UploadEditedSVG(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != ".svg" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only svg allowed"})
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

	if err := h.storage.EnsureEditedDir(targetID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare edited dir"})
	}

	path := h.storage.EditedPath(targetID, fileHeader.Filename)
	if err := h.storage.SaveFile(targetID, path, data); err != nil {
		log.Printf("[AUTH] save edited error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     path,
		"filename": fileHeader.Filename,
	})
}

// GetEditedSVG отдает измененный svg из /edited/.
func (h *AuthHandler) GetEditedSVG(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	name := c.Query("name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "name required"})
	}
	if filepath.Ext(name) != ".svg" {
		name += ".svg"
	}

	path := filepath.Join(h.storage.EditedDir(targetID), name)
	if _, err := os.Stat(path); err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "file not found"})
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendFile(path)
}

// GetSVGAsJSON конвертирует пользовательский svg в JSON через Converter.
func (h *AuthHandler) GetSVGAsJSON(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	path, err := h.resolveFilePath(targetID, h.storage.UserDir(targetID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	body, err := h.convertSVG(path)
	if err != nil {
		log.Printf("[AUTH] convert svg error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	c.Set("Content-Type", "application/json")
	return c.Send(body)
}

// GetEditedSVGAsJSON конвертирует edited svg в JSON через Converter.
func (h *AuthHandler) GetEditedSVGAsJSON(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	path, err := h.resolveFilePath(targetID, h.storage.EditedDir(targetID), c.Query("name"), ".svg")
	if err != nil {
		return err
	}

	body, err := h.convertSVG(path)
	if err != nil {
		log.Printf("[AUTH] convert edited svg error: %v", err)
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": "converter failed"})
	}

	c.Set("Content-Type", "application/json")
	return c.Send(body)
}

// UploadPNGAndReturnSVG сохраняет png в uploads и возвращает одноименный svg, если есть.
func (h *AuthHandler) UploadPNGAndReturnSVG(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	name := fileHeader.Filename
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only png allowed"})
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

	if err := h.storage.EnsureUploadsDir(targetID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare uploads"})
	}

	pngPath := h.storage.UploadsPNGPath(targetID, base)
	if err := h.storage.SaveFile(targetID, pngPath, data); err != nil {
		log.Printf("[AUTH] save uploads png error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	svgPath := h.storage.UploadsSVGPath(targetID, base)
	if _, err := os.Stat(svgPath); err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "matching svg not found",
			"png":   pngPath,
		})
	}

	c.Set("Content-Type", "image/svg+xml")
	return c.SendFile(svgPath)
}

// UploadPNGToJSON сохраняет png и возвращает JSON, полученный из одноименного SVG через Converter.
func (h *AuthHandler) UploadPNGToJSON(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	name := fileHeader.Filename
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "only png allowed"})
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

	if err := h.storage.EnsureUploadsDir(targetID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to prepare uploads"})
	}

	pngPath := h.storage.UploadsPNGPath(targetID, base)
	if err := h.storage.SaveFile(targetID, pngPath, data); err != nil {
		log.Printf("[AUTH] save uploads png error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	svgPath := h.storage.UploadsSVGPath(targetID, base)
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

	c.Set("Content-Type", "application/json")
	return c.Send(sceneJSON)
}

// ListFiles возвращает наличие файлов пользователя.
func (h *AuthHandler) ListFiles(c fiber.Ctx) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	userDir := h.storage.UserDir(targetID)

	return c.JSON(fiber.Map{
		"svg":          fileExists(h.storage.SVGPath(targetID)),
		"pdf":          fileExists(h.storage.PDFPath(targetID)),
		"png":          fileExists(h.storage.PNGPath(targetID)),
		"json":         listFilesWithExt(h.storage.JSONDir(targetID), ".json"),
		"original_svg": listFilesWithExt(userDir, ".svg"),
		"original_png": listFilesWithExt(userDir, ".png"),
		"edited_svg":   listFilesWithExt(h.storage.EditedDir(targetID), ".svg"),
		"uploads": fiber.Map{
			"svg": listFilesWithExt(h.storage.UploadsDir(targetID), ".svg"),
			"png": listFilesWithExt(h.storage.UploadsDir(targetID), ".png"),
		},
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

func (h *AuthHandler) getFile(c fiber.Ctx, pathFn func(string) string, contentType string) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	path := pathFn(targetID)
	if _, err := os.Stat(path); err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "file not found"})
	}

	c.Set("Content-Type", contentType)
	return c.SendFile(path)
}

// getFileNamed ищет файл по name query, иначе если один файл — отдает его, при нескольких — требует name.
func (h *AuthHandler) getFileNamed(c fiber.Ctx, ext, contentType string) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	path, err := h.resolveFilePath(targetID, h.storage.UserDir(targetID), c.Query("name"), ext)
	if err != nil {
		return err
	}
	c.Set("Content-Type", contentType)
	return c.SendFile(path)
}

func (h *AuthHandler) saveFile(c fiber.Ctx, pathFn func(string) string) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
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

	targetPath := pathFn(targetID)
	if err := h.storage.SaveFile(targetID, targetPath, data); err != nil {
		log.Printf("[AUTH] save file error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{"path": targetPath})
}

// saveFileWithOriginal сохраняет файл под исходным именем.
func (h *AuthHandler) saveFileWithOriginal(c fiber.Ctx, allowedExt string) error {
	userID, ok := h.authorize(c)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	targetID := c.Params("id")
	if targetID == "" || targetID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	if allowedExt != "" {
		if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != allowedExt {
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

	originalPath := filepath.Join(h.storage.UserDir(targetID), fileHeader.Filename)
	if err := h.storage.SaveFile(targetID, originalPath, data); err != nil {
		log.Printf("[AUTH] save original error: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save file"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"path":     originalPath,
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
