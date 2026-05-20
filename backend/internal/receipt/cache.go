package receipt

import (
	"errors"
	"os"
	"path/filepath"
)

// receiptCacheDir is the on-disk directory for cached PDFs. Matches the
// /uploads convention (see backend/internal/media/handler.go) so the dir
// rides the same bind-mounted volume in production (docker-compose.prod.yml
// already mounts `uploads:/app/uploads`).
const receiptCacheDir = "./uploads/receipts"

// errInvalidOrderID guards against any caller passing a value that would
// break out of receiptCacheDir via path traversal. Callers should be fine
// in practice — order IDs come straight from the URL and ultimately from
// the database — but the cache is a thin wrapper that gets used from
// multiple sites (download handler, queue worker, invalidation paths)
// and the check is cheap.
var errInvalidOrderID = errors.New("receipt cache: invalid order id")

type Cache struct {
	dir string
}

func NewCache() *Cache { return &Cache{dir: receiptCacheDir} }

// safeOrderID reports whether s is shaped like an order ID we generate
// (UUID or ULID-ish — letters, digits, dash, underscore). Anything else
// is rejected so it can never escape the cache dir via "../".
func safeOrderID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (c *Cache) path(orderID, locale string) (string, error) {
	if !safeOrderID(orderID) {
		return "", errInvalidOrderID
	}
	return filepath.Join(c.dir, orderID+"_"+resolveLocale(locale)+".pdf"), nil
}

// Exists reports whether a cached receipt for (orderID, locale) is on disk.
// Returns false on any error so callers treat it as a cache miss.
func (c *Cache) Exists(orderID, locale string) bool {
	p, err := c.path(orderID, locale)
	if err != nil {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Get reads the cached PDF. Returns os.ErrNotExist (wrapped by os.ReadFile)
// when there is no cache for this order+locale — callers should fall back
// to generating fresh.
func (c *Cache) Get(orderID, locale string) ([]byte, error) {
	p, err := c.path(orderID, locale)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Put writes the PDF atomically: tempfile in the same directory, then
// rename. A crashed mid-write leaves the original (or no file) intact —
// no half-written .pdf can ever be served.
func (c *Cache) Put(orderID, locale string, pdf []byte) error {
	p, err := c.path(orderID, locale)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.dir, "receipt-*.pdf.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(pdf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// DeleteForOrder removes every locale variant of the cached receipt for
// the given order. A missing file is not an error — DeleteForOrder is
// meant to be idempotent so callers can invoke it on every order delete
// / refund without worrying about prior state.
func (c *Cache) DeleteForOrder(orderID string) error {
	if !safeOrderID(orderID) {
		return errInvalidOrderID
	}
	matches, err := filepath.Glob(filepath.Join(c.dir, orderID+"_*.pdf"))
	if err != nil {
		return err
	}
	for _, m := range matches {
		_ = os.Remove(m)
	}
	return nil
}
