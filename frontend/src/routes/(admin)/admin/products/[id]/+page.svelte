<script lang="ts">
  import { enhance } from '$app/forms';
  import { invalidateAll } from '$app/navigation';
  import { adminUploadMedia, adminGetVariants, adminGetVariantStockHistory, adminListProductStockHistory, type VariantHistoryRow, type StockMovementRow } from '$lib/api/admin';
  import StockMovementTable from '$lib/components/admin/StockMovementTable.svelte';
  import type { PageData } from './$types';
  import type { BundleItem, Variant } from '$lib/types';
  import { showResult, notify } from '$lib/stores/notifications.svelte';
  import { spotlight } from '$lib/actions/spotlight';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import SingleMediaPicker from '$lib/components/admin/SingleMediaPicker.svelte';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import {
    isVideo,
    isStreamingVideo,
    detectStreamingVideoFromURL,
    checkMediaSize,
    type MediaSizeRejection
  } from '$lib/media';
  import * as m from '$lib/paraglide/messages';
  import { sortable } from '$lib/actions/sortable';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import ShortcodeToolbar from '$lib/components/admin/ShortcodeToolbar.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';

  // Admin product page renders product images at three scales:
  // - Tiny chips in variant rows (32px square)
  // - Gallery cards (~160-200px tile)
  // - Variant edit preview (square crop, max ~220px)
  const TINY_THUMB_WIDTHS = [160];
  const TINY_THUMB_SIZES = '32px';
  const GALLERY_WIDTHS = [160, 320, 480];
  const GALLERY_SIZES = '(min-width: 1024px) 200px, 33vw';
  const VARIANT_PREVIEW_WIDTHS = [320, 480];
  const VARIANT_PREVIEW_SIZES = '220px';

  let { data }: { data: PageData } = $props();

  function slugify(s: string) {
    return s.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
  }

  // Product form state
  let name = $state(data.product?.name ?? '');
  let slug = $state(data.product?.slug ?? '');
  let kind = $state(data.product?.kind ?? 'simple');
  let autoSlug = $state(!data.product);
  let saving = $state(false);

  // Markdown fields backed by $state so the shortcode toolbar can insert
  // at the cursor. (Was previously plain textareas with default values.)
  let description = $state(data.product?.description ?? '');
  let howToUse = $state(data.product?.how_to_use ?? '');

  // Hero video + banner / media slots. Imported from WooCommerce as ACF
  // meta (video / banner_1 / banner_2 / media_1..4) and editable here.
  // Storefront renders the video iframe between specs grid and tabs;
  // banner_1 / banner_2 sit alongside the content / how-to-use tabs;
  // media_1..4 form a 4-cell strip above BundleComposer.
  let videoId = $state(data.product?.video_id ?? '');
  let banner1MediaId = $state<string | null>(data.product?.banner_1_media_id ?? null);
  let banner2MediaId = $state<string | null>(data.product?.banner_2_media_id ?? null);
  let media1MediaId = $state<string | null>(data.product?.media_1_media_id ?? null);
  let media2MediaId = $state<string | null>(data.product?.media_2_media_id ?? null);
  let media3MediaId = $state<string | null>(data.product?.media_3_media_id ?? null);
  let media4MediaId = $state<string | null>(data.product?.media_4_media_id ?? null);
  // Track files uploaded from a picker so the library tab in the same
  // session sees them without needing a page reload.
  let uploadedMedia = $state<import('$lib/api/admin').MediaFile[]>([]);
  let descriptionTextarea = $state<HTMLTextAreaElement | null>(null);
  let howToUseTextarea = $state<HTMLTextAreaElement | null>(null);
  let descriptionPreview = $state(false);
  let howToUsePreview = $state(false);

  // Category state — the multi-select "Categories" is the source of truth for
  // every category the product belongs to (stored in product_category_links).
  // "Primary Category" is a single FK on `products.category_id` and must be one
  // of the selected categories (or empty). Server dedups the union.
  let primaryCategoryID = $state(data.product?.category_id ?? '');
  let categoryIDs = $state<string[]>(data.product?.category_ids ?? []);
  const categoryOptions = $derived(
    data.categories.map((c) => ({ value: c.id, label: c.name }))
  );
  const primaryCategoryChoices = $derived(
    data.categories.filter((c) => categoryIDs.includes(c.id))
  );
  // If the user removes the current primary from the Categories list, drop
  // the primary so the saved value stays consistent with what's selected.
  $effect(() => {
    if (primaryCategoryID && !categoryIDs.includes(primaryCategoryID)) {
      primaryCategoryID = '';
    }
  });

  // Compatible-surface checkbox state. Keys mirror the storefront tab icons; the
  // server stores them as a TEXT[]. Order is the render order on the storefront.
  const SURFACE_KEYS = ['paint', 'glass', 'wheels', 'leather', 'trim', 'fabric'] as const;
  type SurfaceKey = typeof SURFACE_KEYS[number];
  const surfaceLabels: Record<SurfaceKey, () => string> = {
    paint:   m.product_detail_surface_paint,
    glass:   m.product_detail_surface_glass,
    wheels:  m.product_detail_surface_wheels,
    leather: m.product_detail_surface_leather,
    trim:    m.product_detail_surface_trim,
    fabric:  m.product_detail_surface_fabric
  };
  let selectedSurfaces = $state<Set<SurfaceKey>>(
    new Set((data.product?.compatible_surfaces ?? []).filter((s): s is SurfaceKey => (SURFACE_KEYS as readonly string[]).includes(s)))
  );
  function toggleSurface(key: SurfaceKey) {
    const next = new Set(selectedSurfaces);
    if (next.has(key)) next.delete(key); else next.add(key);
    selectedSurfaces = next;
  }

  // bundle → simple destructive-change confirmation flow
  const originalKind = data.product?.kind ?? 'simple';
  const isBundleToSimplePending = $derived(
    !data.isNew && originalKind === 'bundle' && kind === 'simple'
  );
  let confirmKindChange = $state(false);
  let kindChangeConfirmed = $state(false);
  let savingVariant = $state(false);
  let updatingVariant = $state(false);
  let adjustingStock = $state(false);
  let attachingImage = $state(false);
  let deletingImage = $state(false);

  $effect(() => {
    if (autoSlug) slug = slugify(name);
  });

  // Variant modal state
  let showAddVariant = $state(false);
  let editingVariant = $state<typeof data.variants[0] | null>(null);
  let showStockModal = $state<typeof data.variants[0] | null>(null);
  let historyVariant = $state<typeof data.variants[0] | null>(null);
  let historyRows = $state<VariantHistoryRow[]>([]);
  let historyLoading = $state(false);

  // All-variant stock history for this product (Phase 5 of 進出記錄 module).
  let productHistoryOpen = $state(false);
  let productHistoryRows = $state<StockMovementRow[]>([]);
  let productHistoryLoading = $state(false);
  let productHistoryLoaded = $state(false);

  async function toggleProductHistory() {
    productHistoryOpen = !productHistoryOpen;
    if (productHistoryOpen && !productHistoryLoaded && data.product?.id) {
      productHistoryLoading = true;
      try {
        const list = await adminListProductStockHistory(data.token ?? '', data.product.id, { limit: 100 });
        productHistoryRows = list.items;
        productHistoryLoaded = true;
      } catch (e) {
        notify.error('Failed to load history', e instanceof Error ? e.message : '');
      } finally {
        productHistoryLoading = false;
      }
    }
  }

  async function openHistory(v: typeof data.variants[0]) {
    historyVariant = v;
    historyRows = [];
    historyLoading = true;
    try {
      historyRows = await adminGetVariantStockHistory(data.token ?? '', v.product_id, v.id);
    } catch (e) {
      notify.error('Failed to load history', e instanceof Error ? e.message : '');
    } finally {
      historyLoading = false;
    }
  }

  function fmtReason(r: string): string {
    switch (r) {
      case 'order.checkout': return 'Order checkout';
      case 'admin.adjust': return 'Stock adjustment';
      case 'admin.variant_update': return 'Variant edit';
      case 'wc.import': return 'WooCommerce import';
      default: return r;
    }
  }

  // Usable media — uploaded images, videos (mp4/webm), plus all links
  // (links are tested at render time via onload/onerror).
  function isUsableMedia(f: { mime_type: string; url: string }) {
    return (
      f.mime_type.startsWith('image/') ||
      f.mime_type.startsWith('video/') ||
      f.mime_type === 'link'
    );
  }

  // Variant media picker state (allows both image and video media)
  const imageMedia = $derived((data.mediaFiles ?? []).filter(isUsableMedia));
  // Banner / media slot pickers can also upload directly. The uploaded
  // entries get prepended so the library tab reflects them without a reload.
  const bannerLibrary = $derived([...uploadedMedia, ...imageMedia]);
  let addVariantImageId = $state<string | null>(null);
  let editVariantImageId = $state<string | null>(null);
  let editVariantOldImageId = $state<string | null>(null);
  let editVariantRemoveImage = $state(false);
  const editVariantPreviewUrl = $derived(
    editVariantRemoveImage ? null : (editVariantImageId
      ? (imageMedia.find(m => m.id === editVariantImageId)?.webp_url ?? imageMedia.find(m => m.id === editVariantImageId)?.url ?? null)
      : (editingVariant?.image_url ?? null))
  );

  // Pending state for new products
  type PendingVariant = {
    _localId: string;
    sku: string; name?: string;
    price: number; compare_at_price?: number; stock_qty: number;
    weight_grams?: number;
    length_mm?: number; width_mm?: number; height_mm?: number;
    image_media_file_id?: string; image_preview_url?: string;
  };
  type PendingImage = {
    _localId: string;
    media_file_id: string; preview_url: string; is_primary: boolean; alt_text?: string;
  };
  let pendingVariants = $state<PendingVariant[]>([]);
  let pendingImages = $state<PendingImage[]>([]);

  const pendingVariantsJson = $derived(
    JSON.stringify(pendingVariants.map(({ _localId, image_preview_url, ...rest }) => rest))
  );
  const pendingImagesJson = $derived(
    JSON.stringify(pendingImages.map(({ _localId, preview_url, ...rest }) => rest))
  );

  // Confirm delete image modal state
  let confirmDeleteImageId = $state<string | null>(null);

  // ── Bundle state ─────────────────────────────────────────────────────────────
  type EditableBundleItem = BundleItem & { _localId: string };

  let bundleItems = $state<EditableBundleItem[]>(
    (data.bundleItems ?? []).map(bi => ({ ...bi, _localId: bi.id }))
  );
  // Bundle pricing — single source of truth for both new-product and edit modes.
  // Seeded from the auto-created variant when editing an existing bundle.
  const initialBundleVariant = data.product?.kind === 'bundle' ? (data.variants?.[0] ?? null) : null;
  let pendingBundlePrice          = $state<number | ''>(initialBundleVariant?.price ?? '');
  let pendingBundleCompareAtPrice = $state<number | ''>(initialBundleVariant?.compare_at_price ?? '');
  let pendingBundleWeightGrams    = $state<number | ''>(initialBundleVariant?.weight_grams ?? '');
  let pendingBundleLengthMM       = $state<number | ''>(initialBundleVariant?.length_mm ?? '');
  let pendingBundleWidthMM        = $state<number | ''>(initialBundleVariant?.width_mm  ?? '');
  let pendingBundleHeightMM       = $state<number | ''>(initialBundleVariant?.height_mm ?? '');

  const bundleItemsJson = $derived(
    JSON.stringify(bundleItems.map(({ _localId, ...rest }) => ({
      component_variant_id: rest.component_variant_id,
      quantity: rest.quantity,
      sort_order: rest.sort_order,
      display_name_override: rest.display_name_override || undefined
    })))
  );

  // Bundle component picker
  let pickerProductId = $state('');
  let pickerVariants = $state<Variant[]>([]);
  let pickerVariantId = $state('');
  let pickerQty = $state(1);
  let pickerDisplayName = $state('');
  let loadingPickerVariants = $state(false);

  async function loadPickerVariants(productId: string) {
    pickerVariantId = '';
    pickerVariants = [];
    if (!data.token || !productId) return;
    loadingPickerVariants = true;
    try {
      pickerVariants = await adminGetVariants(data.token, productId);
    } catch {
      pickerVariants = [];
    } finally {
      loadingPickerVariants = false;
    }
  }

  function addBundleComponent() {
    if (!pickerVariantId) return;
    const variant = pickerVariants.find(v => v.id === pickerVariantId);
    const product = (data.allProducts ?? []).find(p => p.id === pickerProductId);
    if (!variant || !product) return;
    if (bundleItems.some(bi => bi.component_variant_id === pickerVariantId)) {
      notify.warning(m.admin_product_edit_bundle_duplicate_title(), m.admin_product_edit_bundle_duplicate_body({ sku: variant.sku }));
      return;
    }
    bundleItems = [...bundleItems, {
      _localId: crypto.randomUUID(),
      id: '',
      bundle_product_id: data.product?.id ?? '',
      component_variant_id: pickerVariantId,
      quantity: pickerQty > 0 ? pickerQty : 1,
      sort_order: bundleItems.length,
      display_name_override: pickerDisplayName.trim() || undefined,
      component_product_name: product.name,
      component_sku: variant.sku,
      component_variant_name: variant.name,
      component_price: variant.price,
      component_stock_qty: variant.stock_qty
    }];
    pickerProductId = '';
    pickerVariantId = '';
    pickerVariants = [];
    pickerQty = 1;
    pickerDisplayName = '';
  }

  // Simple products for bundle component picker (exclude bundles to prevent nesting)
  const simpleProducts = $derived((data.allProducts ?? []).filter(p => p.kind !== 'bundle' && p.id !== data.product?.id));

  // Add Image modal state
  let showAddImage = $state(false);
  let addImageSelectedId = $state<string | null>(null);
  let addImageTab = $state<'upload' | 'library'>('upload');

  // Upload tab state
  type UploadFile = {
    id: string;
    file: File;
    preview: string;
    status: 'uploading' | 'success' | 'error';
    progress: number;
    error?: string;
  };
  let uploadFiles = $state<UploadFile[]>([]);
  let uploadDragOver = $state(false);
  const ACCEPTED_MEDIA = /^(image\/(jpeg|png|webp|gif)|video\/(mp4|webm))$/;

  function resetAddImageModal() {
    showAddImage = false;
    addImageSelectedId = null;
    addImageTab = 'upload';
    for (const f of uploadFiles) URL.revokeObjectURL(f.preview);
    uploadFiles = [];
    uploadDragOver = false;
  }

  function openFilePicker() {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.accept = 'image/jpeg,image/png,image/webp,image/gif,video/mp4,video/webm';
    input.onchange = () => handleFiles(Array.from(input.files ?? []));
    input.click();
  }

  function onUploadDragEnter(e: DragEvent) { e.preventDefault(); uploadDragOver = true; }
  function onUploadDragLeave(e: DragEvent) { e.preventDefault(); uploadDragOver = false; }
  function onUploadDragOver(e: DragEvent)  { e.preventDefault(); uploadDragOver = true; }
  function onUploadDrop(e: DragEvent) {
    e.preventDefault();
    uploadDragOver = false;
    handleFiles(Array.from(e.dataTransfer?.files ?? []));
  }

  async function handleFiles(files: File[]) {
    if (!data.token) return;
    const token = data.token;
    const valid = files.filter((f) => ACCEPTED_MEDIA.test(f.type));
    const rejected = files.length - valid.length;
    if (rejected > 0) {
      notify.warning(
        rejected === 1
          ? m.admin_product_edit_add_media_rejected_title_one({ count: rejected })
          : m.admin_product_edit_add_media_rejected_title_many({ count: rejected }),
        m.admin_product_edit_add_media_rejected_body()
      );
    }
    const sizeOk: File[] = [];
    const sizeRejected: MediaSizeRejection[] = [];
    for (const f of valid) {
      const r = checkMediaSize(f, data.uploadLimits);
      if (r) sizeRejected.push(r);
      else sizeOk.push(f);
    }
    if (sizeRejected.length > 0) {
      const title = sizeRejected.length === 1
        ? m.admin_media_size_rejected_title_one({ count: sizeRejected.length })
        : m.admin_media_size_rejected_title_many({ count: sizeRejected.length });
      const hasImage = sizeRejected.some((r) => r.kind === 'image');
      const hasVideo = sizeRejected.some((r) => r.kind === 'video');
      const body = hasImage && hasVideo
        ? m.admin_media_size_rejected_both({ imageMB: data.uploadLimits.imageMB, videoMB: data.uploadLimits.videoMB })
        : hasVideo
          ? m.admin_media_size_rejected_video({ limitMB: data.uploadLimits.videoMB })
          : m.admin_media_size_rejected_image({ limitMB: data.uploadLimits.imageMB });
      notify.warning(title, body);
    }
    const newItems: UploadFile[] = sizeOk.map((file) => ({
      id: crypto.randomUUID(),
      file,
      preview: URL.createObjectURL(file),
      status: 'uploading',
      progress: 0
    }));
    uploadFiles = [...uploadFiles, ...newItems];
    let attached = 0;
    for (const item of newItems) {
      try {
        const media = await adminUploadMedia(token, item.file, (pct) => {
          const entry = uploadFiles.find((u) => u.id === item.id);
          if (entry) entry.progress = pct;
        });
        const done = uploadFiles.find((u) => u.id === item.id);
        if (done) done.progress = 100;
        if (data.isNew) {
          pendingImages = [...pendingImages, {
            _localId: item.id,
            media_file_id: media.id,
            preview_url: media.webp_url ?? media.url,
            is_primary: pendingImages.length === 0,
            alt_text: undefined
          }];
        } else {
          const fd = new FormData();
          fd.set('media_file_id', media.id);
          fd.set('sort_order', '0');
          fd.set('is_primary', 'false');
          const res = await fetch('?/addImage', { method: 'POST', body: fd });
          if (!res.ok) throw new Error(m.admin_product_edit_add_media_attach_failed({ status: res.status }));
          attached++;
        }
        const ok = uploadFiles.find((u) => u.id === item.id);
        if (ok) ok.status = 'success';
      } catch (e) {
        const fail = uploadFiles.find((u) => u.id === item.id);
        if (fail) {
          fail.status = 'error';
          fail.error = e instanceof Error ? e.message : m.admin_product_edit_add_media_upload_failed();
        }
      }
    }
    if (!data.isNew && attached > 0) await invalidateAll();
  }

  // Image drag-and-drop reorder state
  function sortedImages(imgs: typeof data.images) {
    return [...imgs].sort((a, b) => {
      if (a.is_primary) return -1;
      if (b.is_primary) return 1;
      return a.sort_order - b.sort_order;
    });
  }
  let images = $state(sortedImages(data.images));
  let reorderSaving = $state(false);
  let reorderError = $state<string | null>(null);
  const anyUploading = $derived(uploadFiles.some(f => f.status === 'uploading'));

  $effect(() => { images = sortedImages(data.images); });
  $effect(() => { bundleItems = (data.bundleItems ?? []).map(bi => ({ ...bi, _localId: bi.id })); });
  $effect(() => {
    if (data.product?.kind === 'bundle') {
      const bv = data.variants?.[0];
      pendingBundlePrice          = bv?.price ?? '';
      pendingBundleCompareAtPrice = bv?.compare_at_price ?? '';
      pendingBundleWeightGrams    = bv?.weight_grams ?? '';
      pendingBundleLengthMM       = bv?.length_mm ?? '';
      pendingBundleWidthMM        = bv?.width_mm  ?? '';
      pendingBundleHeightMM       = bv?.height_mm ?? '';
    }
  });

  function onReorderVariants(orderedIds: string[]) {
    if (data.isNew) {
      // Pending-variants (new product) — reorder in place, no server call.
      const byLocalId = new Map(pendingVariants.map(pv => [pv._localId, pv]));
      const reordered = orderedIds
        .map(lid => byLocalId.get(lid))
        .filter((pv): pv is typeof pendingVariants[number] => !!pv);
      if (reordered.length === pendingVariants.length) {
        pendingVariants = reordered;
      }
      return;
    }
    void persistReorderVariants(orderedIds);
  }

  async function persistReorderVariants(ids: string[]) {
    const snapshot = [...data.variants];
    // Optimistic reorder.
    const byId = new Map(data.variants.map(v => [v.id, v]));
    const reordered = ids.map(id => byId.get(id)).filter((v): v is typeof data.variants[number] => !!v);
    if (reordered.length !== data.variants.length) return;
    data.variants = reordered;
    try {
      const fd = new FormData();
      fd.set('variant_ids', ids.join(','));
      const res = await fetch('?/reorderVariants', { method: 'POST', body: fd });
      if (!res.ok) throw new Error();
    } catch {
      data.variants = snapshot;
    }
  }

  function onReorderBundleItems(orderedIds: string[]) {
    const byLocalId = new Map(bundleItems.map(bi => [bi._localId, bi]));
    const reordered = orderedIds
      .map(lid => byLocalId.get(lid))
      .filter((bi): bi is typeof bundleItems[number] => !!bi);
    if (reordered.length !== bundleItems.length) return;
    bundleItems = reordered.map((bi, idx) => ({ ...bi, sort_order: idx }));
  }

  function onReorderImages(orderedIds: string[]) {
    const byId = new Map(images.map(img => [img.id, img]));
    const reordered = orderedIds
      .map(id => byId.get(id))
      .filter((img): img is typeof images[number] => !!img);
    if (reordered.length !== images.length) return;
    void persistReorder(reordered);
  }

  async function persistReorder(reordered: typeof images) {
    reorderSaving = true;
    reorderError = null;
    const snapshot = [...images];
    try {
      const fd = new FormData();
      fd.set('image_ids', reordered.map(img => img.id).join(','));
      const res = await fetch('?/reorderImages', { method: 'POST', body: fd });
      if (!res.ok) throw new Error();
      images = reordered.map((img, i) => ({ ...img, sort_order: i }));
    } catch {
      reorderError = m.admin_product_edit_media_reorder_failed();
      images = snapshot;
    } finally {
      reorderSaving = false;
    }
  }
