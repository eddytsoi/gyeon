package forms

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// uploadsRoot is the on-disk directory that holds attachments for every
// form submission. Files are namespaced by submission UUID:
//
//	./uploads/forms/<submission_id>/<unixNano>_<sanitized-basename>.<ext>
//
// CASCADE on form_submissions handles DB cleanup; the service layer
// recursively removes the directory in DeleteSubmission.
const uploadsRoot = "./uploads/forms"

// UploadedFile is the lightweight bundle the handler passes from the
// multipart parse to the service layer. We keep the raw multipart header
// so the service can stream the body straight to disk without buffering
// large files in memory.
type UploadedFile struct {
	FieldName string
	Header    *multipart.FileHeader
}

// storedFile captures what was written to disk so the service can
// persist a row and clean up on transaction rollback.
type storedFile struct {
	FieldName    string
	OriginalName string
	StoredPath   string // absolute-ish (uploadsRoot/<sid>/<name>) for unlink on rollback
	StoredName   string // basename only — what goes into form_submission_files.stored_filename
	MimeType     string
	Size         int64
}

// saveSubmissionFile copies one multipart upload into the on-disk submission
// folder, returning a storedFile that the caller persists into the DB. The
// caller is responsible for unlinking on rollback (storedFile.StoredPath).
func saveSubmissionFile(submissionID string, u UploadedFile) (*storedFile, error) {
	dir := filepath.Join(uploadsRoot, submissionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir submission dir: %w", err)
	}

	src, err := u.Header.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload %q: %w", u.Header.Filename, err)
	}
	defer src.Close()

	name := timestampedName(u.Header.Filename)
	dst := filepath.Join(dir, name)
	out, err := os.Create(dst)
	if err != nil {
		return nil, fmt.Errorf("create %q: %w", dst, err)
	}
	n, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return nil, fmt.Errorf("write %q: %w", dst, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return nil, fmt.Errorf("close %q: %w", dst, closeErr)
	}

	mime := u.Header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	return &storedFile{
		FieldName:    u.FieldName,
		OriginalName: u.Header.Filename,
		StoredPath:   dst,
		StoredName:   name,
		MimeType:     mime,
		Size:         n,
	}, nil
}

// removeSubmissionDir deletes the on-disk folder for a submission. Used by
// DeleteSubmission after the DB cascade fires, and as a rollback hook when
// the multipart write fails mid-transaction.
func removeSubmissionDir(submissionID string) error {
	if submissionID == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(uploadsRoot, submissionID))
}

// timestampedName produces a disk-safe basename from an original upload
// filename: lowercased, alphanumerics + `-_.` only, prefixed with the
// current Unix-nano timestamp so two uploads of `receipt.pdf` in the same
// submission folder don't collide.
func timestampedName(original string) string {
	base := filepath.Base(original)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	cleanStem := sanitizeBasename(stem)
	cleanExt := sanitizeBasename(strings.TrimPrefix(ext, "."))

	if cleanStem == "" {
		cleanStem = "file"
	}
	out := fmt.Sprintf("%d_%s", time.Now().UnixNano(), cleanStem)
	if cleanExt != "" {
		out += "." + cleanExt
	}
	return out
}

// sanitizeBasename keeps lowercase alphanumerics and `-_`. Other bytes
// (spaces, slashes, unicode marks) are dropped. Truncated at 60 chars to
// keep on-disk paths reasonable even with long original names.
func sanitizeBasename(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteRune('-')
		}
	}
	out := b.String()
	if len(out) > 60 {
		out = out[:60]
	}
	out = strings.Trim(out, "-")
	return out
}

// extensionOf returns the lowercase extension of a filename, without the
// leading dot. Used by the file-field validator to check against the
// `filetypes:` allow-list.
func extensionOf(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return strings.TrimPrefix(ext, ".")
}
