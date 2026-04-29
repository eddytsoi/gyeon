import { error, redirect } from '@sveltejs/kit';
import { adminLookup } from '$lib/api/admin';

const PREFIX_TO_ENTITY = {
  PRD: 'products',
  ORD: 'orders',
  PG: 'pages',
  POST: 'posts',
} as const;

type Prefix = keyof typeof PREFIX_TO_ENTITY;

const PREFIX_RE = /^([A-Za-z]+)-(\d+)$/;

/**
 * Resolves an admin URL `[id]` parameter that may be a UUID or a prefix-id
 * like `PRD-8` to the underlying UUID.
 *
 * - UUID input → returned as-is.
 * - Lower/mixed-case prefix → 301 redirect to the canonical uppercase form.
 * - Wrong prefix for this route (e.g. ORD-1 on /admin/products) → 404.
 * - Number with no matching row → 404 (via lookup endpoint).
 */
export async function resolveAdminId(
  token: string,
  expectedPrefix: Prefix,
  paramId: string,
  routeBase: string,
): Promise<string> {
  const m = paramId.match(PREFIX_RE);
  if (!m) return paramId; // assume UUID; downstream API surfaces 404 if invalid

  if (m[1].toUpperCase() !== expectedPrefix) {
    throw error(404, 'Unknown ID');
  }

  const canonical = `${expectedPrefix}-${m[2]}`;
  if (paramId !== canonical) {
    throw redirect(301, `${routeBase}/${canonical}`);
  }

  const { id } = await adminLookup(token, PREFIX_TO_ENTITY[expectedPrefix], m[2]);
  return id;
}
