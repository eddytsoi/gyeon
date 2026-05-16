// Splits a form's CF7-style markup template into ordered chunks for the
// storefront renderer. Each `[type* name ...]` tag becomes a `field` /
// `submit` chunk; everything else passes through as raw HTML.
//
// All attribute resolution (placeholder, default, options, etc.) is done by
// the backend parser and lives on `form.fields[i]`; this splitter only needs
// to identify which field each tag refers to.

export type FormMarkupChunk =
  | { type: 'html'; html: string }
  | { type: 'submit' }
  | { type: 'field'; name: string };

const KNOWN_TYPES = new Set([
  'text',
  'email',
  'tel',
  'textarea',
  'select',
  'checkbox',
  'radio',
  'date',
  'file',
  'submit',
  'hidden'
]);

const FIELD_NAME_RE = /^[A-Za-z][A-Za-z0-9_-]*$/;

export function splitFormMarkup(src: string | undefined | null): FormMarkupChunk[] {
  if (!src) return [];
  const out: FormMarkupChunk[] = [];
  let cursor = 0;
  let i = 0;
  while (i < src.length) {
    if (src[i] !== '[' || (i > 0 && src[i - 1] === '\\')) {
      i++;
      continue;
    }
    const end = src.indexOf(']', i + 1);
    if (end === -1) break;
    const body = src.slice(i + 1, end).trim();
    const tag = parseTag(body);
    if (!tag) {
      i = end + 1;
      continue;
    }
    if (i > cursor) {
      out.push({ type: 'html', html: unescapeBrackets(src.slice(cursor, i)) });
    }
    out.push(tag);
    cursor = end + 1;
    i = cursor;
  }
  if (cursor < src.length) {
    out.push({ type: 'html', html: unescapeBrackets(src.slice(cursor)) });
  }
  return out;
}

function parseTag(body: string): FormMarkupChunk | null {
  if (!body) return null;
  // Pull the first bare (non-quoted) word as the type.
  const m = /^([A-Za-z][A-Za-z0-9_-]*\*?)(\s+([\s\S]*))?$/.exec(body);
  if (!m) return null;
  let type = m[1].toLowerCase();
  if (type.endsWith('*')) type = type.slice(0, -1);
  if (!KNOWN_TYPES.has(type)) return null;
  if (type === 'submit') return { type: 'submit' };
  const rest = (m[3] ?? '').trimStart();
  // Field name is the next bare token. Skip leading quoted strings — not
  // expected here per CF7 grammar, but the backend already rejects them so
  // we only need to handle the well-formed case.
  const nameMatch = /^([A-Za-z][A-Za-z0-9_-]*)/.exec(rest);
  if (!nameMatch) return null;
  const name = nameMatch[1];
  if (!FIELD_NAME_RE.test(name)) return null;
  return { type: 'field', name };
}

function unescapeBrackets(s: string): string {
  return s.replace(/\\([\\[])/g, '$1');
}
