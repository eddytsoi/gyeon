import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
  adminGetProduct, adminGetCategories, adminGetVariants, adminGetImages, adminGetMedia,
  adminCreateProduct, adminUpdateProduct,
  adminCreateVariant, adminUpdateVariant, adminDeleteVariant, adminAdjustStock,
  adminAddImage, adminUpdateImage, adminDeleteImage
} from '$lib/api/admin';
import { resolveAdminId } from '$lib/admin/resolveId';

const resolve = (token: string, id: string) =>
  id === 'new' ? Promise.resolve(id) : resolveAdminId(token, 'PRD', id, '/admin/products');

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const isNew = params.id === 'new';
  const id = await resolve(token, params.id);

  const [product, categories] = await Promise.all([
    isNew ? Promise.resolve(null) : adminGetProduct(token, id).catch(() => null),
    adminGetCategories(token).catch(() => [])
  ]);

  const [variants, images, mediaFiles] = isNew
    ? [[], [], await adminGetMedia(token).catch(() => [])]
    : await Promise.all([
      adminGetVariants(token, id).catch(() => []),
      adminGetImages(token, id).catch(() => []),
      adminGetMedia(token).catch(() => [])
    ]);

  return { product, categories, variants, images, mediaFiles, isNew };
};

export const actions: Actions = {
  saveProduct: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const body = {
      category_id: form.get('category_id')?.toString() || undefined,
      slug: form.get('slug')?.toString() ?? '',
      name: form.get('name')?.toString() ?? '',
      description: form.get('description')?.toString() || undefined,
      status: form.get('status')?.toString() ?? 'active'
    };

    let newProductId: string | undefined;
    try {
      if (id === 'new') {
        const product = await adminCreateProduct(token, body);
        newProductId = product.id;
      } else {
        await adminUpdateProduct(token, id, body);
      }
    } catch {
      return fail(400, { error: 'Failed to save product' });
    }

    if (newProductId) {
      // Create pending variants
      const pendingVariantsRaw = form.get('pending_variants')?.toString() ?? '[]';
      let pendingVariants: Array<{
        sku: string; price: number; compare_at_price?: number;
        stock_qty: number; image_media_file_id?: string;
      }> = [];
      try { pendingVariants = JSON.parse(pendingVariantsRaw); } catch { /* ignore */ }

      for (const pv of pendingVariants) {
        try {
          const variant = await adminCreateVariant(token, newProductId, {
            sku: pv.sku, price: pv.price,
            compare_at_price: pv.compare_at_price, stock_qty: pv.stock_qty ?? 0
          });
          if (pv.image_media_file_id) {
            await adminAddImage(token, newProductId, {
              variant_id: variant.id, media_file_id: pv.image_media_file_id,
              sort_order: 0, is_primary: false
            });
          }
        } catch { /* non-fatal */ }
      }

      // Add pending product images
      const pendingImagesRaw = form.get('pending_images')?.toString() ?? '[]';
      let pendingImages: Array<{
        media_file_id: string; is_primary: boolean; alt_text?: string; sort_order: number;
      }> = [];
      try { pendingImages = JSON.parse(pendingImagesRaw); } catch { /* ignore */ }

      let sortOrder = 0;
      for (const pi of pendingImages) {
        try {
          await adminAddImage(token, newProductId, {
            media_file_id: pi.media_file_id, is_primary: pi.is_primary,
            alt_text: pi.alt_text, sort_order: sortOrder++
          });
        } catch { /* non-fatal */ }
      }

      throw redirect(303, `/admin/products/${newProductId}`);
    }
    return { success: true };
  },

  addVariant: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    try {
      const variant = await adminCreateVariant(token, id, {
        sku: form.get('sku')?.toString() ?? '',
        price: parseFloat(form.get('price')?.toString() ?? '0'),
        compare_at_price: form.get('compare_at_price')?.toString()
          ? parseFloat(form.get('compare_at_price')!.toString())
          : undefined,
        stock_qty: parseInt(form.get('stock_qty')?.toString() ?? '0', 10)
      });
      const imageMediaFileId = form.get('image_media_file_id')?.toString();
      if (imageMediaFileId) {
        await adminAddImage(token, id, {
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
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    try {
      await adminUpdateVariant(token, id, variantID, {
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
        await adminDeleteImage(token, id, oldImageId);
      }
      if (!removeImage && imageMediaFileId) {
        await adminAddImage(token, id, {
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
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    try {
      await adminDeleteVariant(token, id, variantID);
    } catch {
      return fail(400, { error: 'Failed to delete variant' });
    }
    return { success: true };
  },

  adjustStock: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const variantID = form.get('variant_id')?.toString() ?? '';
    const delta = parseInt(form.get('delta')?.toString() ?? '0', 10);
    try {
      await adminAdjustStock(token, id, variantID, delta);
    } catch {
      return fail(400, { error: 'Failed to adjust stock' });
    }
    return { success: true };
  },

  addImage: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const mediaFileId = form.get('media_file_id')?.toString() || undefined;
    if (!mediaFileId) return fail(400, { error: 'No image selected' });
    try {
      await adminAddImage(token, id, {
        media_file_id: mediaFileId,
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
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const imageID = form.get('image_id')?.toString() ?? '';
    const sortOrder = parseInt(form.get('sort_order')?.toString() ?? '0', 10);
    try {
      await adminUpdateImage(token, id, imageID, {
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
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const imageID = form.get('image_id')?.toString() ?? '';
    try {
      await adminDeleteImage(token, id, imageID);
    } catch {
      return fail(400, { error: 'Failed to delete image' });
    }
    return { success: true };
  },

  reorderImages: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const ids = (form.get('image_ids')?.toString() ?? '').split(',').filter(Boolean);
    try {
      for (let i = 0; i < ids.length; i++) {
        await adminUpdateImage(token, id, ids[i], { sort_order: i });
      }
    } catch {
      return fail(400, { error: 'Failed to reorder images' });
    }
    return { success: true };
  }
};
