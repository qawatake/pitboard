package board

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// The viewed file keeps what the person has looked at, for the viewer's
// unread marks: hashes of what lines said (HashKey), one per line, never
// sent to the AI; a line "-<hash>" takes one back.

// ViewedPath is the viewed file.
func (s *Store) ViewedPath() string { return filepath.Join(s.Dir, "viewed") }

// Viewed is the hashes in the viewed file. A file that cannot be read is
// empty, and a line that is not a hash is skipped.
func (s *Store) Viewed() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(s.ViewedPath())
	if err != nil {
		return out
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		switch {
		case isHash(l):
			out[l] = true
		case strings.HasPrefix(l, "-") && isHash(l[1:]):
			delete(out, l[1:])
		}
	}
	return out
}

// RemoveViewed takes hashes back (appends "-<hash>" lines).
func (s *Store) RemoveViewed(hs []string) error {
	minus := make([]string, len(hs))
	for i, h := range hs {
		minus[i] = "-" + h
	}
	return s.AddViewed(minus)
}

// AddViewed appends hashes to the viewed file.
func (s *Store) AddViewed(hs []string) error {
	if len(hs) == 0 {
		return nil
	}
	f, err := os.OpenFile(s.ViewedPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(hs, "\n") + "\n")
	return err
}

func isHash(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func HashKey(k string) string {
	h := sha256.Sum256([]byte(k))
	return hex.EncodeToString(h[:16])
}
