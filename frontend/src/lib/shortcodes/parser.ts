import type { Chunk, ShortcodeAttrs } from './types';
import { isKnownShortcode } from './types';

// Matches an opening shortcode like `[name attr1="val" attr2="val"]`.
// Group 1: name. Group 2: attribute string. Self-closing or paired (closer
// looked up separately so the regex stays linear).
const OPEN_RE = /\[([a-z][a-z0-9_-]*)((?:\s+[a-z][a-z0-9_-]*="[^"]*")*)\s*\]/gi;

// Attribute pairs within the captured attribute string.
const ATTR_RE = /([a-z][a-z0-9_-]*)="([^"]*)"/gi;

function parseAttrs(s: string): ShortcodeAttrs {
  const attrs: ShortcodeAttrs = {};
  ATTR_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = ATTR_RE.exec(s)) !== null) {
    attrs[m[1].toLowerCase()] = m[2];
  }
  return attrs;
}

// Locate `[/name]` starting at `from`. Returns the index of `[` or -1.
function findCloser(src: string, name: string, from: number): number {
  const needle = `[/${name}]`;
  // Case-insensitive search done by lowercasing slice on demand.
  const haystack = src.toLowerCase();
  return haystack.indexOf(needle.toLowerCase(), from);
}

// True if the `[` at `idx` is preceded by an odd number of backslashes — in
// which case it's escaped and should be treated as literal text.
function isEscaped(src: string, idx: number): boolean {
  let count = 0;
  let i = idx - 1;
  while (i >= 0 && src[i] === '\\') {
    count++;
    i--;
  }
  return count % 2 === 1;
}

export function parseShortcodes(md: string | undefined | null): Chunk[] {
  if (!md) return [];

  const chunks: Chunk[] = [];
  let cursor = 0;

  OPEN_RE.lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = OPEN_RE.exec(md)) !== null) {
    const openStart = match.index;
    const openEnd = openStart + match[0].length;
    const name = match[1].toLowerCase();
    const attrsRaw = match[2] ?? '';

    if (isEscaped(md, openStart)) {
      // Skip — the escape is processed when we flush md chunks below.
      continue;
    }

    if (!isKnownShortcode(name)) {
      // Unknown name: leave the source verbatim (WordPress behavior).
      continue;
    }

    const attrs = parseAttrs(attrsRaw);

    // Look for a matching closer to capture body. If none, treat as
    // self-closing. The closer must come after the opener.
    const closerStart = findCloser(md, name, openEnd);
    let body = '';
    let consumedEnd = openEnd;
    if (closerStart !== -1 && !isEscaped(md, closerStart)) {
      body = md.slice(openEnd, closerStart);
      consumedEnd = closerStart + `[/${name}]`.length;
      // Advance the regex past the closer so we don't re-scan body content.
      OPEN_RE.lastIndex = consumedEnd;
    }

    // Flush any markdown text between the previous cursor and this opener.
    if (openStart > cursor) {
      const text = unescapeBrackets(md.slice(cursor, openStart));
      if (text) chunks.push({ type: 'md', text });
    }

    chunks.push({
      type: 'shortcode',
      name,
      attrs,
      body,
      raw: md.slice(openStart, consumedEnd)
    });

    cursor = consumedEnd;
  }

  if (cursor < md.length) {
    const text = unescapeBrackets(md.slice(cursor));
    if (text) chunks.push({ type: 'md', text });
  }

  return chunks;
}

// Turn `\[` into `[` and `\\` into `\` so escaped shortcode source renders
// as literal text. Unknown sequences are left alone.
function unescapeBrackets(s: string): string {
  return s.replace(/\\([\\[])/g, '$1');
}
