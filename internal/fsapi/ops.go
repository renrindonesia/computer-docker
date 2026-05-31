package fsapi

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Move renames/moves src to dst, both jailed.
func (s *Service) Move(srcRel, dstRel string) error {
	src, err := s.resolve(srcRel)
	if err != nil {
		return err
	}
	dst, err := s.resolve(dstRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// Copy copies a single file src to dst, both jailed.
func (s *Service) Copy(srcRel, dstRel string) error {
	src, err := s.resolve(srcRel)
	if err != nil {
		return err
	}
	dst, err := s.resolve(dstRel)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// Chmod changes the mode bits of rel.
func (s *Service) Chmod(rel string, mode os.FileMode) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.Chmod(abs, mode)
}

// SearchHit is one match from Search.
type SearchHit struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	Text string `json:"text,omitempty"`
}

// Search walks rel and returns files whose name matches glob. When content is
// non-empty, only files containing that substring are returned, with the
// matching line numbers. Capped at limit hits.
func (s *Service) Search(rel, glob, content string, limit int) ([]SearchHit, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	var hits []SearchHit
	err = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if len(hits) >= limit {
			return filepath.SkipAll
		}
		if glob != "" {
			ok, _ := filepath.Match(glob, d.Name())
			if !ok {
				return nil
			}
		}
		relPath, _ := filepath.Rel(s.Root, p)
		jailPath := "/" + filepath.ToSlash(relPath)

		if content == "" {
			hits = append(hits, SearchHit{Path: jailPath})
			return nil
		}
		hits = appendContentMatches(hits, p, jailPath, content, limit)
		return nil
	})
	return hits, err
}

func appendContentMatches(hits []SearchHit, absPath, jailPath, needle string, limit int) []SearchHit {
	f, err := os.Open(absPath)
	if err != nil {
		return hits
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	ln := 0
	for sc.Scan() {
		ln++
		if strings.Contains(sc.Text(), needle) {
			hits = append(hits, SearchHit{Path: jailPath, Line: ln, Text: sc.Text()})
			if len(hits) >= limit {
				break
			}
		}
	}
	return hits
}

// AbsFor exposes the jailed absolute path for streaming handlers.
func (s *Service) AbsFor(rel string) (string, error) {
	return s.resolve(rel)
}
