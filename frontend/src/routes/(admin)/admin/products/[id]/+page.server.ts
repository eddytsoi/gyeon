import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
  adminGetProduct, adminGetCategories, adminGetVariants, adminGetImages, adminGetMedia,
  adminCreateProduct, adminUpdateProduct,
  adminCreateVariant, adminUpdateVariant, adminDeleteVariant, adminAdjustStock,
  adminAddImage, adminUpdateImage, adminDeleteImage
} from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const isNew = params.id === 'new';

  const [product, categories] = await Promise.all([
    isNew ? Promise.resolve(null) : adminGetProduct(token, params.id).catch(() => null),
    adminGetCategories(token).catch(() => [])
  ]);

  const [variants, images, mediaFiles] = isNew ? [[], [], []] : await Promise.all([
    adminGetVariants(token, params.id).catch(() => []),
    adminGetImages(token, params.id).catch(() => []),
    adminGetMedia(token).catch(() => [])
  ]);

  return { product, categories, variants, images, mediaFiles, isNew };
};

export const actions: Actions = {
  saveProduct: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const body = {
      category_id: form.get('category_id')?.toString() || undefined,
      slug: form.get('slug')?.toString() ?? '',
      name: form.get('name')?.toString() ?? '',
      description: form.get('description')?.toString() || undefined,
      is_active: form.get('is_active') === 'true'
    };

    try {
      if (params.id === 'new') {
        const product = await adminCreateProduct(token, { ...body, is_active: true });
        throw redirect(303, `/admin/products/${product.id}`);
      } else {
        await adminUpdateProduct(token, params.id, body);
        return { success: true };
      }
    } catch (e) {
      if (e instanceof Response) throw e; // rethrow redirects
      return fail(400, { error: 'Failed to save product' });
    }
  },

  addVariant: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    try {
      const variant = await adminCreateVariant(token, params.id, {
        sku: form.get('sku')?.toString() ?? '',
        price: parseFloat(form.get('price')?.toString() ?? '0'),
        compare_at_price: form.get('compare_at_price')?.toString()
          ? parseFloat(form.get('compare_at_price')!.toString())
          : undefined,
        stock_qty: parseInt(form.get('stock_qty')?.toString() ?? '0', 10)
      });
      const imageMediaFileId = form.get('image_media_file_id')?.toString();
      if (imageMediaFileId) {
        await adminAddImage(token, params.id, {
          variant_id: variant.id,
          media_file_id: imageMediaFileId,
          sort_order: 0,
          is_primary: false
        });
      }
    } catch {
      return fail(400, { error: 'Failed to add variant' });
    }
    return { success: true };
  },

  updateVariant: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    try {
      await adminUpdateVariant(token, params.id, variantID, {
        sku: form.get('sku')?.toString() ?? '',
        price: parseFloat(form.get('price')?.toString() ?? '0'),
        compare_at_price: form.get('compare_at_price')?.toString()
          ? parseFloat(form.get('compare_at_price')!.toString())
          : undefined,
        stock_qty: parseInt(form.get('stock_qty')?.toString() ?? '0', 10),
        is_active: form.get('is_active') === 'true'
      });
      const oldImageId = form.get('old_image_id')?.toString();
      const imageMediaFileId = form.get('image_media_file_id')?.toString();
      const removeImage = form.get('remove_image') === 'true';
      if (oldImageId && (removeImage || imageMediaFileId)) {
        await adminDeleteImage(token, params.id, oldImageId);
      }
      if (!removeImage && imageMediaFileId) {
        await adminAddImage(token, params.id, {
          variant_id: variantID,
          media_file_id: imageMediaFileId,
          sort_order: 0,
          is_primary: false
        });
      }
    } catch {
      return fail(400, { error: 'Failed to update variant' });
    }
    return { success: true };
  },

  deleteVariant: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    try {
      await adminDeleteVariant(token, params.id, variantID);
    } catch {
      return fail(400, { error: 'Failed to delete variant' });
    }
    return { success: true };
  },

  adjustStock: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    const delta = parseInt(form.get('delta')?.toString() ?? '0', 10);
    try {
      await adminAdjustStock(token, params.id, variantID, delta);
    } catch {
      return fail(400, { error: 'Failed to adjust stock' });
    }
    return { success: true };
  },

  addImage: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    try {
      await adminAddImage(token, params.id, {
        url: form.get('url')?.toString() ?? '',
        alt_text: form.get('alt_text')?.toString() || undefined,
        sort_order: parseInt(form.get('sort_order')?.toString() ?? '0', 10),
        is_primary: form.get('is_primary') === 'true'
      });
    } catch {
      return fail(400, { error: 'Failed to add image' });
    }
    return { success: true };
  },

  setPrimary: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const imageID = form.get('image_id')?.toString() ?? '';
    const sortOrder = parseInt(form.get('sort_order')?.toString() ?? '0', 10);
    try {
      await adminUpdateImage(token, params.id, imageID, {
        is_primary: true,
        sort_order: sortOrder
      });
    } catch {
      return fail(400, { error: 'Failed to update image' });
    }
    return { success: true };
  },

  deleteImage: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const imageID = form.get('image_id')?.toString() ?? '';
    try {
      await adminDeleteImage(token, params.id, imageID);
    } catch {
      return fail(400, { error: 'Failed to delete image' });
    }
    return { success: true };
  }
};
