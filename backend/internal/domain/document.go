package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// MaxDocumentBytes caps a single upload. Personal paperwork — contracts,
// scans, invoices — sits far below this; the limit exists so a stray upload
// can't fill the server's disk.
const MaxDocumentBytes int64 = 25 << 20

// DocumentFolder is a folder in the document tree. ParentID nil means the
// folder sits at the root.
type DocumentFolder struct {
	ID        string
	ParentID  *string
	Name      string
	CreatedAt time.Time
}

func (f DocumentFolder) Validate() error {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if len(name) > 120 {
		return fmt.Errorf("%w: name is too long", ErrValidation)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: name cannot contain slashes", ErrValidation)
	}
	return nil
}

// Document is one stored file. Name is what the person sees; StoredName is
// the file on disk, which they never choose.
type Document struct {
	ID          string
	FolderID    *string
	Name        string
	StoredName  string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

func (d Document) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%w: file name is required", ErrValidation)
	}
	if d.SizeBytes <= 0 {
		return fmt.Errorf("%w: file is empty", ErrValidation)
	}
	if d.SizeBytes > MaxDocumentBytes {
		return fmt.Errorf("%w: file is larger than %d MB", ErrValidation, MaxDocumentBytes>>20)
	}
	return nil
}

// SafeFileName strips anything that could make an uploaded name mean
// something to the filesystem — directory parts, separators, control and
// invisible characters — and leaves a readable label. The result is never
// used alone as a path: the stored file is always <uuid>__<safe name>.
func SafeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.TrimSpace(name)

	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == 0:
			// dropped
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	safe := strings.TrimSpace(b.String())
	safe = strings.Trim(safe, ".")
	if safe == "" {
		return "arquivo"
	}
	if len(safe) > 120 {
		ext := filepath.Ext(safe)
		if len(ext) > 12 {
			ext = ""
		}
		safe = safe[:120-len(ext)] + ext
	}
	return safe
}
