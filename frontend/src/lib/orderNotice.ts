// Safe renderer for order-notice message bodies. Escapes everything first,
// then linkifies `[text](http(s)://…)` markdown links and bare http(s) URLs.
// Only http/https schemes are emitted, so it is XSS-safe even for
// customer-typed bodies (no raw HTML from the body survives escaping).

const LINK_CLASS =
  'text-blue-700 underline underline-offset-2 hover:text-blue-900 break-all';

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function anchor(href: string, text: string): string {
  return `<a href="${href}" target="_blank" rel="noopener noreferrer" class="${LINK_CLASS}">${text}</a>`;
}

export function renderNoticeBody(body: string | null | undefined): string {
  if (!body) return '';
  let html = escapeHtml(body).replace(
    /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
    (_, text: string, url: string) => anchor(url, text)
  );
  // Bare URLs not already wrapped by the markdown pass above. The leading
  // group avoids re-matching a URL that sits inside an emitted href="…" or
  // >…</a> from the previous replacement.
  html = html.replace(
    /(^|[^"=>])(https?:\/\/[^\s<]+)/g,
    (_, pre: string, url: string) => `${pre}${anchor(url, url)}`
  );
  return html;
}
