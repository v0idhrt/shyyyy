package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// ============================================================
// File Storage
// ============================================================

type FileStorage struct {
	root string
}

func NewFileStorage(root string) *FileStorage {
	return &FileStorage{root: root}
}

// ============================================================
// Directory paths
// ============================================================

func (s *FileStorage) UserDir(userID string) string {
	return filepath.Join(s.root, userID)
}

func (s *FileStorage) SVGDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "svg")
}

func (s *FileStorage) PNGDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "png")
}

func (s *FileStorage) PDFDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "pdf")
}

func (s *FileStorage) JSONDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "json")
}

func (s *FileStorage) EditedSVGDir(userID string) string {
	return filepath.Join(s.SVGDir(userID), "edited")
}

func (s *FileStorage) EditedJSONDir(userID string) string {
	return filepath.Join(s.JSONDir(userID), "edited")
}

// ============================================================
// File paths
// ============================================================

func (s *FileStorage) SVGPath(userID, filename string) string {
	return filepath.Join(s.SVGDir(userID), filename)
}

func (s *FileStorage) PNGPath(userID, filename string) string {
	return filepath.Join(s.PNGDir(userID), filename)
}

func (s *FileStorage) PDFPath(userID, filename string) string {
	return filepath.Join(s.PDFDir(userID), filename)
}

func (s *FileStorage) JSONPath(userID, filename string) string {
	return filepath.Join(s.JSONDir(userID), filename)
}

func (s *FileStorage) EditedSVGPath(userID, filename string) string {
	return filepath.Join(s.EditedSVGDir(userID), filename)
}

func (s *FileStorage) EditedJSONPath(userID, filename string) string {
	return filepath.Join(s.EditedJSONDir(userID), filename)
}

// ============================================================
// Ensure directories
// ============================================================

func (s *FileStorage) EnsureDir(userID string) error {
	path := s.UserDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir user dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureSVGDir(userID string) error {
	path := s.SVGDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir svg dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsurePNGDir(userID string) error {
	path := s.PNGDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir png dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsurePDFDir(userID string) error {
	path := s.PDFDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir pdf dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureJSONDir(userID string) error {
	path := s.JSONDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir json dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureEditedSVGDir(userID string) error {
	path := s.EditedSVGDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir edited svg dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureEditedJSONDir(userID string) error {
	path := s.EditedJSONDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir edited json dir: %w", err)
	}
	return nil
}

// ============================================================
// Save file
// ============================================================

func (s *FileStorage) SaveFile(userID, target string, data []byte) error {
	if err := s.EnsureDir(userID); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
