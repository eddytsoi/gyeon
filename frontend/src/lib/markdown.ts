// Storefront-facing markdown → HTML renderer. Hand-rolled instead of
// pulling in a parser dep — we only need the small subset of CommonMark
// our content actually uses (headings, paragraphs, lists, blockquote,
// hr, inline emphasis/links/code).
//
// Block-aware: headings, lists, blockquotes and rules are emitted as
// their own block elements rather than getting trapped inside an outer
// <p>. The previous regex-only renderer relied on a wrapping
// `<p>${html}</p>` at the call site, which produced empty <p></p> +
// stranded text whenever the browser auto-closed the wrapper around a
// nested <h3>. Now the function returns a complete HTML fragment that
// callers should render directly via {@html …} with no outer wrapper.

import { buildResponsiveAttrs } from '$lib/image';

const HEADING_CLASSES = [
  'text-2xl font-bold mt-8 mb-3 text-gray-900', // h1
  'text-xl font-bold mt-8 mb-2 text-gray-900',  // h2
  'text-lg font-bold mt-7 mb-2 text-gray-900',  // h3
  'text-base font-bold mt-6 mb-1 text-gray-900' // h4
];

const PARAGRAPH_CLASS = 'mb-5 leading-relaxed text-gray-700';
const BLOCKQUOTE_CLASS = 'border-l-4 border-gray-200 pl-4 italic text-gray-500 my-4';
const HR_CLASS = 'my-8 border-gray-100';
const LIST_UL_CLASS = 'list-disc ml-5 mb-5';
const LIST_OL_CLASS = 'list-decimal ml-5 mb-5';
const LIST_ITEM_CLASS = 'mb-1';
const CODE_CLASS = 'bg-gray-100 text-gray-800 px-1.5 py-0.5 rounded text-sm font-mono';
const LINK_CLASS = 'text-gray-900 underline underline-offset-2 hover:text-gray-600';

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Raw HTML pass-through: extract HTML tags / comments before escaping so
// admins can mix raw HTML into markdown. Plain `<` in prose (e.g. "if a < b")
// is NOT matched here because the regex requires `[a-zA-Z]` after `<`, so
// stray angle brackets still get escaped normally.
const HTML_TAG_RE = /<!--[\s\S]*?-->|<\/?[a-zA-Z][a-zA-Z0-9-]*(?:\s+[a-zA-Z_:][a-zA-Z0-9_.:-]*(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?)*\s*\/?>/g;

// Block-level HTML tags whose opener/closer on its own line should NOT be
// wrapped in <p>. Anything else (span, a, strong, em, …) stays inline.
// List follows CommonMark spec block-HTML rule 6 plus pre/script/style/
// noscript/textarea from rule 1 and video for media embeds.
const BLOCK_TAG_RE = /^(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|noscript|ol|optgroup|option|p|param|pre|script|search|section|style|summary|table|tbody|td|textarea|tfoot|th|thead|title|tr|track|ul|video)$/i;

const SENTINEL_RE = /\x00HTML(\d+)\x00/g;

function extractHtmlTokens(md: string): { text: string; tokens: string[] } {
  const tokens: string[] = [];
  const text = md.replace(HTML_TAG_RE, (m) => {
    const idx = tokens.length;
    tokens.push(m);
    return `\x00HTML${idx}\x00`;
  });
  return { text, tokens };
}

// True when the trimmed line starts with a block-level HTML tag — in which
// case the whole line is emitted raw rather than getting paragraph-wrapped.
// (Block-level tags like <h2>, <div>, <table> can't legally live inside <p>,
// so the browser would auto-close the wrapper anyway and leave stray empty
// <p></p>s in the DOM.)
function isBlockHtmlLine(trimmed: string, tokens: string[]): boolean {
  const m = /^\x00HTML(\d+)\x00/.exec(trimmed);
  if (!m) return false;
  const tag = tokens[+m[1]];
  const nameMatch = /^<\/?([a-zA-Z][a-zA-Z0-9-]*)/.exec(tag);
  return !!nameMatch && BLOCK_TAG_RE.test(nameMatch[1]);
}

// Inline transformations applied within an already-block-classified
// run (heading text, list-item body, paragraph line). Operates on
// already-escaped input so the regex can't be tricked by user `<`s.
function renderInline(s: string): string {
  return s
    .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`(.+?)`/g, `<code class="${CODE_CLASS}">$1</code>`)
    .replace(/\[(.+?)\]\((.+?)\)/g, `<a href="$2" class="${LINK_CLASS}">$1</a>`);
}

