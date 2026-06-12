import {
  adminImportStockMutationCSV,
  adminListStockMutationCreators,
  adminListStockMutations,
  type StockMutationFilters,
  type StockMutationImportResult,
  type StockMutationList,
  type StockMutationStatus,
  type StockMutationType
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 50;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const statusRaw = url.searchParams.get('status') ?? '';
  const status: StockMutationStatus | '' =
    statusRaw === 'draft' || statusRaw === 'executed' ? statusRaw : '';

  const typeRaw = url.searchParams.get('type') ?? '';
  const type: StockMutationType | '' = typeRaw === 'in' || typeRaw === 'out' ? typeRaw : '';

  const fromParam = url.searchParams.get('from') ?? '';
  const toParam = url.searchParams.get('to') ?? '';
  const from = DATE_RE.test(fromParam) ? fromParam : '';
  const to = DATE_RE.test(toParam) ? toParam : '';
  const q = url.searchParams.get('q') ?? '';
  const createdBy = url.searchParams.get('created_by') ?? '';

  const filters: StockMutationFilters = {
    status,
    type,
    from: from || undefined,
    to: to || undefined,
    created_by: createdBy || undefined,
    q: q || undefined,
    limit: PAGE_SIZE,
    offset
  };

  const [list, creators] = await Promise.all([
    adminListStockMutations(token, filters).catch(
      () => ({ items: [], total: 0 } as StockMutationList)
    ),
    adminListStockMutationCreators(token).catch(() => [])
  ]);

  return {
    list,
    creators,
    page: pageNum,
    pageSize: PAGE_SIZE,
    q,
    status,
    type,
    from,
    to,
    createdBy
  };
};

async function doImport(
  request: Request,
  cookies: { get: (k: string) => string | undefined },
  type: StockMutationType
): Promise<{ importResult: StockMutationImportResult } | ReturnType<typeof fail>> {
  const token = cookies.get('admin_token') ?? '';
  if (!token) return fail(401, { importError: 'not signed in' });
  const data = await request.formData();
  const file = data.get('file') as File | null;
  if (!file || file.size === 0) {
    return fail(400, { importError: 'no file' });
  }
  try {
    const result = await adminImportStockMutationCSV(token, type, file);
    return { importResult: result };
  } catch (err) {
    return fail(500, { importError: err instanceof Error ? err.message : 'Import failed' });
  }
}

export const actions: Actions = {
  importIn: ({ request, cookies }) => doImport(request, cookies, 'in'),
  importOut: ({ request, cookies }) => doImport(request, cookies, 'out')
};