</script>

<svelte:head>
  <title>{data.isNew ? m.admin_product_edit_new_title() : m.admin_product_edit_edit_title({ name: data.product?.name ?? '' })}</title>
</svelte:head>

<!-- Back + title -->
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/products" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">{m.admin_product_edit_back()}</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">
      {data.isNew ? m.admin_product_edit_new_heading() : (data.product?.name ?? m.admin_product_edit_edit_heading())}
    </h1>
  </div>

  <form id="product-form" method="POST" action="?/saveProduct"
        use:enhance={({ cancel }) => {
          // Gate destructive bundle→simple change behind a confirm dialog.
          if (isBundleToSimplePending && !kindChangeConfirmed) {
            cancel();
            confirmKindChange = true;
            return;
          }
          if (saving) return;
          saving = true;
          const productName = name;
          return async ({ result, update }) => {
            showResult(result,
              data.isNew ? m.admin_product_edit_save_create_success({ name: productName }) : m.admin_product_edit_save_save_success({ name: productName }),
              data.isNew ? m.admin_product_edit_save_create_failure({ name: productName }) : m.admin_product_edit_save_save_failure({ name: productName }));
            await update({ reset: false });
            saving = false;
            kindChangeConfirmed = false;
          };
        }}>

    <!-- ── Product Details ── -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
      <h2 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_section_details()}</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="name" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_name()} {m.admin_product_edit_required_marker()}</label>
          <input id="name" name="name" required bind:value={name}
                 oninput={() => { autoSlug = true; }}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="slug" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_slug()} {m.admin_product_edit_required_marker()}</label>
          <input id="slug" name="slug" required bind:value={slug}
                 oninput={() => { autoSlug = false; }}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
        </div>
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <label for="subtitle" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_subtitle()}</label>
          <input id="subtitle" name="subtitle" type="text" maxlength="255"
                 value={data.product?.subtitle ?? ''}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_status()}</label>
          <select name="status"
                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="active" selected={data.isNew || data.product?.status === 'active'}>{m.admin_product_edit_status_active()}</option>
            <option value="inactive" selected={!data.isNew && data.product?.status === 'inactive'}>{m.admin_product_edit_status_inactive()}</option>
          </select>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_type()}</label>
          <select name="kind" bind:value={kind}
                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="simple">{m.admin_product_edit_type_simple()}</option>
            <option value="bundle">{m.admin_product_edit_type_bundle()}</option>
          </select>
        </div>
        <div class="flex flex-col gap-1.5">
          <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_product_edit_label_additional_categories()}
          </span>
          <MultiSelect
            options={categoryOptions}
            selected={categoryIDs}
            placeholder={m.admin_product_edit_additional_categories_placeholder()}
            onChange={(values) => (categoryIDs = values)}
          />
          {#each categoryIDs as id (id)}
            <input type="hidden" name="category_ids" value={id} />
          {/each}
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="category_id" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_category()}</label>
          <select id="category_id" name="category_id" bind:value={primaryCategoryID}
                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="">{m.admin_product_edit_category_none()}</option>
            {#each primaryCategoryChoices as cat}
              <option value={cat.id}>{cat.name}</option>
            {/each}
          </select>
        </div>
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <label for="excerpt" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_excerpt()}</label>
          <textarea id="excerpt" name="excerpt" rows="3"
                    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"
                    >{data.product?.excerpt ?? ''}</textarea>
        </div>
      </div>
    </section>

    {#if data.isNew}
      <input type="hidden" name="pending_variants" value={pendingVariantsJson} />
      <input type="hidden" name="pending_images"   value={pendingImagesJson} />
      {#if kind === 'bundle'}
        <input type="hidden" name="pending_bundle_price"            value={pendingBundlePrice} />
        <input type="hidden" name="pending_bundle_compare_at_price" value={pendingBundleCompareAtPrice} />
        <input type="hidden" name="pending_bundle_weight_grams"     value={pendingBundleWeightGrams} />
        <input type="hidden" name="pending_bundle_length_mm"        value={pendingBundleLengthMM} />
        <input type="hidden" name="pending_bundle_width_mm"         value={pendingBundleWidthMM} />
        <input type="hidden" name="pending_bundle_height_mm"        value={pendingBundleHeightMM} />
        <input type="hidden" name="pending_bundle_items"            value={bundleItemsJson} />
      {/if}
    {:else if kind === 'bundle'}
      <input type="hidden" name="bundle_items_json"            value={bundleItemsJson} />
      <input type="hidden" name="bundle_price"                 value={pendingBundlePrice} />
      <input type="hidden" name="bundle_compare_at_price"      value={pendingBundleCompareAtPrice} />
      <input type="hidden" name="bundle_weight_grams"          value={pendingBundleWeightGrams} />
      <input type="hidden" name="bundle_length_mm"             value={pendingBundleLengthMM} />
      <input type="hidden" name="bundle_width_mm"              value={pendingBundleWidthMM} />
      <input type="hidden" name="bundle_height_mm"             value={pendingBundleHeightMM} />
    {/if}
  </form>

  <!-- ── bundle → simple destructive change warning ── -->
  {#if isBundleToSimplePending}
    {@const bundleItemCount = (data.bundleItems ?? []).length}
    <section class="bg-amber-50 border border-amber-200 rounded-2xl p-4 mb-6 flex items-start gap-3">
      <svg class="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"/>
      </svg>
      <div class="text-sm text-amber-900">
        <p class="font-semibold mb-1">{m.admin_product_edit_kind_warning_title()}</p>
        <p class="text-amber-800">
          {bundleItemCount === 0
            ? m.admin_product_edit_kind_warning_body_no_components()
            : bundleItemCount === 1
              ? m.admin_product_edit_kind_warning_body_with_components_one({ count: bundleItemCount })
              : m.admin_product_edit_kind_warning_body_with_components_many({ count: bundleItemCount })}
        </p>
      </div>
    </section>
  {/if}

  <!-- ── Variants (simple products only) ── -->
  {#if kind === 'simple'}
    <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
        <h2 class="font-semibold text-gray-900">{m.admin_product_edit_variants_heading()}</h2>
        <button onclick={() => showAddVariant = true}
                class="px-3 py-1.5 bg-gray-900 text-white text-xs font-medium rounded-lg
                       hover:bg-gray-700 transition-colors">
          {m.admin_product_edit_variants_add()}
        </button>
      </div>

      <div use:spotlight={{ selector: '.js-variant-row' }}>
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-100">
          <tr>
            <th class="px-2 py-3 w-8"></th>
            <th class="px-5 py-3 w-12"></th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_col_sku()}</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_col_name()}</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_col_price()}</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">{m.admin_product_edit_variants_col_compare_at()}</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_col_stock()}</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden md:table-cell">{m.admin_product_edit_variants_col_status()}</th>
            <th class="px-5 py-3"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50"
               use:sortable={{
                 onReorder: onReorderVariants,
                 handle: '[data-drag-handle]',
                 filter: 'button, form, input, a, [role="button"]'
               }}>
          {#if data.isNew}
            {#each pendingVariants as pv (pv._localId)}
              <tr class="js-variant-row" data-id={pv._localId}>
                <td class="px-2 py-3 text-gray-300 cursor-grab active:cursor-grabbing select-none" data-drag-handle aria-hidden="true">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9h16.5m-16.5 6.75h16.5"/>
                  </svg>
                </td>
                <td class="px-5 py-3">
                  {#if pv.image_preview_url}
                    {#if detectStreamingVideoFromURL(pv.image_preview_url)}
                      <div class="w-8 h-8 rounded bg-black flex items-center justify-center">
                        <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                      </div>
                    {:else if isVideo({ url: pv.image_preview_url })}
                      <video src={pv.image_preview_url} muted playsinline preload="metadata"
                             class="w-8 h-8 rounded object-cover bg-black"></video>
                    {:else}
                      <ResponsiveImage src={pv.image_preview_url} alt=""
                                       widths={TINY_THUMB_WIDTHS} sizes={TINY_THUMB_SIZES}
                                       class="w-8 h-8 rounded object-cover" />
                    {/if}
                  {:else}
                    <div class="w-8 h-8 rounded bg-gray-100"></div>
                  {/if}
                </td>
                <td class="px-5 py-3 font-mono text-xs text-gray-700">{pv.sku}</td>
                <td class="px-5 py-3 text-sm text-gray-900">{pv.name ?? m.admin_product_edit_variants_dash()}</td>
                <td class="px-5 py-3 font-medium text-gray-900">HK${pv.price.toFixed(2)}</td>
                <td class="px-5 py-3 text-gray-400 hidden sm:table-cell">
                  {pv.compare_at_price ? `HK$${pv.compare_at_price.toFixed(2)}` : m.admin_product_edit_variants_dash()}
                </td>
                <td class="px-5 py-3">
                  <span class="font-medium text-gray-900">{pv.stock_qty}</span>
                </td>
                <td class="px-5 py-3 hidden md:table-cell">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700">{m.admin_product_edit_status_active()}</span>
                </td>
                <td class="px-5 py-3 text-right">
                  <button type="button"
                          title={m.admin_product_edit_variants_tip_remove()}
                          aria-label={m.admin_product_edit_variants_aria_remove_pending()}
                          onclick={() => pendingVariants = pendingVariants.filter(v => v._localId !== pv._localId)}
                          class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="9" class="px-5 py-8 text-center text-gray-400 text-sm">
                  {m.admin_product_edit_variants_empty()}
                </td>
              </tr>
            {/each}
          {:else}
            {#each data.variants as variant (variant.id)}
              <tr class="js-variant-row" data-id={variant.id}>
                <td class="px-2 py-3 text-gray-300 cursor-grab active:cursor-grabbing select-none" data-drag-handle aria-hidden="true">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9h16.5m-16.5 6.75h16.5"/>
                  </svg>
                </td>
                <td class="px-5 py-3">
                  {#if variant.image_url}
                    {#if detectStreamingVideoFromURL(variant.image_url)}
                      <div class="w-8 h-8 rounded bg-black flex items-center justify-center">
                        <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                      </div>
                    {:else if isVideo({ url: variant.image_url })}
                      <video src={variant.image_url} muted playsinline preload="metadata"
                             class="w-8 h-8 rounded object-cover bg-black"></video>
                    {:else}
                      <ResponsiveImage src={variant.image_url} alt=""
                                       widths={TINY_THUMB_WIDTHS} sizes={TINY_THUMB_SIZES}
                                       class="w-8 h-8 rounded object-cover" />
                    {/if}
                  {:else}
                    <div class="w-8 h-8 rounded bg-gray-100"></div>
                  {/if}
                </td>
                <td class="px-5 py-3 font-mono text-xs text-gray-700">{variant.sku}</td>
                <td class="px-5 py-3 text-sm text-gray-900">{variant.name ?? m.admin_product_edit_variants_dash()}</td>
                <td class="px-5 py-3 font-medium text-gray-900">HK${variant.price.toFixed(2)}</td>
                <td class="px-5 py-3 text-gray-400 hidden sm:table-cell">
                  {variant.compare_at_price ? `HK$${variant.compare_at_price.toFixed(2)}` : m.admin_product_edit_variants_dash()}
                </td>
                <td class="px-5 py-3">
                  <span class="font-medium {variant.stock_qty <= 5 ? 'text-red-600' : 'text-gray-900'}">
                    {variant.stock_qty}
                  </span>
                </td>
                <td class="px-5 py-3 hidden md:table-cell">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                               {variant.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
                    {variant.is_active ? m.admin_product_edit_status_active() : m.admin_product_edit_status_inactive()}
                  </span>
                </td>
                <td class="px-5 py-3 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <!-- Adjust Stock -->
                    <button onclick={() => showStockModal = variant}
                            title={m.admin_product_edit_variants_tip_adjust_stock()}
                            aria-label={m.admin_product_edit_variants_aria_adjust_stock()}
                            class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
                      </svg>
                    </button>
                    <!-- Stock History -->
                    <button onclick={() => openHistory(variant)}
                            title={m.admin_product_edit_variants_tip_history()}
                            aria-label={m.admin_product_edit_variants_tip_history()}
                            class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
                      </svg>
                    </button>
                    <!-- Edit -->
                    <button onclick={() => {
                              editingVariant = variant;
                              const cur = data.images.find(img => img.variant_id === variant.id);
                              editVariantOldImageId = cur?.id ?? null;
                              editVariantImageId = cur?.media_file_id ?? null;
                              editVariantRemoveImage = false;
                            }}
                            title={m.admin_product_edit_variants_tip_edit()}
                            aria-label={m.admin_product_edit_variants_aria_edit()}
                            class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                      </svg>
                    </button>
                    <!-- Delete -->
                    <form method="POST" action="?/deleteVariant" class="inline-flex"
                          use:enhance={() => {
                            const sku = variant.sku;
                            return async ({ result, update }) => {
                              showResult(result, m.admin_product_edit_variant_deleted_success({ sku }), m.admin_product_edit_variant_deleted_failure({ sku }));
                              await update();
                            };
                          }}>
                      <input type="hidden" name="variant_id" value={variant.id} />
                      <button type="submit"
                              title={m.admin_product_edit_variants_tip_delete()}
                              aria-label={m.admin_product_edit_variants_aria_delete()}
                              class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                              onclick={(e) => { if (!confirm(m.admin_product_edit_variants_confirm_delete())) e.preventDefault(); }}>
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                        </svg>
                      </button>
                    </form>
                  </div>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="9" class="px-5 py-8 text-center text-gray-400 text-sm">
                  {m.admin_product_edit_variants_empty()}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
      </div>
    </section>
  {/if}

  <!-- ── Bundle Pricing (bundle products only) ── -->
  {#if kind === 'bundle'}
    {@const bv = data.isNew ? null : (data.variants[0] ?? null)}
    <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
      <h2 class="font-semibold text-gray-900 mb-1">{m.admin_product_edit_bundle_pricing_heading()}</h2>
      <p class="text-xs text-gray-400 mb-4">{m.admin_product_edit_bundle_pricing_subtitle()}</p>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_bundle_label_price()} {m.admin_product_edit_required_marker()}</label>
          <input type="number" step="0.01" min="0" required form="product-form"
                 bind:value={pendingBundlePrice}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_bundle_label_compare_at()}</label>
          <input type="number" step="0.01" min="0" form="product-form"
                 bind:value={pendingBundleCompareAtPrice}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_bundle_label_weight()}</label>
          <input type="number" min="0" step="1" placeholder={m.admin_product_edit_bundle_label_optional()} form="product-form"
                 bind:value={pendingBundleWeightGrams}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="col-span-2 flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_dimensions()}</label>
          <div class="grid grid-cols-3 gap-2">
            <input type="number" min="0" step="1" placeholder={m.admin_product_edit_add_variant_dim_l()} form="product-form"
                   bind:value={pendingBundleLengthMM}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            <input type="number" min="0" step="1" placeholder={m.admin_product_edit_add_variant_dim_w()} form="product-form"
                   bind:value={pendingBundleWidthMM}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            <input type="number" min="0" step="1" placeholder={m.admin_product_edit_add_variant_dim_h()} form="product-form"
                   bind:value={pendingBundleHeightMM}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_bundle_label_derived_stock()}</label>
          <div class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm text-gray-500 bg-gray-50">
            {m.admin_product_edit_bundle_derived_stock_units({ count: bv?.stock_qty ?? 0 })}
          </div>
        </div>
      </div>
      <p class="mt-3 text-xs text-gray-400">
        {data.isNew
          ? m.admin_product_edit_bundle_pricing_hint_new()
          : m.admin_product_edit_bundle_pricing_hint_existing()}
      </p>
    </section>
  {/if}

  <!-- ── Bundle Contents (bundle products only) ── -->
  {#if kind === 'bundle'}
    <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
        <div>
          <h2 class="font-semibold text-gray-900">{m.admin_product_edit_bundle_contents_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_product_edit_bundle_contents_subtitle()}</p>
        </div>
      </div>

      <!-- Component list -->
      {#if bundleItems.length === 0}
        <p class="px-6 py-6 text-sm text-gray-400 text-center">{m.admin_product_edit_bundle_contents_empty()}</p>
      {:else}
        <table class="w-full text-sm">
          <thead class="bg-gray-50 border-b border-gray-100">
            <tr>
              <th class="px-2 py-3"></th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_bundle_contents_col_product()}</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_bundle_contents_col_sku()}</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_bundle_contents_col_qty()}</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden md:table-cell">{m.admin_product_edit_bundle_contents_col_display_override()}</th>
              <th class="px-5 py-3"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-50"
                 use:sortable={{ onReorder: onReorderBundleItems, handle: '[data-drag-handle]' }}>
            {#each bundleItems as bi, idx (bi._localId)}
              <tr data-id={bi._localId}>
                <td class="px-2 py-3 text-gray-300 cursor-grab active:cursor-grabbing select-none" data-drag-handle aria-hidden="true">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9h16.5m-16.5 6.75h16.5"/>
                  </svg>
                </td>
                <td class="px-5 py-3 font-medium text-gray-900">{bi.component_product_name ?? m.admin_product_edit_variants_dash()}</td>
                <td class="px-5 py-3 font-mono text-xs text-gray-600">{bi.component_sku ?? m.admin_product_edit_variants_dash()}</td>
                <td class="px-5 py-3">
                  <input type="number" min="1" value={bi.quantity}
                         oninput={(e) => { bundleItems[idx] = { ...bundleItems[idx], quantity: parseInt((e.target as HTMLInputElement).value) || 1 }; }}
                         class="w-16 border border-gray-200 rounded-lg px-2 py-1 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                </td>
                <td class="px-5 py-3 hidden md:table-cell">
                  <input type="text" value={bi.display_name_override ?? ''}
                         placeholder={m.admin_product_edit_bundle_contents_optional()}
                         oninput={(e) => { bundleItems[idx] = { ...bundleItems[idx], display_name_override: (e.target as HTMLInputElement).value || undefined }; }}
                         class="w-full border border-gray-200 rounded-lg px-2 py-1 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                </td>
                <td class="px-5 py-3 text-right">
                  <button type="button"
                          title={m.admin_product_edit_bundle_contents_tip_remove()}
                          aria-label={m.admin_product_edit_bundle_contents_aria_remove()}
                          onclick={() => bundleItems = bundleItems.filter(b => b._localId !== bi._localId)}
                          class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}

      <!-- Add component row -->
      <div class="px-6 py-4 border-t border-gray-100 bg-gray-50">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_product_edit_bundle_add_heading()}</p>
        <div class="flex flex-wrap gap-3 items-end">
          <div class="flex flex-col gap-1">
            <label class="text-xs text-gray-500">{m.admin_product_edit_bundle_add_label_product()}</label>
            <select bind:value={pickerProductId}
                    onchange={() => loadPickerVariants(pickerProductId)}
                    class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900 min-w-[180px]">
              <option value="">{m.admin_product_edit_bundle_add_select_product()}</option>
              {#each simpleProducts as p}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-xs text-gray-500">{m.admin_product_edit_bundle_add_label_variant()}</label>
            <select bind:value={pickerVariantId}
                    disabled={!pickerProductId || loadingPickerVariants}
                    class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900 min-w-[160px]
                           disabled:opacity-50">
              <option value="">
                {loadingPickerVariants ? m.admin_product_edit_bundle_add_loading() : m.admin_product_edit_bundle_add_select_variant()}
              </option>
              {#each pickerVariants as v}
                <option value={v.id}>{v.sku}{v.name ? ` — ${v.name}` : ''}</option>
              {/each}
            </select>
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-xs text-gray-500">{m.admin_product_edit_bundle_add_label_qty()}</label>
            <input type="number" min="1" bind:value={pickerQty}
                   class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900 w-20" />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-xs text-gray-500">{m.admin_product_edit_bundle_add_label_display()}</label>
            <input type="text" bind:value={pickerDisplayName}
                   placeholder={m.admin_product_edit_bundle_add_display_placeholder()}
                   class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900 min-w-[160px]" />
          </div>
          <button type="button"
                  onclick={addBundleComponent}
                  disabled={!pickerVariantId}
                  class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed">
            {m.admin_product_edit_bundle_add_button()}
          </button>
        </div>
      </div>

      <div class="px-6 py-3 border-t border-gray-100 text-xs text-gray-400">
        {data.isNew
          ? m.admin_product_edit_bundle_contents_hint_new()
          : m.admin_product_edit_bundle_contents_hint_existing()}
      </div>
    </section>
  {/if}

  <!-- ── Media ── -->
  <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
    <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
      <h2 class="font-semibold text-gray-900">
        {m.admin_product_edit_media_heading()}
        {#if !data.isNew && reorderSaving}
          <span class="ml-2 text-xs font-normal text-gray-400">{m.admin_product_edit_media_saving_order()}</span>
        {/if}
      </h2>
      <button onclick={() => showAddImage = true}
              class="px-3 py-1.5 bg-gray-900 text-white text-xs font-medium rounded-lg
                     hover:bg-gray-700 transition-colors">
        {m.admin_product_edit_media_add()}
      </button>
    </div>

    {#if !data.isNew && reorderError}
      <div class="px-6 py-2 text-sm text-red-500 bg-red-50 border-b border-red-100 flex items-center justify-between">
        {reorderError}
        <button onclick={() => reorderError = null} class="ml-4 text-red-400 hover:text-red-600">✕</button>
      </div>
    {/if}

    {#if data.isNew}
      {#if pendingImages.length === 0}
        <div class="px-6 py-8 text-center text-gray-400 text-sm">{m.admin_product_edit_media_empty()}</div>
      {:else}
        <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 p-6">
          {#each pendingImages as pi (pi._localId)}
            <div class="aspect-square rounded-xl overflow-hidden relative group bg-gray-100">
              {#if isVideo({ url: pi.preview_url })}
                <video src={pi.preview_url} muted loop playsinline preload="metadata"
                       class="w-full h-full object-cover"></video>
                <span class="absolute inset-0 flex items-center justify-center pointer-events-none"
                      aria-hidden="true">
                  <span class="flex items-center justify-center w-12 h-12 rounded-full bg-black/60 text-white shadow-lg">
                    <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M8 5v14l11-7z"/>
                    </svg>
                  </span>
                </span>
              {:else}
                <ResponsiveImage src={pi.preview_url} alt={pi.alt_text ?? ''}
                                 widths={GALLERY_WIDTHS} sizes={GALLERY_SIZES}
                                 class="w-full h-full object-cover" />
              {/if}
              {#if pi.is_primary}
                <span class="absolute top-2 left-2 p-1.5 rounded-lg bg-amber-400/90 text-white"
                      title={m.admin_product_edit_media_tip_primary()} aria-label={m.admin_product_edit_media_aria_primary()}>
                  <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                  </svg>
                </span>
              {/if}
              <div class="absolute inset-0 bg-gray-900/70 opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-end justify-center p-2.5">
                <div class="flex items-center justify-center gap-1.5">
                  {#if !pi.is_primary}
                    <button type="button"
                            title={m.admin_product_edit_media_tip_set_primary()} aria-label={m.admin_product_edit_media_aria_set_primary()}
                            onclick={() => { pendingImages = pendingImages.map(p => ({ ...p, is_primary: p._localId === pi._localId })); }}
                            class="p-1.5 rounded-lg bg-white/10 hover:bg-amber-400/90 transition-colors text-white">
                      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                      </svg>
                    </button>
                  {/if}
                  <button type="button"
                          title={m.admin_product_edit_media_tip_remove()} aria-label={m.admin_product_edit_media_aria_remove()}
                          onclick={() => {
                            const removing = pendingImages.find(p => p._localId === pi._localId);
                            pendingImages = pendingImages.filter(p => p._localId !== pi._localId);
                            if (removing?.is_primary && pendingImages.length > 0) {
                              pendingImages = pendingImages.map((p, i) => i === 0 ? { ...p, is_primary: true } : p);
                            }
                          }}
                          class="p-1.5 rounded-lg bg-white/10 hover:bg-red-500/80 transition-colors text-white">
                    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {:else}
      {#if images.length === 0}
        <div class="px-6 py-8 text-center text-gray-400 text-sm">{m.admin_product_edit_media_empty()}</div>
      {:else}
        <div class="js-media-grid grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 p-6"
             use:sortable={{
               onReorder: onReorderImages,
               handle: false,
               filter: '[data-primary="true"], button, form, input, a, [role="button"]',
               onMove: (evt) => (evt.related as HTMLElement)?.dataset.primary !== 'true'
             }}>
          {#each images as image (image.id)}
            <div
              data-id={image.id}
              data-primary={image.is_primary ? 'true' : undefined}
              class="aspect-square rounded-xl overflow-hidden relative group bg-gray-100
                     {image.is_primary ? '' : 'cursor-grab active:cursor-grabbing'}"
            >
              {#if isVideo(image)}
                {#if isStreamingVideo(image)}
                  <ResponsiveImage src={image.thumbnail_url ?? ''} alt={image.alt_text ?? ''}
                                   widths={GALLERY_WIDTHS} sizes={GALLERY_SIZES}
                                   class="w-full h-full object-cover pointer-events-none bg-gray-100" />
                {:else}
                  <video src={image.url} muted loop playsinline preload="metadata"
                         class="w-full h-full object-cover pointer-events-none"></video>
                {/if}
                <span class="absolute inset-0 flex items-center justify-center pointer-events-none"
                      aria-hidden="true">
                  <span class="flex items-center justify-center w-12 h-12 rounded-full bg-black/60 text-white shadow-lg">
                    <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M8 5v14l11-7z"/>
                    </svg>
                  </span>
                </span>
              {:else}
                <ResponsiveImage src={image.url} alt={image.alt_text ?? ''}
                                 widths={GALLERY_WIDTHS} sizes={GALLERY_SIZES}
                                 class="w-full h-full object-cover" />
              {/if}

              {#if image.is_primary}
                <span class="absolute top-2 left-2 p-1.5 rounded-lg bg-amber-400/90 text-white"
                      title={m.admin_product_edit_media_tip_primary()} aria-label={m.admin_product_edit_media_aria_primary()}>
                  <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                  </svg>
                </span>
              {/if}

              <div class="absolute inset-0 bg-gray-900/70 opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-end justify-center p-2.5">
                <div class="flex items-center justify-center gap-1.5">
                  {#if !image.is_primary}
                    <form method="POST" action="?/setPrimary"
                          use:enhance={() => async ({ result, update }) => {
                            showResult(result, m.admin_product_edit_media_primary_set_success(), m.admin_product_edit_media_primary_set_failure());
                            await update();
                          }}>
                      <input type="hidden" name="image_id" value={image.id} />
                      <input type="hidden" name="sort_order" value={image.sort_order} />
                      <button type="submit"
                              title={m.admin_product_edit_media_tip_set_primary()} aria-label={m.admin_product_edit_media_aria_set_primary()}
                              class="p-1.5 rounded-lg bg-white/10 hover:bg-amber-400/90 transition-colors text-white">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                        </svg>
                      </button>
                    </form>
                  {/if}
                  <button type="button"
                          title={m.admin_product_edit_media_tip_delete()} aria-label={m.admin_product_edit_media_aria_delete()}
                          class="p-1.5 rounded-lg bg-white/10 hover:bg-red-500/80 transition-colors text-white"
                          onclick={() => confirmDeleteImageId = image.id}>
                    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {/if}
  </section>

  <!-- ── 內容 ── -->
  <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_section_content()}</h2>
    <div class="flex flex-col gap-3">
      <SingleMediaPicker
        files={bannerLibrary}
        value={banner1MediaId}
        token={data.token ?? null}
        label={m.admin_product_edit_label_banner_1()}
        description={m.admin_product_edit_help_banner_1()}
        name="banner_1_media_id"
        form="product-form"
        onChange={(id) => (banner1MediaId = id)}
        onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        previewClass="aspect-[16/9] w-full max-w-md"
      />
      <div class="flex flex-col gap-1.5">
        <div class="flex items-center justify-between">
          <label for="description" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_product_edit_label_content()}
            <span class="normal-case font-normal text-gray-400">{m.admin_product_edit_content_markdown_hint()}</span>
          </label>
          <button type="button" onclick={() => descriptionPreview = !descriptionPreview}
                  class="text-[11px] uppercase tracking-wide text-gray-500 hover:text-gray-900 transition-colors">
            {descriptionPreview ? m.admin_cms_post_edit_edit_button() : m.admin_cms_post_edit_preview_button()}
          </button>
        </div>
        <ShortcodeToolbar bind:value={description} textarea={descriptionTextarea} />
        <div class="{descriptionPreview ? 'grid grid-cols-1 lg:grid-cols-2 gap-3' : ''}">
          <textarea id="description" name="description" form="product-form" rows="10" bind:value={description} bind:this={descriptionTextarea}
                    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                           focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
          {#if descriptionPreview}
            <div class="rounded-xl border border-gray-100 bg-gray-50 px-4 py-3 prose prose-sm max-w-none overflow-y-auto"
                 style="max-height: 360px">
              <MarkdownContent content={description || m.admin_cms_post_edit_preview_no_content()} placeholderMode />
            </div>
          {/if}
        </div>
      </div>
    </div>
  </section>

  <!-- ── 使用方法 ── -->
  <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_section_how_to_use()}</h2>
    <div class="flex flex-col gap-3">
      <SingleMediaPicker
        files={bannerLibrary}
        value={banner2MediaId}
        token={data.token ?? null}
        label={m.admin_product_edit_label_banner_2()}
        description={m.admin_product_edit_help_banner_2()}
        name="banner_2_media_id"
        form="product-form"
        onChange={(id) => (banner2MediaId = id)}
        onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        previewClass="aspect-[16/9] w-full max-w-md"
      />
      <div class="flex flex-col gap-1.5">
        <div class="flex items-center justify-between">
          <label for="how_to_use" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_product_edit_label_how_to_use()}
            <span class="normal-case font-normal text-gray-400">{m.admin_product_edit_content_markdown_hint()}</span>
          </label>
          <button type="button" onclick={() => howToUsePreview = !howToUsePreview}
                  class="text-[11px] uppercase tracking-wide text-gray-500 hover:text-gray-900 transition-colors">
            {howToUsePreview ? m.admin_cms_post_edit_edit_button() : m.admin_cms_post_edit_preview_button()}
          </button>
        </div>
        <ShortcodeToolbar bind:value={howToUse} textarea={howToUseTextarea} />
        <div class="{howToUsePreview ? 'grid grid-cols-1 lg:grid-cols-2 gap-3' : ''}">
          <textarea id="how_to_use" name="how_to_use" form="product-form" rows="10" bind:value={howToUse} bind:this={howToUseTextarea}
                    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                           focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
          {#if howToUsePreview}
            <div class="rounded-xl border border-gray-100 bg-gray-50 px-4 py-3 prose prose-sm max-w-none overflow-y-auto"
                 style="max-height: 360px">
              <MarkdownContent content={howToUse || m.admin_cms_post_edit_preview_no_content()} placeholderMode />
            </div>
          {/if}
        </div>
      </div>
    </div>
  </section>

  <!-- ── 適用表面 ── -->
  <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_section_compatible_surfaces()}</h2>
    <div class="grid grid-cols-2 sm:grid-cols-3 gap-2">
      {#each SURFACE_KEYS as key}
        <label class="flex items-center gap-2 px-3 py-2 border border-gray-200 rounded-xl text-sm cursor-pointer
                      hover:bg-gray-50 transition-colors">
          <input type="checkbox" name="compatible_surfaces" form="product-form" value={key}
                 checked={selectedSurfaces.has(key)}
                 onchange={() => toggleSurface(key)}
                 class="rounded border-gray-300 focus:ring-gray-900" />
          <span>{surfaceLabels[key]()}</span>
        </label>
      {/each}
    </div>
  </section>

  <!-- ── 影片與媒體 ── -->
  <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_section_video_and_media()}</h2>

    <div class="flex flex-col gap-1.5">
      <label for="video_id" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
        {m.admin_product_edit_label_video_id()}
      </label>
      <input id="video_id" name="video_id" form="product-form" type="text" maxlength="32"
             placeholder="IuUvIgc_9UA"
             bind:value={videoId}
             class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                    focus:outline-none focus:ring-2 focus:ring-gray-900" />
      <p class="text-xs text-gray-500">{m.admin_product_edit_help_video_id()}</p>
    </div>

    <div class="flex flex-col gap-2 mt-6">
      <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
        {m.admin_product_edit_section_media_strip()}
      </span>
      <p class="text-xs text-gray-500">{m.admin_product_edit_help_media_strip()}</p>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <SingleMediaPicker
          files={bannerLibrary}
          value={media1MediaId}
          token={data.token ?? null}
          name="media_1_media_id"
          form="product-form"
          onChange={(id) => (media1MediaId = id)}
          onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        />
        <SingleMediaPicker
          files={bannerLibrary}
          value={media2MediaId}
          token={data.token ?? null}
          name="media_2_media_id"
          form="product-form"
          onChange={(id) => (media2MediaId = id)}
          onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        />
        <SingleMediaPicker
          files={bannerLibrary}
          value={media3MediaId}
          token={data.token ?? null}
          name="media_3_media_id"
          form="product-form"
          onChange={(id) => (media3MediaId = id)}
          onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        />
        <SingleMediaPicker
          files={bannerLibrary}
          value={media4MediaId}
          token={data.token ?? null}
          name="media_4_media_id"
          form="product-form"
          onChange={(id) => (media4MediaId = id)}
          onUpload={(mf) => (uploadedMedia = [mf, ...uploadedMedia])}
        />
      </div>
    </div>
  </section>

  {#if !data.isNew && data.product?.id}
    <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <button type="button" onclick={toggleProductHistory}
              class="w-full flex items-center justify-between px-6 py-4 text-left hover:bg-gray-50 transition-colors">
        <div>
          <h3 class="text-base font-bold text-gray-900">{m.admin_product_stock_history_heading()}</h3>
          <p class="text-xs text-gray-500 mt-0.5">{m.admin_product_stock_history_subtitle()}</p>
        </div>
        <svg class="w-5 h-5 text-gray-400 transition-transform" class:rotate-180={productHistoryOpen}
             fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5"/>
        </svg>
      </button>
      {#if productHistoryOpen}
        <div class="border-t border-gray-100">
          {#if productHistoryLoading}
            <p class="px-6 py-12 text-sm text-gray-400 text-center">{m.admin_product_edit_variants_history_loading()}</p>
          {:else}
            <StockMovementTable items={productHistoryRows} />
          {/if}
        </div>
      {/if}
    </section>
  {/if}

  <!-- ── Actions ── -->
  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5
              flex flex-col sm:flex-row sm:items-center gap-4">
    {#if data.isNew && anyUploading}
      <p class="text-xs text-amber-600 sm:mr-auto">{m.admin_product_edit_actions_uploading_wait()}</p>
    {/if}
    <div class="sm:ml-auto flex gap-3">
      <a href="/admin/products"
         class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                text-gray-700 hover:bg-gray-50 transition-colors">
        {m.admin_product_edit_actions_cancel()}
      </a>
      <SaveButton type="submit" form="product-form" loading={saving} disabled={data.isNew && anyUploading}
              class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                     text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
        {data.isNew && anyUploading ? m.admin_product_edit_actions_uploading() : data.isNew ? m.admin_product_edit_actions_create() : m.admin_product_edit_actions_save()}
      </SaveButton>
    </div>
  </div>

<!-- ── Add Variant Modal ── -->
{#if showAddVariant}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showAddVariant = false} role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <h3 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_add_variant_title()}</h3>
      <form method="POST" action="?/addVariant"
            use:enhance={({ formData, cancel }) => {
              if (savingVariant) { cancel(); return; }
              const sku = formData.get('sku')?.toString() ?? '';
              if (data.isNew) {
                cancel();
                if (!sku) return;
                if (pendingVariants.some(v => v.sku === sku)) {
                  notify.warning(m.admin_product_edit_add_variant_duplicate_title(), m.admin_product_edit_add_variant_duplicate_body({ sku }));
                  return;
                }
                const imageId = addVariantImageId ?? undefined;
                const imageMedia_ = imageMedia.find(m => m.id === imageId);
                const nameVal = formData.get('name')?.toString().trim() ?? '';
                const weightStr = formData.get('weight_grams')?.toString().trim() ?? '';
                const lenStr = formData.get('length_mm')?.toString().trim() ?? '';
                const widStr = formData.get('width_mm')?.toString().trim() ?? '';
                const hgtStr = formData.get('height_mm')?.toString().trim() ?? '';
                pendingVariants = [...pendingVariants, {
                  _localId: crypto.randomUUID(),
                  sku,
                  name: nameVal || undefined,
                  price: parseFloat(formData.get('price')?.toString() ?? '0'),
                  compare_at_price: formData.get('compare_at_price')?.toString()
                    ? parseFloat(formData.get('compare_at_price')!.toString())
                    : undefined,
                  stock_qty: parseInt(formData.get('stock_qty')?.toString() ?? '0', 10),
                  weight_grams: weightStr ? parseInt(weightStr, 10) : undefined,
                  length_mm: lenStr ? parseInt(lenStr, 10) : undefined,
                  width_mm:  widStr ? parseInt(widStr,  10) : undefined,
                  height_mm: hgtStr ? parseInt(hgtStr,  10) : undefined,
                  image_media_file_id: imageId,
                  image_preview_url: imageMedia_?.webp_url ?? imageMedia_?.url
                }];
                showAddVariant = false;
                addVariantImageId = null;
                return;
              }
              savingVariant = true;
              return async ({ result, update }) => {
                showResult(result, m.admin_product_edit_add_variant_added_success({ sku }), m.admin_product_edit_add_variant_added_failure({ sku }));
                await update();
                savingVariant = false;
                if (result.type === 'success') { showAddVariant = false; addVariantImageId = null; }
              };
            }}>
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_sku()} {m.admin_product_edit_required_marker()}</label>
            <input name="sku" required class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                   focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_name()}</label>
            <input name="name" placeholder={m.admin_product_edit_add_variant_name_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_price()} {m.admin_product_edit_required_marker()}</label>
            <input name="price" type="number" step="0.01" min="0" required
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_compare_at()}</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_stock()}</label>
            <input name="stock_qty" type="number" min="0" value="0"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_low_stock_threshold()}</label>
            <input name="low_stock_threshold" type="number" min="0" step="1"
                   placeholder={m.admin_product_edit_add_variant_low_stock_threshold_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_weight()}</label>
            <input name="weight_grams" type="number" min="0" step="1"
                   placeholder={m.admin_product_edit_add_variant_weight_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_dimensions()}</label>
            <div class="grid grid-cols-3 gap-2">
              <input name="length_mm" type="number" min="0" step="1"
                     placeholder={m.admin_product_edit_add_variant_dim_l()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <input name="width_mm" type="number" min="0" step="1"
                     placeholder={m.admin_product_edit_add_variant_dim_w()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <input name="height_mm" type="number" min="0" step="1"
                     placeholder={m.admin_product_edit_add_variant_dim_h()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>
        <!-- Image picker -->
        <div class="mt-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_images()}</label>
          <input type="hidden" name="image_media_file_id" value={addVariantImageId ?? ''} />
          {#if imageMedia.length === 0}
            <p class="mt-2 text-xs text-gray-400">{m.admin_product_edit_add_variant_no_media()}</p>
          {:else}
            <div class="mt-2 flex gap-2 overflow-x-auto pb-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => addVariantImageId = addVariantImageId === mf.id ? null : mf.id}
                        style={mf.mime_type === 'link' ? 'display: none' : ''}
                        class="relative flex-none w-14 h-14 rounded-lg overflow-hidden border-2 transition-colors
                               {addVariantImageId === mf.id ? 'border-gray-900' : 'border-transparent'}">
                  {#if isVideo(mf)}
                    {#if isStreamingVideo(mf)}
                      <ResponsiveImage src={mf.thumbnail_url ?? ''} alt={mf.original_name}
                                       widths={[160, 320]} sizes="56px"
                                       class="w-full h-full object-cover bg-black" />
                    {:else}
                      <video src={mf.url} muted playsinline preload="metadata" class="w-full h-full object-cover bg-black"></video>
                    {/if}
                    <span class="absolute bottom-0.5 right-0.5 p-0.5 rounded bg-black/60 text-white" aria-hidden="true">
                      <svg class="w-2 h-2" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    </span>
                  {:else}
                    <ResponsiveImage src={mf.webp_url ?? mf.url} alt={mf.original_name}
                                     widths={[160, 320]} sizes="56px"
                                     class="w-full h-full object-cover"
                                     onload={mf.mime_type === 'link' ? (e) => { ((e.currentTarget as HTMLImageElement).parentElement as HTMLElement).style.display = ''; } : null}
                                     onerror={mf.mime_type === 'link' ? (e) => { ((e.currentTarget as HTMLImageElement).parentElement as HTMLElement).style.display = 'none'; } : null} />
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="flex gap-3 mt-5">
          <SaveButton loading={savingVariant}
                  class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900
                         text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
            {m.admin_product_edit_add_variant_submit()}
          </SaveButton>
          <button type="button" onclick={() => { showAddVariant = false; addVariantImageId = null; }}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            {m.admin_product_edit_add_variant_cancel()}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- ── Edit Variant Modal ── -->
{#if editingVariant}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => editingVariant = null} role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-md max-h-[90vh] flex flex-col overflow-hidden">
      <div class="p-6 pb-0">
        <h3 class="font-semibold text-gray-900">{m.admin_product_edit_edit_variant_title()}</h3>
      </div>
      <form method="POST" action="?/updateVariant"
            class="flex flex-col min-h-0 flex-1"
            use:enhance={({ formData }) => {
              if (updatingVariant) return;
              updatingVariant = true;
              const sku = formData.get('sku')?.toString() ?? '';
              return async ({ result, update }) => {
                showResult(result, m.admin_product_edit_edit_variant_saved_success({ sku }), m.admin_product_edit_edit_variant_saved_failure({ sku }));
                await update();
                updatingVariant = false;
                if (result.type === 'success') editingVariant = null;
              };
            }}>
        <input type="hidden" name="variant_id" value={editingVariant.id} />
        <input type="hidden" name="old_image_id" value={editVariantOldImageId ?? ''} />
        <input type="hidden" name="image_media_file_id" value={editVariantRemoveImage ? '' : (editVariantImageId ?? '')} />
        <input type="hidden" name="remove_image" value={String(editVariantRemoveImage)} />
        <div class="flex-1 overflow-y-auto min-h-0">
        <!-- Full-width image preview -->
        <div class="relative mt-4 w-full aspect-video bg-gray-100">
          {#if editVariantPreviewUrl}
            {#if isVideo({ url: editVariantPreviewUrl })}
              <video src={editVariantPreviewUrl} muted loop playsinline preload="metadata"
                     class="w-full h-full object-cover bg-black"></video>
            {:else}
              <ResponsiveImage src={editVariantPreviewUrl} alt=""
                               widths={VARIANT_PREVIEW_WIDTHS} sizes={VARIANT_PREVIEW_SIZES}
                               class="w-full h-full object-cover" />
            {/if}
            <button type="button"
                    onclick={() => { editVariantRemoveImage = true; editVariantImageId = null; }}
                    class="absolute bottom-2 right-2 p-1.5 rounded-lg bg-black/40 hover:bg-red-500/80 transition-colors text-white">
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
              </svg>
            </button>
          {:else}
            <div class="w-full h-full flex flex-col items-center justify-center gap-1.5 text-gray-400">
              <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
              </svg>
              <span class="text-xs font-medium">{m.admin_product_edit_edit_variant_no_image()}</span>
            </div>
          {/if}
        </div>
        <div class="p-6 pt-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_sku()} {m.admin_product_edit_required_marker()}</label>
            <input name="sku" required value={editingVariant.sku}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_name()}</label>
            <input name="name" value={editingVariant.name ?? ''}
                   placeholder={m.admin_product_edit_add_variant_name_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_price()} {m.admin_product_edit_required_marker()}</label>
            <input name="price" type="number" step="0.01" min="0" required value={editingVariant.price}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_compare_at()}</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   value={editingVariant.compare_at_price ?? ''}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_stock()}</label>
            <input name="stock_qty" type="number" min="0" value={editingVariant.stock_qty}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_low_stock_threshold()}</label>
            <input name="low_stock_threshold" type="number" min="0" step="1"
                   value={editingVariant.low_stock_threshold ?? ''}
                   placeholder={m.admin_product_edit_add_variant_low_stock_threshold_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_weight()}</label>
            <input name="weight_grams" type="number" min="0" step="1"
                   value={editingVariant.weight_grams ?? ''}
                   placeholder={m.admin_product_edit_add_variant_weight_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_dimensions()}</label>
            <div class="grid grid-cols-3 gap-2">
              <input name="length_mm" type="number" min="0" step="1"
                     value={editingVariant.length_mm ?? ''}
                     placeholder={m.admin_product_edit_add_variant_dim_l()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <input name="width_mm" type="number" min="0" step="1"
                     value={editingVariant.width_mm ?? ''}
                     placeholder={m.admin_product_edit_add_variant_dim_w()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <input name="height_mm" type="number" min="0" step="1"
                     value={editingVariant.height_mm ?? ''}
                     placeholder={m.admin_product_edit_add_variant_dim_h()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_label_status()}</label>
            <select name="is_active"
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              <option value="true" selected={editingVariant.is_active}>{m.admin_product_edit_status_active()}</option>
              <option value="false" selected={!editingVariant.is_active}>{m.admin_product_edit_status_inactive()}</option>
            </select>
          </div>
        </div>
        <!-- Image picker -->
        <div class="mt-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_variant_label_images()}</label>
          {#if imageMedia.length === 0}
            <p class="mt-2 text-xs text-gray-400">{m.admin_product_edit_add_variant_no_media()}</p>
          {:else}
            <div class="mt-2 flex gap-2 overflow-x-auto pb-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => { editVariantImageId = editVariantImageId === mf.id ? null : mf.id; editVariantRemoveImage = false; }}
                        style={mf.mime_type === 'link' ? 'display: none' : ''}
                        class="relative flex-none w-14 h-14 rounded-lg overflow-hidden border-2 transition-colors
                               {editVariantImageId === mf.id ? 'border-gray-900' : 'border-transparent'}">
                  {#if isVideo(mf)}
                    {#if isStreamingVideo(mf)}
                      <ResponsiveImage src={mf.thumbnail_url ?? ''} alt={mf.original_name}
                                       widths={[160, 320]} sizes="56px"
                                       class="w-full h-full object-cover bg-black" />
                    {:else}
                      <video src={mf.url} muted playsinline preload="metadata" class="w-full h-full object-cover bg-black"></video>
                    {/if}
                    <span class="absolute bottom-0.5 right-0.5 p-0.5 rounded bg-black/60 text-white" aria-hidden="true">
                      <svg class="w-2 h-2" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    </span>
                  {:else}
                    <ResponsiveImage src={mf.webp_url ?? mf.url} alt={mf.original_name}
                                     widths={[160, 320]} sizes="56px"
                                     class="w-full h-full object-cover"
                                     onload={mf.mime_type === 'link' ? (e) => { ((e.currentTarget as HTMLImageElement).parentElement as HTMLElement).style.display = ''; } : null}
                                     onerror={mf.mime_type === 'link' ? (e) => { ((e.currentTarget as HTMLImageElement).parentElement as HTMLElement).style.display = 'none'; } : null} />
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="flex gap-3 mt-5">
          <SaveButton loading={updatingVariant}
                  class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900
                         text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
            {m.admin_product_edit_edit_variant_submit()}
          </SaveButton>
          <button type="button" onclick={() => editingVariant = null}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            {m.admin_product_edit_edit_variant_cancel()}
          </button>
        </div>
        </div>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- ── Stock Adjust Modal ── -->
{#if showStockModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showStockModal = null} role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="font-semibold text-gray-900 mb-1">{m.admin_product_edit_stock_title()}</h3>
      <p class="text-sm text-gray-500 mb-4">
        {m.admin_product_edit_stock_subtitle_pre({ sku: showStockModal.sku })}<strong>{showStockModal.stock_qty}</strong>
      </p>
      <form method="POST" action="?/adjustStock"
            use:enhance={({ formData }) => {
              if (adjustingStock) return;
              adjustingStock = true;
              const sku = showStockModal?.sku ?? '';
              const delta = formData.get('delta')?.toString() ?? '0';
              return async ({ result, update }) => {
                const signed = parseInt(delta, 10) >= 0 ? `+${delta}` : delta;
                showResult(result, m.admin_product_edit_stock_adjusted_success({ signed, sku }), m.admin_product_edit_stock_adjusted_failure({ sku }));
                await update();
                adjustingStock = false;
                if (result.type === 'success') showStockModal = null;
              };
            }}>
        <input type="hidden" name="variant_id" value={showStockModal.id} />
        <div class="flex flex-col gap-1.5 mb-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_stock_label_delta()}</label>
          <p class="text-xs text-gray-400 mb-1">{m.admin_product_edit_stock_hint()}</p>
          <input name="delta" type="number" required value="0"
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex gap-3">
          <SaveButton loading={adjustingStock}
                  class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900
                         text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
            {m.admin_product_edit_stock_apply()}
          </SaveButton>
          <button type="button" onclick={() => showStockModal = null}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            {m.admin_product_edit_stock_cancel()}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- ── Add Image Modal ── -->
{#if showAddImage}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={resetAddImageModal}
         role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-2xl">
      <h3 class="font-semibold text-gray-900 mb-4">{m.admin_product_edit_add_media_title()}</h3>

      <!-- Tabs -->
      <div class="flex gap-1 mb-5 border-b border-gray-100">
        <button type="button" onclick={() => addImageTab = 'upload'}
                class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                       {addImageTab === 'upload'
                         ? 'border-gray-900 text-gray-900'
                         : 'border-transparent text-gray-400 hover:text-gray-700'}">
          {m.admin_product_edit_add_media_tab_upload()}
        </button>
        <button type="button" onclick={() => addImageTab = 'library'}
                class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                       {addImageTab === 'library'
                         ? 'border-gray-900 text-gray-900'
                         : 'border-transparent text-gray-400 hover:text-gray-700'}">
          {m.admin_product_edit_add_media_tab_library()}
        </button>
      </div>

      {#if addImageTab === 'upload'}
        <!-- Drag & drop zone -->
        <button type="button"
                ondragenter={onUploadDragEnter}
                ondragleave={onUploadDragLeave}
                ondragover={onUploadDragOver}
                ondrop={onUploadDrop}
                onclick={openFilePicker}
                class="w-full flex flex-col items-center justify-center gap-2 px-6 py-10
                       rounded-2xl border-2 border-dashed transition-colors text-center
                       {uploadDragOver
                         ? 'border-gray-900 bg-gray-50'
                         : 'border-gray-200 hover:border-gray-400 hover:bg-gray-50'}">
          <svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 7.5m0 0L7.5 12M12 7.5v9"/>
          </svg>
          <p class="text-sm font-medium text-gray-700">{m.admin_product_edit_add_media_dropzone()}</p>
          <p class="text-xs text-gray-400">{m.admin_product_edit_add_media_accepted()}</p>
        </button>

        <!-- Files list -->
        {#if uploadFiles.length > 0}
          <div class="mt-4 space-y-2 max-h-64 overflow-y-auto pr-1">
            {#each uploadFiles as f (f.id)}
              <div class="flex items-center gap-3 p-2 rounded-xl bg-gray-50">
                {#if f.file.type.startsWith('video/')}
                  <video src={f.preview} muted playsinline preload="metadata"
                         class="w-10 h-10 rounded-lg object-cover flex-shrink-0 bg-black"></video>
                {:else}
                  <img src={f.preview} alt="" class="w-10 h-10 rounded-lg object-cover flex-shrink-0" />
                {/if}
                <div class="flex-1 min-w-0">
                  <p class="text-sm text-gray-900 truncate">{f.file.name}</p>
                  {#if f.status === 'uploading'}
                    <div class="mt-1.5 h-1 w-full rounded-full bg-gray-200 overflow-hidden">
                      <div class="h-full bg-gray-900 transition-[width] duration-150"
                           style="width: {f.progress}%"></div>
                    </div>
                  {:else if f.status === 'error' && f.error}
                    <p class="text-xs text-red-500 truncate">{f.error}</p>
                  {:else}
                    <p class="text-xs text-gray-400">
                      {m.admin_product_edit_add_media_size_kb({ kb: (f.file.size / 1024).toFixed(0) })}
                    </p>
                  {/if}
                </div>
                <div class="flex-shrink-0 w-10 text-right">
                  {#if f.status === 'uploading'}
                    <span class="text-xs font-medium text-gray-500 tabular-nums">{f.progress}%</span>
                  {:else if f.status === 'success'}
                    <svg class="w-5 h-5 text-green-600 inline-block" fill="none" viewBox="0 0 24 24"
                         stroke="currentColor" stroke-width="2.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
                    </svg>
                  {:else}
                    <svg class="w-5 h-5 text-red-500 inline-block" fill="none" viewBox="0 0 24 24"
                         stroke="currentColor" stroke-width="2.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                    </svg>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}

        <div class="flex gap-3 mt-5">
          <button type="button" onclick={resetAddImageModal}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            {m.admin_product_edit_add_media_done()}
          </button>
        </div>
      {:else}
        <!-- Library tab — pick from existing media -->
        <form method="POST" action="?/addImage"
              use:enhance={({ formData, cancel }) => {
                if (attachingImage) { cancel(); return; }
                if (data.isNew) {
                  cancel();
                  if (!addImageSelectedId) return;
                  const mf = imageMedia.find(m => m.id === addImageSelectedId);
                  if (!mf) return;
                  const isFirst = pendingImages.length === 0;
                  pendingImages = [...pendingImages, {
                    _localId: crypto.randomUUID(),
                    media_file_id: addImageSelectedId,
                    preview_url: mf.webp_url ?? mf.url,
                    is_primary: isFirst,
                    alt_text: formData.get('alt_text')?.toString() || undefined
                  }];
                  resetAddImageModal();
                  return;
                }
                attachingImage = true;
                return async ({ result, update }) => {
                  showResult(result, m.admin_product_edit_add_media_added_success(), m.admin_product_edit_add_media_added_failure());
                  await update();
                  attachingImage = false;
                  if (result.type === 'success') resetAddImageModal();
                };
              }}>
          <input type="hidden" name="media_file_id" value={addImageSelectedId ?? ''} />
          <input type="hidden" name="sort_order" value="0" />

          {#if imageMedia.length === 0}
            <p class="text-sm text-gray-400 py-6 text-center">{m.admin_product_edit_add_media_no_media()}</p>
          {:else}
            <div class="grid grid-cols-4 sm:grid-cols-5 lg:grid-cols-6 gap-2 max-h-80 overflow-y-auto mb-4 pr-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => addImageSelectedId = addImageSelectedId === mf.id ? null : mf.id}
                        class="relative aspect-square rounded-xl overflow-hidden border-2 transition-colors
                               {addImageSelectedId === mf.id ? 'border-gray-900' : 'border-transparent hover:border-gray-300'}">
                  {#if isVideo(mf)}
                    {#if isStreamingVideo(mf)}
                      <ResponsiveImage src={mf.thumbnail_url ?? ''} alt={mf.original_name}
                                       widths={[160, 320]} sizes="56px"
                                       class="w-full h-full object-cover bg-black" />
                    {:else}
                      <video src={mf.url} muted playsinline preload="metadata" class="w-full h-full object-cover bg-black"></video>
                    {/if}
                    <span class="absolute bottom-1 right-1 p-0.5 rounded bg-black/60 text-white" aria-hidden="true">
                      <svg class="w-2.5 h-2.5" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M8 5v14l11-7z"/>
                      </svg>
                    </span>
                  {:else}
                    <ResponsiveImage src={mf.webp_url ?? mf.url} alt={mf.original_name}
                                     widths={[160, 320, 480]} sizes="120px"
                                     class="w-full h-full object-cover" />
                  {/if}
                  {#if addImageSelectedId === mf.id}
                    <div class="absolute inset-0 bg-gray-900/20 flex items-center justify-center">
                      <svg class="w-5 h-5 text-white drop-shadow" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 00-1.414 0L8 12.586 4.707 9.293a1 1 0 00-1.414 1.414l4 4a1 1 0 001.414 0l8-8a1 1 0 000-1.414z" clip-rule="evenodd"/>
                      </svg>
                    </div>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}

          <div class="flex flex-col gap-1.5 mb-5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_product_edit_add_media_label_alt()}</label>
            <input name="alt_text" placeholder={m.admin_product_edit_add_media_alt_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>

          <div class="flex gap-3">
            <SaveButton loading={attachingImage} disabled={!addImageSelectedId}
                    class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900
                           text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors
                           disabled:opacity-40 disabled:cursor-not-allowed">
              {m.admin_product_edit_add_media_submit()}
            </SaveButton>
            <button type="button" onclick={resetAddImageModal}
                    class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                           hover:border-gray-400 transition-colors">
              {m.admin_product_edit_add_media_cancel()}
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}

<!-- ── Confirm Delete Image Modal ── -->
{#if confirmDeleteImageId !== null}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => confirmDeleteImageId = null}
         role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <div class="flex items-center gap-3 mb-4">
        <div class="flex-none w-10 h-10 rounded-full bg-red-50 flex items-center justify-center">
          <svg class="w-5 h-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
          </svg>
        </div>
        <div>
          <h3 class="font-semibold text-gray-900">{m.admin_product_edit_delete_media_title()}</h3>
          <p class="text-sm text-gray-500">{m.admin_product_edit_delete_media_irreversible()}</p>
        </div>
      </div>
      <form method="POST" action="?/deleteImage"
            use:enhance={() => {
              if (deletingImage) return;
              deletingImage = true;
              return async ({ result, update }) => {
                showResult(result, m.admin_product_edit_delete_media_success(), m.admin_product_edit_delete_media_failure());
                confirmDeleteImageId = null;
                await update();
                deletingImage = false;
              };
            }}>
        <input type="hidden" name="image_id" value={confirmDeleteImageId} />
        <div class="flex gap-2 justify-end">
          <button type="button"
                  onclick={() => confirmDeleteImageId = null}
                  class="px-4 py-2 rounded-xl text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 transition-colors">
            {m.admin_product_edit_delete_media_cancel()}
          </button>
          <SaveButton loading={deletingImage}
                  class="inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium text-white bg-red-500 hover:bg-red-600 transition-colors disabled:opacity-50">
            {m.admin_product_edit_delete_media_confirm()}
          </SaveButton>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- ── Confirm Bundle → Simple Modal ── -->
{#if confirmKindChange}
  {@const componentCount = (data.bundleItems ?? []).length}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => { confirmKindChange = false; kind = 'bundle'; }}
         role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <div class="flex items-center gap-3 mb-4">
        <div class="flex-none w-10 h-10 rounded-full bg-red-50 flex items-center justify-center">
          <svg class="w-5 h-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"/>
          </svg>
        </div>
        <div>
          <h3 class="font-semibold text-gray-900">{m.admin_product_edit_kind_modal_title()}</h3>
          <p class="text-sm text-gray-500">{m.admin_product_edit_kind_modal_irreversible()}</p>
        </div>
      </div>
      <div class="text-sm text-gray-700 mb-5 space-y-2">
        <p>{m.admin_product_edit_kind_modal_intro_pre()}<span class="font-semibold">{m.admin_product_edit_kind_modal_intro_bundle()}</span>{m.admin_product_edit_kind_modal_intro_to()}<span class="font-semibold">{m.admin_product_edit_kind_modal_intro_simple()}</span>{m.admin_product_edit_kind_modal_intro_post()}</p>
        <ul class="list-disc list-inside text-gray-600 space-y-1">
          <li>{componentCount === 1 ? m.admin_product_edit_kind_modal_item_components_one({ count: componentCount }) : m.admin_product_edit_kind_modal_item_components_many({ count: componentCount })}</li>
          <li>{m.admin_product_edit_kind_modal_item_variant()}</li>
        </ul>
        <p class="text-gray-500 text-xs pt-1">{m.admin_product_edit_kind_modal_footer()}</p>
      </div>
      <div class="flex gap-2 justify-end">
        <button type="button"
                onclick={() => { confirmKindChange = false; kind = 'bundle'; }}
                class="px-4 py-2 rounded-xl text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 transition-colors">
          {m.admin_product_edit_kind_modal_cancel()}
        </button>
        <button type="button"
                onclick={() => {
                  kindChangeConfirmed = true;
                  confirmKindChange = false;
                  (document.getElementById('product-form') as HTMLFormElement | null)?.requestSubmit();
                }}
                class="inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-xl text-sm font-medium text-white bg-red-500 hover:bg-red-600 transition-colors">
          {m.admin_product_edit_kind_modal_confirm()}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* SortableJS visual classes for the media tile grid. */
  :global(.js-media-grid .gy-ghost) {
    background: #f3f4f6;
    outline: 2px dashed rgba(17, 24, 39, 0.4);
    outline-offset: -2px;
    border-radius: 0.75rem;
    opacity: 1;
  }
  :global(.js-media-grid .gy-ghost) > * {
    opacity: 0;
  }
  :global(.js-media-grid .gy-chosen) {
    opacity: 0.6;
  }
  :global(.js-media-grid .gy-drag) {
    opacity: 1 !important;
    border-radius: 0.75rem;
    box-shadow: 0 12px 32px -8px rgba(17, 24, 39, 0.35),
                0 4px 12px -2px rgba(17, 24, 39, 0.15);
    cursor: grabbing !important;
  }
</style>

<!-- Variant Stock History modal (P2 #23) -->
{#if historyVariant}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => historyVariant = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-2xl max-h-[80vh] flex flex-col">
      <div class="px-6 py-4 border-b border-gray-100 flex items-start justify-between">
        <div>
          <h3 class="text-base font-bold text-gray-900">{m.admin_product_edit_variants_history_title()}</h3>
          <p class="text-xs text-gray-500 mt-0.5">{historyVariant.sku} · {m.admin_product_edit_variants_history_current({ qty: historyVariant.stock_qty })}</p>
        </div>
        <button onclick={() => historyVariant = null}
                class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>
      <div class="flex-1 overflow-y-auto">
        {#if historyLoading}
          <p class="px-6 py-12 text-sm text-gray-400 text-center">{m.admin_product_edit_variants_history_loading()}</p>
        {:else if historyRows.length === 0}
          <p class="px-6 py-12 text-sm text-gray-400 text-center">{m.admin_product_edit_variants_history_empty()}</p>
        {:else}
          <table class="w-full text-xs">
            <thead>
              <tr class="border-b border-gray-50 sticky top-0 bg-white">
                <th class="text-left px-6 py-3 font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_history_col_time()}</th>
                <th class="text-left px-3 py-3 font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_history_col_change()}</th>
                <th class="text-left px-3 py-3 font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_history_col_reason()}</th>
                <th class="text-left px-3 py-3 font-semibold text-gray-400 uppercase tracking-wide">{m.admin_product_edit_variants_history_col_actor()}</th>
                <th class="px-6 py-3"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50">
              {#each historyRows as h}
                <tr>
                  <td class="px-6 py-2.5 text-gray-500 whitespace-nowrap">{new Date(h.created_at).toLocaleString()}</td>
                  <td class="px-3 py-2.5 font-mono">
                    <span class={h.delta > 0 ? 'text-emerald-700' : 'text-red-600'}>{h.delta > 0 ? '+' : ''}{h.delta}</span>
                    <span class="text-gray-400 ml-1">({h.before_qty}→{h.after_qty})</span>
                  </td>
                  <td class="px-3 py-2.5 text-gray-700">{fmtReason(h.reason)}</td>
                  <td class="px-3 py-2.5 text-gray-500">
                    {#if h.actor_email}
                      {h.actor_email}
                    {:else if h.order_id}
                      <span class="font-mono text-xs">order ·{h.order_id.slice(0, 8)}</span>
                    {:else}
                      —
                    {/if}
                  </td>
                  <td class="px-6 py-2.5"></td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>
  </div>
{/if}
