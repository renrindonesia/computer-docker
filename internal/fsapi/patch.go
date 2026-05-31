package fsapi

import (
	"errors"
	"os"
	"strings"
)

// ErrPatchNoMatch is returned when the `old` block is not found uniquely.
var ErrPatchNoMatch = errors.New("patch context not found")

// ErrPatchAmbiguous is returned when the `old` block matches more than once.
var ErrPatchAmbiguous = errors.New("patch context matches multiple locations")

// Patch applies an apply_patch-style edit: it replaces the first unique
// occurrence of old with new in the file at rel. If old is empty the file is
// created/overwritten with new. Returns ErrPatchNoMatch / ErrPatchAmbiguous on
// failure so the caller can map to a 4xx.
func (s *Service) Patch(rel, oldStr, newStr string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}

	if oldStr == "" {
		// create or full overwrite
		return s.Write(rel, []byte(newStr))
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	content := string(data)

	n := strings.Count(content, oldStr)
	switch n {
	case 0:
		return ErrPatchNoMatch
	case 1:
		updated := strings.Replace(content, oldStr, newStr, 1)
		return os.WriteFile(abs, []byte(updated), 0o644)
	default:
		return ErrPatchAmbiguous
	}
}
