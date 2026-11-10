package storage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"familyflix/internal/db"
)

func SaveTempUpload(file multipart.File, header *multipart.FileHeader, targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	targetPath := filepath.Join(targetDir, fmt.Sprintf("upload-%s%s", strings.ReplaceAll(header.Filename, ext, ""), ext))
	out, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return targetPath, nil
}

func ValidateMovieFile(filePath string) error {
	if !db.SupportedFile(filePath) {
		return fmt.Errorf("unsupported file type: %s", filepath.Ext(filePath))
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("file is empty")
	}
	return nil
}
