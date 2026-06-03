// Stateless "share cart" codec. The shareable cart URL carries the cart's
// contents (variant id + quantity per line) encoded directly in the query
// string — there is no server-side share record. The recipient's
// /cart/shared page decodes this and re-adds each line to their own session
// cart (merge semantics, courtesy of the backend's ON CONFLICT upsert).
//
// We deliberately encode only variant_id + quantity, NEVER the owner's
// cart.id (which is effectively a bearer token to mutate that cart). Re-adding
// goes through the normal add-to-cart path, so stock limits and per-role
// purchase rules still apply on the recipient side.

export type SharedItem = { variantId: string; quantity: number };

// base64url so the value stays within [A-Za-z0-9_-] and URLSearchParams adds
// no %-escaping — keeps the shared link clean. Payload is a compact tuple
// array [[variantId, qty], …]. Browser-only (btoa); call from onMount.
export function encodeSharedCart(items: SharedItem[]): string {
  const tuples = items
    .filter((it) => it.variantId && it.quantity > 0)
    .map((it) => [it.variantId, it.quantity] as [string, number]);
  // JSON of UUIDs + ints is pure ASCII, so btoa is safe (no Unicode escaping).
  return btoa(JSON.stringify(tuples))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

// Full shareable URL for a set of lines, pointing at the /cart/shared import
// route. Browser-only (window + btoa); call from a click handler / onMount.
export function buildShareUrl(items: SharedItem[]): string {
  return `${window.location.origin}/cart/shared?c=${encodeSharedCart(items)}`;
}

// Share a URL via the native share sheet (mobile), falling back to copying it
// to the clipboard, then to prompt() as a last resort. Returns which path was
// taken so the caller can show the right inline confirmation.
export async function shareOrCopyUrl(
  url: string,
  nativeTitle: string,
  promptLabel: string
): Promise<'shared' | 'copied' | 'prompted'> {
  if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
    try {
      await navigator.share({ title: nativeTitle, url });
      return 'shared';
    } catch {
      // user dismissed the sheet, or share failed — fall through to clipboard
    }
  }
  try {
    await navigator.clipboard.writeText(url);
    return 'copied';
  } catch {
    // clipboard blocked (insecure context / denied permission) — last resort
    window.prompt(promptLabel, url);
    return 'prompted';
  }
}

// Tolerant decode: never throws. Garbage or tampered input yields the valid
// subset (or []), so the import page can still redirect cleanly instead of
// hanging on a broken link.
export function decodeSharedCart(code: string): SharedItem[] {
  if (!code) return [];
  try {
    let b64 = code.replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4) b64 += '='; // restore stripped padding
    const parsed = JSON.parse(atob(b64));
    if (!Array.isArray(parsed)) return [];
    const out: SharedItem[] = [];
    for (const entry of parsed) {
      if (!Array.isArray(entry) || entry.length < 2) continue;
      const [variantId, quantity] = entry;
      if (typeof variantId !== 'string' || !variantId) continue;
      const q = Number(quantity);
      if (!Number.isInteger(q) || q <= 0) continue;
      out.push({ variantId, quantity: q });
    }
    return out;
  } catch {
    return [];
  }
}
