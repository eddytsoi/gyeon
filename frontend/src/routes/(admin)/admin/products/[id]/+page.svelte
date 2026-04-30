<script lang="ts">
  import { enhance } from '$app/forms';
  import { invalidateAll } from '$app/navigation';
  import { adminUploadMedia } from '$lib/api/admin';
  import type { PageData } from './$types';
  import { showResult, notify } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();

  function slugify(s: string) {
    return s.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
  }

  // Product form state
  let name = $state(data.product?.name ?? '');
  let slug = $state(data.product?.slug ?? '');
  let autoSlug = $state(!data.product);
  let saving = $state(false);

  $effect(() => {
    if (autoSlug) slug = slugify(name);
  });

  // Variant modal state
  let showAddVariant = $state(false);
  let editingVariant = $state<typeof data.variants[0] | null>(null);
  let showStockModal = $state<typeof data.variants[0] | null>(null);

  // Image media helper — uploaded images + all links (links are tested at render time via onload/onerror)
  function isImageMedia(f: { mime_type: string; url: string }) {
    return f.mime_type.startsWith('image/') || f.mime_type === 'link';
  }

  // Variant image picker state
  const imageMedia = $derived((data.mediaFiles ?? []).filter(isImageMedia));
  let addVariantImageId = $state<string | null>(null);
  let editVariantImageId = $state<string | null>(null);
  let editVariantOldImageId = $state<string | null>(null);
  let editVariantRemoveImage = $state(false);
  const editVariantPreviewUrl = $derived(
    editVariantRemoveImage ? null : (editVariantImageId
      ? (imageMedia.find(m => m.id === editVariantImageId)?.webp_url ?? imageMedia.find(m => m.id === editVariantImageId)?.url ?? null)
      : (editingVariant?.image_url ?? null))
  );

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
    error?: string;
  };
  let uploadFiles = $state<UploadFile[]>([]);
  let uploadDragOver = $state(false);
  const ACCEPTED_IMAGE = /^image\/(jpeg|png|webp|gif)$/;

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
    input.accept = 'image/jpeg,image/png,image/webp,image/gif';
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
    if (!data.product || !data.token) return;
    const token = data.token;
    const valid = files.filter((f) => ACCEPTED_IMAGE.test(f.type));
    const rejected = files.length - valid.length;
    if (rejected > 0) {
      notify.warning(
        `${rejected} file${rejected !== 1 ? 's' : ''} skipped`,
        'Only JPEG, PNG, WebP, or GIF images are accepted.'
      );
    }
    const newItems: UploadFile[] = valid.map((file) => ({
      id: crypto.randomUUID(),
      file,
      preview: URL.createObjectURL(file),
      status: 'uploading'
    }));
    uploadFiles = [...uploadFiles, ...newItems];
    let attached = 0;
    for (const item of newItems) {
      try {
        const media = await adminUploadMedia(token, item.file);
        const fd = new FormData();
        fd.set('media_file_id', media.id);
        fd.set('sort_order', '0');
        fd.set('is_primary', 'false');
        const res = await fetch('?/addImage', { method: 'POST', body: fd });
        if (!res.ok) throw new Error(`Failed to attach (${res.status})`);
        item.status = 'success';
        attached++;
      } catch (e) {
        item.status = 'error';
        item.error = e instanceof Error ? e.message : 'Upload failed';
      }
      uploadFiles = [...uploadFiles];
    }
    if (attached > 0) await invalidateAll();
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
  let dragSrcIdx = $state<number | null>(null);
  let dragOverIdx = $state<number | null>(null);
  let reorderSaving = $state(false);
  let reorderError = $state<string | null>(null);

  $effect(() => { images = sortedImages(data.images); });

  function handleDragStart(e: DragEvent, idx: number) {
    dragSrcIdx = idx;
    e.dataTransfer!.effectAllowed = 'move';
  }

  function handleDragOver(e: DragEvent, idx: number) {
    if (idx === 0) return;
    e.preventDefault();
    e.dataTransfer!.dropEffect = 'move';
    dragOverIdx = idx;
  }

  function handleDragLeave(e: DragEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
      dragOverIdx = null;
    }
  }

  function handleDrop(e: DragEvent, idx: number) {
    e.preventDefault();
    if (dragSrcIdx === null || dragSrcIdx === idx || idx === 0) {
      dragSrcIdx = null; dragOverIdx = null;
      return;
    }
    const reordered = [...images];
    const [moved] = reordered.splice(dragSrcIdx, 1);
    reordered.splice(idx, 0, moved);
    images = reordered;
    dragSrcIdx = null; dragOverIdx = null;
    persistReorder(reordered);
  }

  function handleDragEnd() {
    dragSrcIdx = null; dragOverIdx = null;
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
      reorderError = 'Failed to save image order.';
      images = snapshot;
    } finally {
      reorderSaving = false;
    }
  }
