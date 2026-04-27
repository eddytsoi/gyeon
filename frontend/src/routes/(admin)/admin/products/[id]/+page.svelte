<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';

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

  // Add Image modal state
  let showAddImage = $state(false);
  let addImageSelectedId = $state<string | null>(null);

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
    <form method="POST" action="?/saveProduct"
          use:enhance={() => {
            saving = true;
            return async ({ update }) => { await update(); saving = false; };
          }}>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="name" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Name *</label>
          <input id="name" name="name" required bind:value={name}
                 oninput={() => { autoSlug = true; }}
                 class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="slug" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Slug *</label>
          <input id="slug" name="slug" required bind:value={slug}
                 oninput={() => { autoSlug = false; }}
                 class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                           focus:outline-none focus:ring-2 focus:ring-gray-900 resize-none"
                    >{data.product?.description ?? ''}</textarea>
        </div>
      </div>
      <div class="flex gap-3 mt-5">
        <button type="submit" disabled={saving}
                class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                       hover:bg-gray-700 transition-colors disabled:opacity-50">
          {saving ? 'Saving…' : (data.isNew ? 'Create Product' : 'Save Changes')}
        </button>
        <a href="/admin/products"
           class="px-5 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                  hover:border-gray-400 transition-colors">
          Cancel
        </a>
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
                <div class="flex items-center justify-end gap-3">
                  <button onclick={() => showStockModal = variant}
                          class="text-xs text-gray-400 hover:text-gray-700 transition-colors">
                    Stock ±
                  </button>
                  <button onclick={() => {
                            editingVariant = variant;
                            const cur = data.images.find(img => img.variant_id === variant.id);
                            editVariantOldImageId = cur?.id ?? null;
                            editVariantImageId = cur?.media_file_id ?? null;
                            editVariantRemoveImage = false;
                          }}
                          class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                    Edit
                  </button>
                  <form method="POST" action="?/deleteVariant" use:enhance>
                    <input type="hidden" name="variant_id" value={variant.id} />
                    <button type="submit"
                            class="text-xs text-red-400 hover:text-red-600 transition-colors"
                            onclick={(e) => { if (!confirm('Delete this variant?')) e.preventDefault(); }}>
                      Delete
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
              class="relative group rounded-xl overflow-hidden border bg-gray-50 aspect-square
                     transition-all duration-150
                     {i !== 0 ? 'cursor-grab' : ''}
                     {dragSrcIdx === i ? 'opacity-40 scale-95 border-gray-300' :
                      dragOverIdx === i ? 'border-2 border-gray-900 ring-2 ring-gray-900/20' :
                      'border-gray-100'}"
            >
              <img src={image.url} alt={image.alt_text ?? ''} class="w-full h-full object-cover" />
              {#if image.is_primary}
                <span class="absolute top-2 left-2 bg-gray-900 text-white text-[10px] font-medium px-1.5 py-0.5 rounded">
                  Primary
                </span>
              {/if}
              <div class="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity
                          flex flex-col items-center justify-center gap-2">
                {#if !image.is_primary}
                  <form method="POST" action="?/setPrimary" use:enhance>
                    <input type="hidden" name="image_id" value={image.id} />
                    <input type="hidden" name="sort_order" value={image.sort_order} />
                    <button type="submit"
                            class="px-3 py-1 bg-white text-gray-900 text-xs font-medium rounded-lg">
                      Set Primary
                    </button>
                  </form>
                {/if}
                <form method="POST" action="?/deleteImage" use:enhance>
                  <input type="hidden" name="image_id" value={image.id} />
                  <button type="submit"
                          class="px-3 py-1 bg-red-500 text-white text-xs font-medium rounded-lg"
                          onclick={(e) => { if (!confirm('Delete this image?')) e.preventDefault(); }}>
                    Delete
                  </button>
                </form>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
</div>

<!-- ── Add Variant Modal ── -->
{#if showAddVariant}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showAddVariant = false} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <h3 class="font-semibold text-gray-900 mb-4">Add Variant</h3>
      <form method="POST" action="?/addVariant"
            use:enhance={() => async ({ update }) => { await update(); showAddVariant = false; addVariantImageId = null; }}>
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">SKU *</label>
            <input name="sku" required class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                   focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Price (HKD) *</label>
            <input name="price" type="number" step="0.01" min="0" required
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Compare at</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Stock Qty</label>
            <input name="stock_qty" type="number" min="0" value="0"
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <h3 class="font-semibold text-gray-900 mb-4">Edit Variant</h3>
      <form method="POST" action="?/updateVariant"
            use:enhance={() => async ({ update }) => { await update(); editingVariant = null; }}>
        <input type="hidden" name="variant_id" value={editingVariant.id} />
        <input type="hidden" name="old_image_id" value={editVariantOldImageId ?? ''} />
        <input type="hidden" name="image_media_file_id" value={editVariantRemoveImage ? '' : (editVariantImageId ?? '')} />
        <input type="hidden" name="remove_image" value={String(editVariantRemoveImage)} />
        <div class="grid grid-cols-2 gap-4">
          <div class="col-span-2 flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">SKU *</label>
            <input name="sku" required value={editingVariant.sku}
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900 font-mono" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Price (HKD) *</label>
            <input name="price" type="number" step="0.01" min="0" required value={editingVariant.price}
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Compare at</label>
            <input name="compare_at_price" type="number" step="0.01" min="0"
                   value={editingVariant.compare_at_price ?? ''}
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Stock Qty</label>
            <input name="stock_qty" type="number" min="0" value={editingVariant.stock_qty}
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
          {#if !editVariantRemoveImage && (editingVariant.image_url || editVariantImageId)}
            {@const previewUrl = editVariantImageId
              ? (imageMedia.find(m => m.id === editVariantImageId)?.webp_url ?? imageMedia.find(m => m.id === editVariantImageId)?.url)
              : editingVariant.image_url}
            {#if previewUrl}
              <div class="mt-2 relative w-14 h-14">
                <img src={previewUrl} alt="" class="w-full h-full rounded-lg object-cover border border-gray-200" />
                <button type="button"
                        onclick={() => { editVariantRemoveImage = true; editVariantImageId = null; }}
                        class="absolute bottom-1 right-1 p-1 rounded-md bg-black/40 hover:bg-red-500/80 transition-colors text-white">
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                  </svg>
                </button>
              </div>
            {/if}
          {/if}
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
            use:enhance={() => async ({ update }) => { await update(); showStockModal = null; }}>
        <input type="hidden" name="variant_id" value={showStockModal.id} />
        <div class="flex flex-col gap-1.5 mb-4">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Delta</label>
          <p class="text-xs text-gray-400 mb-1">Positive to add stock, negative to remove.</p>
          <input name="delta" type="number" required value="0"
                 class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
         onclick={() => { showAddImage = false; addImageSelectedId = null; }}
         role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-2xl">
      <h3 class="font-semibold text-gray-900 mb-4">Add Image</h3>
      <form method="POST" action="?/addImage"
            use:enhance={() => async ({ update }) => { await update(); showAddImage = false; addImageSelectedId = null; }}>
        <input type="hidden" name="media_file_id" value={addImageSelectedId ?? ''} />
        <input type="hidden" name="sort_order" value="0" />

        <!-- Media pick list -->
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

        <!-- Options row -->
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
                   class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>

        <div class="flex gap-3">
          <button type="submit" disabled={!addImageSelectedId}
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed">
            Add Image
          </button>
          <button type="button" onclick={() => { showAddImage = false; addImageSelectedId = null; }}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
