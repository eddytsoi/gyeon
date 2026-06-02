<script lang="ts">
  /*
   * Admin editor for a product's WooCommerce up-sells OR cross-sells — a
   * product-to-product picker with search, single-column CSV import, and
   * drag-reorder. Mirrors the 優惠套裝 (promo-bundles) editor on the product
   * edit page, generalised so both relation types reuse one component.
   *
   * The selected target IDs are emitted as a comma-separated hidden input
   * (name={hiddenName}) so the parent <form>'s ?/saveProduct action persists
   * them via adminSetUpsells / adminSetCrossSells. Any product is a valid
   * target (no kind filter), so the search box is unfiltered.
   */
  import type { RelatedProductRef } from '$lib/types';
  import { adminGetVariants, adminGetImages, adminResolveProductRefsCSV } from '$lib/api/admin';
  import ProductSearchSelect from '$lib/components/admin/ProductSearchSelect.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import { sortable } from '$lib/actions/sortable';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  const TINY_THUMB_WIDTHS = [160];
  const TINY_THUMB_SIZES = '32px';

  let {
    token,
    productId,
    items,
    hiddenName,
    formId,
    heading,
    subtitle,
    addHeading,
    searchPlaceholder,
    emptyText
  }: {
    token: string;
    productId: string;
    items: RelatedProductRef[];
    hiddenName: string;
    // The id of the <form> to submit with. The section renders outside the
    // form's DOM subtree, so the hidden input associates via the `form`
    // attribute (same mechanism as the page's SaveButton).
    formId: string;
    heading: string;
    subtitle: string;
    addHeading: string;
    searchPlaceholder: string;
    emptyText: string;
  } = $props();

  type EditableRef = RelatedProductRef & { _localId: string };
  let refs = $state<EditableRef[]>((items ?? []).map((r) => ({ ...r, _localId: r.product_id })));
  // Re-seed from the freshly-saved server state after a reload (invalidateAll).
  $effect(() => {
    refs = (items ?? []).map((r) => ({ ...r, _localId: r.product_id }));
  });

  const idsCsv = $derived(refs.map((r) => r.product_id).join(','));
  // Hide from the search box: targets already added + the product itself.
  const excludedIds = $derived(
    [productId, ...refs.map((r) => r.product_id)].filter((x): x is string => !!x)
  );

  // A product to add — either an AdminProductRow (search pick) or a CSV-resolved
  // row (id/name/slug/status). Variant + image are hydrated on the fly because
  // the admin product list omits them.
  type Candidate = {
    id: string;
    name: string;
    slug: string;
    status: string;
    default_variant_id?: string | null;
    default_variant_price?: number | null;
    default_variant_compare_at_price?: number | null;
    default_variant_stock_qty?: number | null;
    primary_image_url?: string | null;
  };

  async function addCandidate(candidate: Candidate): Promise<boolean> {
    if (candidate.id === productId) return false;
    if (refs.some((r) => r.product_id === candidate.id)) return false;

    let variantId: string | null = candidate.default_variant_id ?? null;
    let price: number | null = candidate.default_variant_price ?? null;
    let compareAt: number | null = candidate.default_variant_compare_at_price ?? null;
    let stockQty: number | null = candidate.default_variant_stock_qty ?? null;
    let imageUrl: string | null = candidate.primary_image_url ?? null;

    if (token) {
      try {
        const [variants, images] = await Promise.all([
          adminGetVariants(token, candidate.id).catch(() => []),
          adminGetImages(token, candidate.id).catch(() => [])
        ]);
        const dv = variants[0];
        if (dv) {
          variantId = dv.id;
          price = dv.price;
          compareAt = dv.compare_at_price ?? null;
          stockQty = dv.stock_qty;
        }
        const primary = images.find((img) => img.is_primary) ?? images[0];
        if (primary?.url) imageUrl = primary.url;
      } catch { /* non-fatal — keep candidate fallbacks */ }
    }

    refs = [...refs, {
      _localId: crypto.randomUUID(),
      product_id: candidate.id,
      position: refs.length,
      slug: candidate.slug,
      name: candidate.name,
      status: candidate.status,
      variant_id: variantId,
      price,
      compare_at_price: compareAt,
      stock_qty: stockQty,
      primary_image_url: imageUrl
    }];
    return true;
  }

  // CSV import (one product name or slug per row)
  let csvInputEl = $state<HTMLInputElement | null>(null);
  let importing = $state(false);
  let importErrors = $state<{ row: number; message: string }[] | undefined>(undefined);

  function openCSVPicker() {
    importErrors = undefined;
    csvInputEl?.click();
  }

  async function onCSVPicked() {
    const file = csvInputEl?.files?.[0];
    if (!file || !token) return;
    importing = true;
    try {
      const result = await adminResolveProductRefsCSV(token, file);
      importErrors = result.errors;
      let added = 0;
      for (const it of result.items) { if (await addCandidate(it)) added++; }
      if (added > 0) {
        if (result.skipped > 0) {
          notify.error(m.admin_order_create_items_import_partial({ ok: String(added), skip: String(result.skipped) }));
        } else {
          notify.success(m.admin_order_create_items_import_success({ n: String(added) }));
        }
      } else {
        notify.error(m.admin_order_create_items_import_zero_rows({ skip: String(result.skipped) }));
      }
    } catch (e) {
      notify.error(m.admin_order_create_items_import_failure(), e instanceof Error ? e.message : '');
    } finally {
      importing = false;
      if (csvInputEl) csvInputEl.value = '';
    }
  }

  function removeRef(localId: string) {
    refs = refs.filter((r) => r._localId !== localId);
  }

  function onReorder(orderedIds: string[]) {
    const byLocalId = new Map(refs.map((r) => [r._localId, r]));
    refs = orderedIds
      .map((lid) => byLocalId.get(lid))
      .filter((r): r is EditableRef => !!r);
  }