</script>

<svelte:head>
  <title>{data.isNew ? 'New Product' : (data.product?.name ?? 'Edit Product')} — Gyeon Admin</title>
</svelte:head>

<div class="max-w-4xl">
  <!-- Back + title -->
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/products" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">← Products</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">
      {data.isNew ? 'New Product' : (data.product?.name ?? 'Edit Product')}
    </h1>
  </div>

  <!-- ── Product Details ── -->
  <section class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">Product Details</h2>
    <form id="product-form" method="POST" action="?/saveProduct"
          use:enhance={() => {
            saving = true;
            const productName = name;
            return async ({ result, update }) => {
              showResult(result,
                data.isNew ? `Product '${productName}' created` : `Product '${productName}' saved`,
                data.isNew ? `Failed to create product '${productName}'` : `Failed to save product '${productName}'`);
              await update();
              saving = false;
            };
          }}>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="name" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Name *</label>
          <input id="name" name="name" required bind:value={name}
                 oninput={() => { autoSlug = true; }}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="slug" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Slug *</label>
          <input id="slug" name="slug" required bind:value={slug}
                 oninput={() => { autoSlug = false; }}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="category_id" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Category</label>
          <select id="category_id" name="category_id"
                  class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="">— None —</option>
            {#each data.categories as cat}
              <option value={cat.id} selected={data.product?.category_id === cat.id}>{cat.name}</option>
            {/each}
          </select>
        </div>
        {#if !data.isNew}
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</label>
            <select name="is_active"
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              <option value="true" selected={data.product?.is_active}>Active</option>
              <option value="false" selected={!data.product?.is_active}>Inactive</option>
            </select>
          </div>
        {/if}
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <label for="description" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Description</label>
          <textarea id="description" name="description" rows="4"
                    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900 resize-none"
                    >{data.product?.description ?? ''}</textarea>
        </div>
      </div>
    </form>
  </section>

  {#if !data.isNew}
    <!-- ── Variants ── -->
    <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
        <h2 class="font-semibold text-gray-900">Variants</h2>
        <button onclick={() => showAddVariant = true}
                class="px-3 py-1.5 bg-gray-900 text-white text-xs font-medium rounded-lg
                       hover:bg-gray-700 transition-colors">
          + Add Variant
        </button>
      </div>

      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-100">
          <tr>
            <th class="px-5 py-3 w-12"></th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">SKU</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Price</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">Compare at</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Stock</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden md:table-cell">Status</th>
            <th class="px-5 py-3"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.variants as variant}
            <tr class="hover:bg-gray-50 transition-colors">
              <td class="px-5 py-3">
                {#if variant.image_url}
                  <img src={variant.image_url} alt="" class="w-8 h-8 rounded object-cover" />
                {:else}
                  <div class="w-8 h-8 rounded bg-gray-100"></div>
                {/if}
              </td>
              <td class="px-5 py-3 font-mono text-xs text-gray-700">{variant.sku}</td>
              <td class="px-5 py-3 font-medium text-gray-900">HK${variant.price.toFixed(2)}</td>
              <td class="px-5 py-3 text-gray-400 hidden sm:table-cell">
                {variant.compare_at_price ? `HK$${variant.compare_at_price.toFixed(2)}` : '—'}
              </td>
              <td class="px-5 py-3">
                <span class="font-medium {variant.stock_qty <= 5 ? 'text-red-600' : 'text-gray-900'}">
                  {variant.stock_qty}
                </span>
              </td>
              <td class="px-5 py-3 hidden md:table-cell">
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                             {variant.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
                  {variant.is_active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td class="px-5 py-3 text-right">
                <div class="flex items-center justify-end gap-1">
                  <!-- Adjust Stock -->
                  <button onclick={() => showStockModal = variant}
                          title="Adjust stock"
                          aria-label="Adjust stock"
                          class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
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
                          title="Edit"
                          aria-label="Edit variant"
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
                            showResult(result, `Variant '${sku}' deleted`, `Failed to delete variant '${sku}'`);
                            await update();
                          };
                        }}>
                    <input type="hidden" name="variant_id" value={variant.id} />
                    <button type="submit"
                            title="Delete"
                            aria-label="Delete variant"
                            class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                            onclick={(e) => { if (!confirm('Delete this variant?')) e.preventDefault(); }}>
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
              <td colspan="7" class="px-5 py-8 text-center text-gray-400 text-sm">
                No variants yet. Add one to set pricing and stock.
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>

    <!-- ── Images ── -->
    <section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
        <h2 class="font-semibold text-gray-900">
          Images
          {#if reorderSaving}
            <span class="ml-2 text-xs font-normal text-gray-400">Saving order…</span>
          {/if}
        </h2>
        <button onclick={() => showAddImage = true}
                class="px-3 py-1.5 bg-gray-900 text-white text-xs font-medium rounded-lg
                       hover:bg-gray-700 transition-colors">
          + Add Image
        </button>
      </div>

      {#if reorderError}
        <div class="px-6 py-2 text-sm text-red-500 bg-red-50 border-b border-red-100 flex items-center justify-between">
          {reorderError}
          <button onclick={() => reorderError = null} class="ml-4 text-red-400 hover:text-red-600">✕</button>
        </div>
      {/if}

      {#if images.length === 0}
        <div class="px-6 py-8 text-center text-gray-400 text-sm">No images yet.</div>
      {:else}
        <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 p-6">
          {#each images as image, i}
            <div
              draggable={i !== 0}
              ondragstart={i !== 0 ? (e) => handleDragStart(e, i) : undefined}
              ondragover={(e) => handleDragOver(e, i)}
              ondragleave={handleDragLeave}
              ondrop={(e) => handleDrop(e, i)}
              ondragend={handleDragEnd}
              class="aspect-square rounded-xl overflow-hidden relative group bg-gray-100
                     transition-all duration-150
                     {i !== 0 ? 'cursor-grab' : ''}
                     {dragSrcIdx === i ? 'opacity-40 scale-95' :
                      dragOverIdx === i ? 'ring-2 ring-gray-900/40' : ''}"
            >
              <img src={image.url} alt={image.alt_text ?? ''} class="w-full h-full object-cover" />

              <!-- Always-visible primary indicator -->
              {#if image.is_primary}
                <span class="absolute top-2 left-2 p-1.5 rounded-lg bg-amber-400/90 text-white"
                      title="Primary image" aria-label="Primary image">
                  <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                  </svg>
                </span>
              {/if}

              <!-- Hover overlay with bottom-centered action icons -->
              <div class="absolute inset-0 bg-gray-900/70 opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-end justify-center p-2.5">
                <div class="flex items-center justify-center gap-1.5">
                  {#if !image.is_primary}
                    <form method="POST" action="?/setPrimary"
                          use:enhance={() => async ({ result, update }) => {
                            showResult(result, 'Primary image set', 'Failed to set primary image');
                            await update();
                          }}>
                      <input type="hidden" name="image_id" value={image.id} />
                      <input type="hidden" name="sort_order" value={image.sort_order} />
                      <button type="submit"
                              title="Set as primary" aria-label="Set as primary"
                              class="p-1.5 rounded-lg bg-white/10 hover:bg-amber-400/90 transition-colors text-white">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M11.48 3.499a.562.562 0 0 1 1.04 0l2.125 5.111a.563.563 0 0 0 .475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 0 0-.182.557l1.285 5.385a.562.562 0 0 1-.84.61l-4.725-2.885a.562.562 0 0 0-.586 0L6.982 20.54a.562.562 0 0 1-.84-.61l1.285-5.386a.562.562 0 0 0-.182-.557l-4.204-3.602a.562.562 0 0 1 .321-.988l5.518-.442a.563.563 0 0 0 .475-.345L11.48 3.5Z"/>
                        </svg>
                      </button>
                    </form>
                  {/if}
                  <form method="POST" action="?/deleteImage"
                        use:enhance={() => async ({ result, update }) => {
                          showResult(result, 'Image deleted', 'Failed to delete image');
                          await update();
                        }}>
                    <input type="hidden" name="image_id" value={image.id} />
                    <button type="submit"
                            title="Delete" aria-label="Delete image"
                            class="p-1.5 rounded-lg bg-white/10 hover:bg-red-500/80 transition-colors text-white"
                            onclick={(e) => { if (!confirm('Delete this image?')) e.preventDefault(); }}>
                      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                      </svg>
                    </button>
                  </form>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>
  {/if}

  <!-- ── Actions ── -->
  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5
              flex flex-col sm:flex-row sm:items-center gap-4">
    <div class="sm:ml-auto flex gap-3">
      <a href="/admin/products"
         class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                text-gray-700 hover:bg-gray-50 transition-colors">
        Cancel
      </a>
      <button type="submit" form="product-form" disabled={saving}
              class="px-5 py-2.5 rounded-xl bg-gray-900 text-white text-sm font-medium
                     hover:bg-gray-700 transition-colors disabled:opacity-50">
        {saving ? 'Saving…' : (data.isNew ? 'Create Product' : 'Save Changes')}
      </button>
    </div>
  </div>
</div>

<!-- ── Add Variant Modal ── -->
{#if showAddVariant}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showAddVariant = false} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <h3 class="font-semibold text-gray-900 mb-4">Add Variant</h3>
      <form method="POST" action="?/addVariant"
            use:enhance={({ formData }) => {
              const sku = formData.get('sku')?.toString() ?? '';
              return async ({ result, update }) => {
                showResult(result, `Variant '${sku}' added`, `Failed to add variant '${sku}'`);
                await update();
                if (result.type === 'success') { showAddVariant = false; addVariantImageId = null; }
              };
            }}>
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">SKU *</label>
            <input name="sku" required class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                   focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Price (HKD) *</label>
            <input name="price" type="number" step="0.01" min="0" required
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Compare at</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Stock Qty</label>
            <input name="stock_qty" type="number" min="0" value="0"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
        <!-- Image picker -->
        <div class="mt-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Images</label>
          <input type="hidden" name="image_media_file_id" value={addVariantImageId ?? ''} />
          {#if imageMedia.length === 0}
            <p class="mt-2 text-xs text-gray-400">No images in media library yet.</p>
          {:else}
            <div class="mt-2 flex gap-2 overflow-x-auto pb-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => addVariantImageId = addVariantImageId === mf.id ? null : mf.id}
                        style={mf.mime_type === 'link' ? 'display: none' : ''}
                        class="flex-none w-14 h-14 rounded-lg overflow-hidden border-2 transition-colors
                               {addVariantImageId === mf.id ? 'border-gray-900' : 'border-transparent'}">
                  <img src={mf.webp_url ?? mf.url} alt={mf.original_name} class="w-full h-full object-cover"
                       onload={mf.mime_type === 'link' ? (e) => { (e.currentTarget.parentElement as HTMLElement).style.display = ''; } : null}
                       onerror={mf.mime_type === 'link' ? (e) => { (e.currentTarget.parentElement as HTMLElement).style.display = 'none'; } : null} />
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="flex gap-3 mt-5">
          <button type="submit"
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            Add Variant
          </button>
          <button type="button" onclick={() => { showAddVariant = false; addVariantImageId = null; }}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
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
         onclick={() => editingVariant = null} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-md overflow-hidden">
      <div class="p-6 pb-0">
        <h3 class="font-semibold text-gray-900">Edit Variant</h3>
      </div>
      <form method="POST" action="?/updateVariant"
            use:enhance={({ formData }) => {
              const sku = formData.get('sku')?.toString() ?? '';
              return async ({ result, update }) => {
                showResult(result, `Variant '${sku}' saved`, `Failed to save variant '${sku}'`);
                await update();
                if (result.type === 'success') editingVariant = null;
              };
            }}>
        <input type="hidden" name="variant_id" value={editingVariant.id} />
        <input type="hidden" name="old_image_id" value={editVariantOldImageId ?? ''} />
        <input type="hidden" name="image_media_file_id" value={editVariantRemoveImage ? '' : (editVariantImageId ?? '')} />
        <input type="hidden" name="remove_image" value={String(editVariantRemoveImage)} />
        <!-- Full-width image preview -->
        <div class="relative mt-4 w-full aspect-video bg-gray-100">
          {#if editVariantPreviewUrl}
            <img src={editVariantPreviewUrl} alt="" class="w-full h-full object-cover" />
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
              <span class="text-xs font-medium">No image</span>
            </div>
          {/if}
        </div>
        <div class="p-6 pt-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">SKU *</label>
            <input name="sku" required value={editingVariant.sku}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Price (HKD) *</label>
            <input name="price" type="number" step="0.01" min="0" required value={editingVariant.price}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Compare at</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   value={editingVariant.compare_at_price ?? ''}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Stock Qty</label>
            <input name="stock_qty" type="number" min="0" value={editingVariant.stock_qty}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</label>
            <select name="is_active"
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              <option value="true" selected={editingVariant.is_active}>Active</option>
              <option value="false" selected={!editingVariant.is_active}>Inactive</option>
            </select>
          </div>
        </div>
        <!-- Image picker -->
        <div class="mt-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Images</label>
          {#if imageMedia.length === 0}
            <p class="mt-2 text-xs text-gray-400">No images in media library yet.</p>
          {:else}
            <div class="mt-2 flex gap-2 overflow-x-auto pb-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => { editVariantImageId = editVariantImageId === mf.id ? null : mf.id; editVariantRemoveImage = false; }}
                        style={mf.mime_type === 'link' ? 'display: none' : ''}
                        class="flex-none w-14 h-14 rounded-lg overflow-hidden border-2 transition-colors
                               {editVariantImageId === mf.id ? 'border-gray-900' : 'border-transparent'}">
                  <img src={mf.webp_url ?? mf.url} alt={mf.original_name} class="w-full h-full object-cover"
                       onload={mf.mime_type === 'link' ? (e) => { (e.currentTarget.parentElement as HTMLElement).style.display = ''; } : null}
                       onerror={mf.mime_type === 'link' ? (e) => { (e.currentTarget.parentElement as HTMLElement).style.display = 'none'; } : null} />
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="flex gap-3 mt-5">
          <button type="submit"
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            Save Changes
          </button>
          <button type="button" onclick={() => editingVariant = null}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
          </button>
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
         onclick={() => showStockModal = null} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="font-semibold text-gray-900 mb-1">Adjust Stock</h3>
      <p class="text-sm text-gray-500 mb-4">
        {showStockModal.sku} — current stock: <strong>{showStockModal.stock_qty}</strong>
      </p>
      <form method="POST" action="?/adjustStock"
            use:enhance={({ formData }) => {
              const sku = showStockModal?.sku ?? '';
              const delta = formData.get('delta')?.toString() ?? '0';
              return async ({ result, update }) => {
                const signed = parseInt(delta, 10) >= 0 ? `+${delta}` : delta;
                showResult(result, `Stock adjusted ${signed} for '${sku}'`, `Failed to adjust stock for '${sku}'`);
                await update();
                if (result.type === 'success') showStockModal = null;
              };
            }}>
        <input type="hidden" name="variant_id" value={showStockModal.id} />
        <div class="flex flex-col gap-1.5 mb-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Delta</label>
          <p class="text-xs text-gray-400 mb-1">Positive to add stock, negative to remove.</p>
          <input name="delta" type="number" required value="0"
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex gap-3">
          <button type="submit"
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            Apply
          </button>
          <button type="button" onclick={() => showStockModal = null}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
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
         role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-2xl">
      <h3 class="font-semibold text-gray-900 mb-4">Add Image</h3>

      <!-- Tabs -->
      <div class="flex gap-1 mb-5 border-b border-gray-100">
        <button type="button" onclick={() => addImageTab = 'upload'}
                class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                       {addImageTab === 'upload'
                         ? 'border-gray-900 text-gray-900'
                         : 'border-transparent text-gray-400 hover:text-gray-700'}">
          Upload
        </button>
        <button type="button" onclick={() => addImageTab = 'library'}
                class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                       {addImageTab === 'library'
                         ? 'border-gray-900 text-gray-900'
                         : 'border-transparent text-gray-400 hover:text-gray-700'}">
          Library
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
          <p class="text-sm font-medium text-gray-700">Drag & drop images here, or click to browse</p>
          <p class="text-xs text-gray-400">JPEG, PNG, WebP, or GIF</p>
        </button>

        <!-- Files list -->
        {#if uploadFiles.length > 0}
          <div class="mt-4 space-y-2 max-h-64 overflow-y-auto pr-1">
            {#each uploadFiles as f (f.id)}
              <div class="flex items-center gap-3 p-2 rounded-xl bg-gray-50">
                <img src={f.preview} alt="" class="w-10 h-10 rounded-lg object-cover flex-shrink-0" />
                <div class="flex-1 min-w-0">
                  <p class="text-sm text-gray-900 truncate">{f.file.name}</p>
                  {#if f.status === 'error' && f.error}
                    <p class="text-xs text-red-500 truncate">{f.error}</p>
                  {:else}
                    <p class="text-xs text-gray-400">
                      {(f.file.size / 1024).toFixed(0)} KB
                    </p>
                  {/if}
                </div>
                <div class="flex-shrink-0">
                  {#if f.status === 'uploading'}
                    <svg class="w-5 h-5 text-gray-400 animate-spin" viewBox="0 0 24 24" fill="none">
                      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" opacity="0.25"/>
                      <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="3"
                            stroke-linecap="round"/>
                    </svg>
                  {:else if f.status === 'success'}
                    <svg class="w-5 h-5 text-green-600" fill="none" viewBox="0 0 24 24"
                         stroke="currentColor" stroke-width="2.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
                    </svg>
                  {:else}
                    <svg class="w-5 h-5 text-red-500" fill="none" viewBox="0 0 24 24"
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
            Done
          </button>
        </div>
      {:else}
        <!-- Library tab — pick from existing media -->
        <form method="POST" action="?/addImage"
              use:enhance={() => async ({ result, update }) => {
                showResult(result, 'Image added', 'Failed to add image');
                await update();
                if (result.type === 'success') resetAddImageModal();
              }}>
          <input type="hidden" name="media_file_id" value={addImageSelectedId ?? ''} />
          <input type="hidden" name="sort_order" value="0" />

          {#if imageMedia.length === 0}
            <p class="text-sm text-gray-400 py-6 text-center">No images in media library yet.</p>
          {:else}
            <div class="grid grid-cols-4 sm:grid-cols-5 lg:grid-cols-6 gap-2 max-h-80 overflow-y-auto mb-4 pr-1">
              {#each imageMedia as mf}
                <button type="button"
                        onclick={() => addImageSelectedId = addImageSelectedId === mf.id ? null : mf.id}
                        class="relative aspect-square rounded-xl overflow-hidden border-2 transition-colors
                               {addImageSelectedId === mf.id ? 'border-gray-900' : 'border-transparent hover:border-gray-300'}">
                  <img src={mf.webp_url ?? mf.url} alt={mf.original_name} class="w-full h-full object-cover" />
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

          <div class="flex items-center gap-4 mb-5">
            <div class="flex flex-col gap-1.5">
              <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Set as Primary?</label>
              <select name="is_primary"
                      class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                             focus:outline-none focus:ring-2 focus:ring-gray-900">
                <option value="false">No</option>
                <option value="true">Yes</option>
              </select>
            </div>
            <div class="flex-1 flex flex-col gap-1.5">
              <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Alt Text</label>
              <input name="alt_text" placeholder="Optional"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>

          <div class="flex gap-3">
            <button type="submit" disabled={!addImageSelectedId}
                    class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                           hover:bg-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed">
              Add Image
            </button>
            <button type="button" onclick={resetAddImageModal}
                    class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                           hover:border-gray-400 transition-colors">
              Cancel
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}
