// Package fsapi provides path-jailed filesystem operations.
package fsapi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrEscape is returned when a path resolves outside the root jail.
var ErrEscape = errors.New("path escapes root")

// Service performs filesystem operations confined to Root.
type Service struct {
	Root string
}

// New creates a Service jailed to root. Root is made absolute.
func New(root string) (*Service, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Service{Root: abs}, nil
}

// resolve joins rel onto Root and rejects traversal outside Root.
func (s *Service) resolve(rel string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(rel, "/"))
	abs := filepath.Join(s.Root, clean)
	if abs != s.Root && !strings.HasPrefix(abs, s.Root+string(os.PathSeparator)) {
		return "", ErrEscape
	}
	return abs, nil
}

// Entry describes a directory entry.
type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

// List returns the entries of the directory at rel.
func (s *Service) List(rel string) ([]Entry, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	des, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, toEntry(s.Root, filepath.Join(abs, de.Name()), info))
	}
	return out, nil
}

// Stat returns metadata for rel.
func (s *Service) Stat(rel string) (Entry, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Entry{}, err
	}
	return toEntry(s.Root, abs, info), nil
}

// Read returns the contents of the file at rel.
func (s *Service) Read(rel string) ([]byte, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

// Write creates or overwrites the file at rel, making parent dirs as needed.
func (s *Service) Write(rel string, data []byte) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

// Mkdir creates the directory tree at rel.
func (s *Service) Mkdir(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// Delete removes the file or directory tree at rel.
func (s *Service) Delete(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if abs == s.Root {
		return fmt.Errorf("refusing to delete root")
	}
	return os.RemoveAll(abs)
}

func toEntry(root, abs string, info fs.FileInfo) Entry {
	rel, _ := filepath.Rel(root, abs)
	return Entry{
		Name:    info.Name(),
		Path:    "/" + filepath.ToSlash(rel),
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime(),
	}
}