export function renderMarkdown(md: string | undefined | null): string {
  if (!md) return '';
  const { text, tokens } = extractHtmlTokens(md);
  const lines = escapeHtml(text).split('\n');

  const out: string[] = [];
  let pBuf: string[] = [];
  let listType: 'ul' | 'ol' | null = null;
  let listItems: string[] = [];

  // Paragraphs use <br /> for hard line breaks within a single block,
  // so a "title\nbody" pair still groups under one <p>. Lists are
  // properly wrapped so consecutive `- ` lines collapse into one <ul>.

  const flushParagraph = () => {
    if (pBuf.length === 0) return;
    out.push(`<p class="${PARAGRAPH_CLASS}">${pBuf.join('<br />')}</p>`);
    pBuf = [];
  };
  const flushList = () => {
    if (!listType) return;
    const cls = listType === 'ul' ? LIST_UL_CLASS : LIST_OL_CLASS;
    const items = listItems.map((it) => `<li class="${LIST_ITEM_CLASS}">${it}</li>`).join('');
    out.push(`<${listType} class="${cls}">${items}</${listType}>`);
    listType = null;
    listItems = [];
  };

  const headingRe = /^(#{1,4})\s+(.+)$/;
  const quoteRe = /^>\s+(.+)$/;
  const ulRe = /^-\s+(.+)$/;
  const olRe = /^\d+\.\s+(.+)$/;

  for (const raw of lines) {
    const trimmed = raw.trim();

    if (trimmed === '') {
      flushParagraph();
      flushList();
      continue;
    }

    let m: RegExpExecArray | null;

    if ((m = headingRe.exec(trimmed))) {
      flushParagraph();
      flushList();
      const level = m[1].length;
      const cls = HEADING_CLASSES[level - 1];
      out.push(`<h${level} class="${cls}">${renderInline(m[2])}</h${level}>`);
      continue;
    }

    if ((m = quoteRe.exec(trimmed))) {
      flushParagraph();
      flushList();
      out.push(`<blockquote class="${BLOCKQUOTE_CLASS}">${renderInline(m[1])}</blockquote>`);
      continue;
    }

    if (trimmed === '---') {
      flushParagraph();
      flushList();
      out.push(`<hr class="${HR_CLASS}" />`);
      continue;
    }

    if (isBlockHtmlLine(trimmed, tokens)) {
      flushParagraph();
      flushList();
      out.push(raw);
      continue;
    }

    if ((m = ulRe.exec(trimmed))) {
      flushParagraph();
      if (listType !== 'ul') {
        flushList();
        listType = 'ul';
      }
      listItems.push(renderInline(m[1]));
      continue;
    }

    if ((m = olRe.exec(trimmed))) {
      flushParagraph();
      if (listType !== 'ol') {
        flushList();
        listType = 'ol';
      }
      listItems.push(renderInline(m[1]));
      continue;
    }

    // Plain paragraph line. Trim trailing whitespace only — leading
    // whitespace is preserved so the rare bit of indented prose still
    // looks intentional in the rendered output.
    flushList();
    pBuf.push(renderInline(raw.replace(/\s+$/, '')));
  }
  flushParagraph();
  flushList();
  const html = out.join('\n').replace(SENTINEL_RE, (_, i) => tokens[+i] ?? '');
  return rewriteResponsiveImages(html);
}

// CMS-embedded <img> tags pointing at /uploads/ get a srcset + sizes pair
// injected so admins don't need to author responsive markup by hand. Author-
// provided srcset/sizes win and are left untouched.
const CMS_IMG_WIDTHS = [480, 768, 1280];
const CMS_IMG_SIZES = '(min-width: 768px) 720px, 100vw';
const IMG_TAG_RE = /<img\b([^>]*?)>/gi;
const IMG_SRC_RE = /\bsrc\s*=\s*(?:"([^"]+)"|'([^']+)'|([^\s>]+))/i;

function rewriteResponsiveImages(html: string): string {
  return html.replace(IMG_TAG_RE, (full, attrsStr: string) => {
    if (/\bsrcset\s*=/i.test(attrsStr)) return full;
    const srcMatch = attrsStr.match(IMG_SRC_RE);
    if (!srcMatch) return full;
    const origSrc = srcMatch[1] ?? srcMatch[2] ?? srcMatch[3];
    const { src, srcset } = buildResponsiveAttrs(origSrc, CMS_IMG_WIDTHS);
    if (!srcset) return full;
    const newAttrs = attrsStr.replace(
      IMG_SRC_RE,
      `src="${src}"`
    );
    const sizesAttr = /\bsizes\s*=/i.test(attrsStr) ? '' : ` sizes="${CMS_IMG_SIZES}"`;
    return `<img${newAttrs} srcset="${srcset}"${sizesAttr}>`;
  });
}
