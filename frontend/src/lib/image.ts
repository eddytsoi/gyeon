// Responsive-image URL helper. Rewrites same-host /uploads/ image URLs into
// a srcset pointing at the on-demand resize endpoint exposed by the Go
// backend at /uploads/r/{width}/{filename}. The whitelist below MUST stay in
// sync with backend/internal/media/resize.go::allowedWidths — anything not in
// that set returns 400 from the backend.

const ALLOWED_WIDTHS = new Set([160, 320, 480, 640, 768, 960, 1280, 1600, 1920]);

// Default widths cover the storefront's grid + PDP needs. Surface-specific
// callers (hero, thumbnails) pass their own narrower or wider set.
export const DEFAULT_WIDTHS = [320, 480, 640, 960, 1280];

export interface ResponsiveAttrs {
  src: string;
  srcset: string;
}

// buildResponsiveAttrs returns a srcset for /uploads/ paths and rewrites src
// to a sensible fallback width. Non-/uploads/ URLs (external CDNs, data URLs,
// already-resized /uploads/r/) pass through unchanged with an empty srcset.
export function buildResponsiveAttrs(
  src: string,
  widths: number[] = DEFAULT_WIDTHS
): ResponsiveAttrs {
  if (!src) return { src: '', srcset: '' };

  // Capture optional scheme+host prefix and the /uploads/<rest> tail. Strip
  // any query/hash so the resize URLs stay clean.
  const match = src.match(/^(.*?)\/uploads\/([^?#]+)(\?.*)?$/);
  if (!match) return { src, srcset: '' };

  const origin = match[1];
  const filename = match[2];

  // Already pointing at the resize endpoint — don't double-process.
  if (filename.startsWith('r/')) return { src, srcset: '' };

  // Backend rejects slashes/dotdot in filename, so the rewrite is a no-op for
  // anything that wouldn't be served anyway.
  if (filename.includes('/') || filename.includes('..')) return { src, srcset: '' };

  const valid = widths.filter((w) => ALLOWED_WIDTHS.has(w));
  if (valid.length === 0) return { src, srcset: '' };

  const sorted = [...valid].sort((a, b) => a - b);
  const srcset = sorted
    .map((w) => `${origin}/uploads/r/${w}/${filename} ${w}w`)
    .join(', ');

  // Browsers without srcset support fall back to a middle-of-the-set width.
  const fallback = sorted[Math.floor(sorted.length / 2)];
  return { src: `${origin}/uploads/r/${fallback}/${filename}`, srcset };
}
