package models

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
)

var (
	ErrEmailTaken  = errors.New("models: email address is already in use")
	ErrNotFound    = errors.New("models: resource could not be found")
	ErrLinkExpired = errors.New("models: your link has expired")
)

type FileError struct {
	Issue string
}

func (fe FileError) Error() string {
	return fmt.Sprintf("invalid file: %v", fe.Issue)
}

func checkContentType(r io.ReadSeeker, allowedTypes []string) error {
	testBytes := make([]byte, 512)
	_, err := r.Read(testBytes)
	if err != nil {
		return fmt.Errorf("checking content type: %w", err)
	}
	// Reset file to the starting position after reading from it
	_, err = r.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("checking content type: %w", err)
	}
	contentType := http.DetectContentType(testBytes)
	for _, t := range allowedTypes {
		if contentType == t {
			return nil
		}
	}
	return FileError{
		Issue: fmt.Sprintf("invalid content type: %v", contentType),
	}
}

func checkExtension(filename string, allowedExtension []string) error {
	if hasExtension(filename, allowedExtension) {
		return nil
	}
	return FileError{
		Issue: fmt.Sprintf("extension not supported: %v", filepath.Ext(filename)),
	}
}
