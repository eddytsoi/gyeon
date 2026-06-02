<script lang="ts">
  /*
   * Admin editor for a product's WooCommerce up-sells OR cross-sells. Mirrors
   * the 套裝內容 (bundle contents) editor: a ProductPicker with variant pills,
   * a `name,variant` CSV importer, and drag-reorder — but each entry may be a
   * 單品 (simple) OR a 套裝 (bundle) product, and pins a specific variant.
   *
   * The selected (product, variant) refs are emitted as a JSON hidden input
   * (name={hiddenName}) so the parent <form>'s ?/saveProduct action persists
   * them via adminSetUpsells / adminSetCrossSells. The same product may appear
   * more than once with different variants; only a self-reference (the product
   * being edited) is excluded from the picker.
   */
  import type { RelatedProductRef } from '$lib/types';
  import { adminResolveProductRefsCSV } from '$lib/api/admin';
  import type { ProductRefCSVResolveItem } from '$lib/api/admin';
  import ProductPicker from '$lib/components/admin/ProductPicker.svelte';
  import type { ProductPickerAddPayload } from '$lib/components/admin/ProductPicker.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import { sortable } from '$lib/actions/sortable';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  const TINY_THUMB_WIDTHS = [160];
  const TINY_THUMB_SIZES = '40px';

  let {
    token,
    productId,
    items,
    hiddenName,
    formId,
    heading,
    subtitle,
    addHeading,
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
    emptyText: string;
  } = $props();

  type EditableRef = {
    _localId: string;
    product_id: string;
    variant_id: string | null;
    kind: string;
    name: string;
    slug: string;
    variant_name?: string | null;
    price?: number | null;
    compare_at_price?: number | null;
    stock_qty?: number | null;
    primary_image_url?: string | null;
  };

  // A stable per-row key. Uniqueness is at the (product, variant) level, so this
  // is unique even when the same product appears twice with different variants.
  const localId = (productID: string, variantID: string | null) =>
    `${productID}:${variantID ?? ''}`;

  function seed(rows: RelatedProductRef[]): EditableRef[] {
    return (rows ?? []).map((r) => ({
      _localId: localId(r.product_id, r.variant_id ?? null),
      product_id: r.product_id,
      variant_id: r.variant_id ?? null,
      kind: r.kind,
      name: r.name,
      slug: r.slug,
      variant_name: r.variant_name,
      price: r.price,
      compare_at_price: r.compare_at_price,
      stock_qty: r.stock_qty,
      primary_image_url: r.primary_image_url
    }));
  }

  let refs = $state<EditableRef[]>(seed(items));
  // Re-seed from the freshly-saved server state after a reload (invalidateAll).
  $effect(() => {
    refs = seed(items);
  });

  const refsJson = $derived(
    JSON.stringify(refs.map((r) => ({ product_id: r.product_id, variant_id: r.variant_id })))
  );

  // Add a resolved row; rejects an exact (product, variant) duplicate.
  function addRow(row: Omit<EditableRef, '_localId'>): boolean {
    const lid = localId(row.product_id, row.variant_id);
    if (refs.some((r) => r._localId === lid)) {
      notify.warning(
        m.admin_product_edit_bundle_duplicate_title(),
        m.admin_product_edit_bundle_duplicate_body({ sku: row.variant_name || row.name })
      );
      return false;
    }
    refs = [...refs, { _localId: lid, ...row }];
    return true;
  }

  function addFromPicker(p: ProductPickerAddPayload) {
    addRow({
      product_id: p.variant.product_id,
      variant_id: p.variant.id,
      kind: p.productKind,
      name: p.productName,
      slug: p.productSlug,
      variant_name: p.productKind === 'bundle' ? null : (p.variant.name ?? null),
      price: p.variant.price,
      compare_at_price: p.variant.compare_at_price ?? null,
      stock_qty: p.variant.stock_qty,
      primary_image_url: p.variant.image_url ?? p.primaryImageUrl ?? null
    });
  }

  function addFromCSV(it: ProductRefCSVResolveItem): boolean {
    return addRow({
      product_id: it.product_id,
      variant_id: it.variant_id,
      kind: it.kind,
      name: it.name,
      slug: it.slug,
      variant_name: it.kind === 'bundle' ? null : (it.variant_name ?? null),
      price: it.price,
      compare_at_price: it.compare_at_price,
      stock_qty: it.stock_qty,
      primary_image_url: it.primary_image_url
    });
  }

  // CSV import (name,variant per row)
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
      for (const it of result.items) { if (addFromCSV(it)) added++; }
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

  function removeRef(localIdValue: string) {
    refs = refs.filter((r) => r._localId !== localIdValue);
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
      <div class="min-w-0">
        <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{addHeading}</span>
        <p class="text-[11px] text-gray-400 mt-0.5">{m.admin_product_edit_related_csv_hint()}</p>
      </div>
      <button type="button" onclick={openCSVPicker} disabled={importing}
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 disabled:opacity-50 flex-shrink-0">
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
    <ProductPicker {token} mode="variant-only" kind="" showQuantity={false}
                   excludeProductIds={[productId]}
                   onAdd={addFromPicker} />
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
            <div class="text-sm text-gray-900 truncate flex items-center gap-2">
              <span class="truncate">{r.name}</span>
              {#if r.kind === 'bundle'}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-50 text-indigo-700 flex-shrink-0">
                  {m.admin_order_create_items_bundle_badge()}
                </span>
              {:else}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 text-gray-600 flex-shrink-0">
                  {m.admin_product_edit_related_simple_badge()}
                </span>
              {/if}
              {#if r.kind !== 'bundle' && r.variant_name}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-gray-900 text-white flex-shrink-0">
                  {r.variant_name}
                </span>
              {/if}
            </div>
            <div class="text-xs text-gray-400 font-mono truncate">{r.slug}</div>
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

  <input type="hidden" form={formId} name={hiddenName} value={refsJson} />
</section>
