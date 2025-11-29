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

func (s *FileStorage) UserDir(userID string) string {
	return filepath.Join(s.root, userID)
}

func (s *FileStorage) SVGPath(userID string) string {
	return filepath.Join(s.UserDir(userID), "plan.svg")
}

func (s *FileStorage) PDFPath(userID string) string {
	return filepath.Join(s.UserDir(userID), "document.pdf")
}

func (s *FileStorage) PNGPath(userID string) string {
	return filepath.Join(s.UserDir(userID), "plan.png")
}

func (s *FileStorage) UploadsDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "uploads")
}

func (s *FileStorage) UploadsPNGPath(userID, base string) string {
	return filepath.Join(s.UploadsDir(userID), base+".png")
}

func (s *FileStorage) UploadsSVGPath(userID, base string) string {
	return filepath.Join(s.UploadsDir(userID), base+".svg")
}

func (s *FileStorage) EditedDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "edited")
}

func (s *FileStorage) EditedPath(userID, filename string) string {
	return filepath.Join(s.EditedDir(userID), filename)
}

func (s *FileStorage) JSONDir(userID string) string {
	return filepath.Join(s.UserDir(userID), "json")
}

func (s *FileStorage) JSONPath(userID, filename string) string {
	return filepath.Join(s.JSONDir(userID), filename)
}

func (s *FileStorage) EditedJSONDir(userID string) string {
	return filepath.Join(s.JSONDir(userID), "edited")
}

func (s *FileStorage) EditedJSONPath(userID, filename string) string {
	return filepath.Join(s.EditedJSONDir(userID), filename)
}

func (s *FileStorage) EnsureDir(userID string) error {
	path := s.UserDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir user dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureUploadsDir(userID string) error {
	path := s.UploadsDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir uploads dir: %w", err)
	}
	return nil
}

func (s *FileStorage) EnsureEditedDir(userID string) error {
	path := s.EditedDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir edited dir: %w", err)
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

func (s *FileStorage) EnsureEditedJSONDir(userID string) error {
	path := s.EditedJSONDir(userID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir edited json dir: %w", err)
	}
	return nil
}

func (s *FileStorage) SaveFile(userID, target string, data []byte) error {
	if err := s.EnsureDir(userID); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
