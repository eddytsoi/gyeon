import { adminGetProducts, adminGetCategories, adminGetVariants } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const [products, categories] = await Promise.all([
    adminGetProducts(token).catch(() => []).then(r => r ?? []),
    adminGetCategories(token).catch(() => []).then(r => r ?? [])
  ]);

  const enriched = await Promise.all(
    products.map(async (p) => ({
      product: p,
      variants: await adminGetVariants(token, p.id).catch(() => []).then(r => r ?? [])
    }))
  );

  return { products: enriched, categories };
};

export const actions: Actions = {
  create: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const body = {
      category_id: form.get('category_id')?.toString() || undefined,
      slug: form.get('slug')?.toString() ?? '',
      name: form.get('name')?.toString() ?? '',
      description: form.get('description')?.toString() || undefined,
      status: 'active'
    };

    try {
      const { adminCreateProduct } = await import('$lib/api/admin');
      await adminCreateProduct(token, body);
    } catch (e) {
      return fail(400, { error: 'Failed to create product' });
    }
    return { success: true };
  },

  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      const { adminDeleteProduct } = await import('$lib/api/admin');
      await adminDeleteProduct(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete product' });
    }
    redirect(303, '/admin/products');
  },

  toggle: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const id = form.get('id')?.toString() ?? '';
    const currentStatus = form.get('status')?.toString() ?? 'inactive';

    try {
      const { adminUpdateProduct } = await import('$lib/api/admin');
      await adminUpdateProduct(token, id, {
        slug: form.get('slug')?.toString() ?? '',
        name: form.get('name')?.toString() ?? '',
        status: currentStatus === 'active' ? 'inactive' : 'active'
      });
    } catch {
      return fail(400, { error: 'Failed to update product' });
    }
    return { success: true };
  }
};
