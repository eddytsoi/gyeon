/*
 * Variant name helpers.
 *
 * A variant's `name` is stored as `key:value` pairs joined by ` / `, e.g.
 * "容量:500ml" or "尺寸:L / 顏色:紅". When displaying a cart line, checkout
 * summary or order email we want the bare value portion as a suffix on the
 * product name — "Q²M InteriorDetailer 500ml", not "Q²M InteriorDetailer
 * 容量:500ml".
 */

export function variantSuffix(name: string | null | undefined): string {
  if (!name) return '';
  return name
    .split(' / ')
    .map((p) => {
      const i = p.indexOf(':');
      return i >= 0 ? p.slice(i + 1).trim() : p.trim();
    })
    .filter(Boolean)
    .join(' / ');
}

export function productDisplayName(
  productName: string,
  variantName: string | null | undefined,
  kind?: string | null
): string {
  if (kind === 'bundle') return productName;
  const v = variantSuffix(variantName);
  return v ? `${productName} ${v}` : productName;
}
