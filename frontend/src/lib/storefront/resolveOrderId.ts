import { error, redirect } from '@sveltejs/kit';
import { lookupMyOrder } from '$lib/api';

const PREFIX_RE = /^([A-Za-z]+)-(\d+)$/;

/**
 * Resolves a customer order URL `[id]` parameter that may be a UUID or a
 * prefix-id like `ORD-8` to the underlying UUID.
 *
 * - UUID input → returned as-is.
 * - Lower/mixed-case `ord-8` → 301 redirect to canonical `ORD-8`.
 * - Wrong prefix (e.g. `PRD-1`) → 404.
 * - Number not owned by this customer or not found → 404 (lookup is customer-scoped).
 */
export async function resolveCustomerOrderId(token: string, paramId: string): Promise<string> {
  const m = paramId.match(PREFIX_RE);
  if (!m) return paramId;

  if (m[1].toUpperCase() !== 'ORD') {
    throw error(404, 'Unknown ID');
  }

  const canonical = `ORD-${m[2]}`;
  if (paramId !== canonical) {
    throw redirect(301, `/account/orders/${canonical}`);
  }

  const { id } = await lookupMyOrder(token, m[2]).catch((e: unknown) => {
    if (e instanceof Error && /API 404/.test(e.message)) throw error(404, 'Order not found');
    throw e;
  });
  return id;
}