</script>

<section class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
  <div class="px-6 py-4 border-b border-gray-100">
    <h2 class="font-semibold text-gray-900">{heading}</h2>
    <p class="text-xs text-gray-400 mt-1">{subtitle}</p>
  </div>

  <div class="px-6 py-4 border-b border-gray-100">
    <div class="flex items-center justify-between mb-2">
      <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{addHeading}</span>
      <button type="button" onclick={openCSVPicker} disabled={importing}
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 disabled:opacity-50">
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
        </svg>
        {m.admin_order_create_items_import()}
      </button>
    </div>
    <input type="file" accept=".csv,text/csv" class="hidden" bind:this={csvInputEl} onchange={onCSVPicked} />
    {#if importErrors && importErrors.length > 0}
      <div class="mb-3 bg-amber-50 border border-amber-200 rounded-xl p-3 text-xs text-amber-800 space-y-1">
        <div class="font-medium">{m.admin_order_create_items_import_errors_heading({ n: String(importErrors.length) })}</div>
        <ul class="list-disc pl-5 space-y-0.5">
          {#each importErrors.slice(0, 20) as e}
            <li>{m.admin_product_edit_import_error_row({ row: String(e.row), message: e.message })}</li>
          {/each}
          {#if importErrors.length > 20}
            <li>{m.admin_product_edit_import_error_more({ n: String(importErrors.length - 20) })}</li>
          {/if}
        </ul>
      </div>
    {/if}
    <ProductSearchSelect {token} excludeIds={excludedIds}
                         onSelect={addCandidate}
                         placeholder={searchPlaceholder} />
  </div>

  {#if refs.length === 0}
    <div class="px-6 py-6 text-sm text-gray-400">{emptyText}</div>
  {:else}
    <ul class="divide-y divide-gray-50"
        use:sortable={{
          onReorder,
          handle: '[data-drag-handle]',
          filter: 'button, form, input, a, [role="button"]'
        }}>
      {#each refs as r (r._localId)}
        <li class="flex items-center gap-3 px-6 py-3" data-id={r._localId}>
          <span class="text-gray-300 cursor-grab active:cursor-grabbing select-none" data-drag-handle aria-hidden="true">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9h16.5m-16.5 6.75h16.5"/>
            </svg>
          </span>
          {#if r.primary_image_url}
            <ResponsiveImage src={r.primary_image_url} alt="" widths={TINY_THUMB_WIDTHS} sizes={TINY_THUMB_SIZES}
                             class="w-10 h-10 rounded object-cover bg-gray-100" />
          {:else}
            <div class="w-10 h-10 rounded bg-gray-100"></div>
          {/if}
          <div class="flex-1 min-w-0">
            <div class="text-sm text-gray-900 truncate">{r.name}</div>
            <div class="text-xs text-gray-400 font-mono">{r.slug}</div>
          </div>
          {#if r.price != null}
            <div class="text-sm text-gray-900 whitespace-nowrap">
              {#if r.compare_at_price != null && r.compare_at_price > r.price}
                <span class="text-xs text-gray-400 line-through mr-1.5">HK${r.compare_at_price}</span>
              {/if}
              HK${r.price}
            </div>
          {/if}
          <button type="button" onclick={() => removeRef(r._localId)}
                  class="text-xs text-red-500 hover:text-red-700 px-2 py-1">
            {m.common_remove()}
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  <input type="hidden" form={formId} name={hiddenName} value={idsCsv} />
</section>
