<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import { adminGetProducts, adminGetVariants, adminGetBundleItems } from '$lib/api/admin';
  import type { AdminProductRow } from '$lib/api/admin';
  import type { Variant, BundleItem } from '$lib/types';

  export type ProductPickerAddPayload = {
    variant: Variant;
    productName: string;
    productSlug: string;
    productKind: 'simple' | 'bundle' | string;
    primaryImageUrl?: string | null;
    quantity: number;
    bundleItems: BundleItem[]; // only populated for bundles
  };

  // mode='order' (default) is the original add-to-order flow with prices,
  // bundle components, and OOS chips disabled.
  // mode='variant-only' is the Stock Management flow: hide prices, allow
  // selecting OOS variants (a stock-in mutation refills them), and skip the
  // bundle components UI since stock is tracked per concrete variant.
  // kind (optional) restricts the search to products of that kind ('simple' |
  // 'bundle'); empty searches all. Used by the product-detail bundle-contents
  // picker to surface only simple products as components.
  // showQuantity (default true) toggles the qty stepper — the up-sell /
  // cross-sell editors set it false (associations have no quantity).
  // excludeProductIds drops those products from the search results (e.g. the
  // product being edited, to prevent a self-reference).
  // includeInactive (default false) surfaces deactivated variants (and badges
  // inactive products) in the picker — opt-in for the Stock Management (進出單)
  // flow, which must adjust stock on variants that are temporarily disabled.
  let { token, onAdd, mode = 'order', kind = '', showQuantity = true, excludeProductIds = [], includeInactive = false }: {
    token: string;
    onAdd: (payload: ProductPickerAddPayload) => void;
    mode?: 'order' | 'variant-only';
    kind?: string;
    showQuantity?: boolean;
    excludeProductIds?: string[];
    includeInactive?: boolean;
  } = $props();

  let query = $state('');
  let results = $state<AdminProductRow[]>([]);
  let searching = $state(false);
  let searched = $state(false);

  // Expansion panel state — set when a search result is clicked
  let selectedProduct = $state<AdminProductRow | null>(null);
  let variants = $state<Variant[]>([]);
  let bundleItems = $state<BundleItem[]>([]);
  let selectedVariantId = $state<string | null>(null);
  let qty = $state(1);
  let loadingDetail = $state(false);

  let timer: ReturnType<typeof setTimeout> | undefined;
  function runSearch(q: string) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(async () => {
      const trimmed = q.trim();
      if (!trimmed) {
        results = [];
        searched = false;
        return;
      }
      searching = true;
      try {
        const res = await adminGetProducts(token, 8, 0, trimmed, '', kind);
        results = (res.items ?? []).filter((p) => !excludeProductIds.includes(p.id));
      } catch {
        results = [];
      } finally {
        searching = false;
        searched = true;
      }
    }, 300);
  }

  function onQueryInput(e: Event) {
    query = (e.currentTarget as HTMLInputElement).value;
    runSearch(query);
  }

  async function pickProduct(p: AdminProductRow) {
    selectedProduct = p;
    qty = 1;
    selectedVariantId = null;
    variants = [];
    bundleItems = [];
    loadingDetail = true;
    try {
      variants = await adminGetVariants(token, p.id);
      // Prefer the product's default variant; fall back to the first active
      // variant. With includeInactive (進出單), fall back further to the first
      // variant of any state so an all-inactive product still has a preselect.
      const active = variants.filter((v) => v.is_active);
      const pool = includeInactive ? variants : active;
      const preferred = active.find((v) => v.id === p.default_variant_id) ?? active[0] ?? pool[0] ?? null;
      selectedVariantId = preferred?.id ?? null;

      if (p.kind === 'bundle') {
        try {
          bundleItems = await adminGetBundleItems(token, p.id);
        } catch {
          bundleItems = [];
        }
      }
    } catch {
      variants = [];
    } finally {
      loadingDetail = false;
    }
  }

  function cancelPick() {
    selectedProduct = null;
    variants = [];
    bundleItems = [];
    selectedVariantId = null;
    qty = 1;
  }

  function decQty() { if (qty > 1) qty -= 1; }
  function incQty() { qty += 1; }
  function onQtyInput(e: Event) {
    const n = parseInt((e.currentTarget as HTMLInputElement).value, 10);
    qty = isNaN(n) || n < 1 ? 1 : n;
  }

  function handleAdd() {
    if (!selectedProduct || !selectedVariantId) return;
    const variant = variants.find((v) => v.id === selectedVariantId);
    if (!variant) return;
    onAdd({
      variant,
      productName: selectedProduct.name,
      productSlug: selectedProduct.slug,
      productKind: selectedProduct.kind ?? 'simple',
      primaryImageUrl: variant.image_url ?? selectedProduct.primary_image_url ?? null,
      quantity: qty,
      bundleItems: selectedProduct.kind === 'bundle' ? bundleItems : []
    });
    // Reset for the next add — admin can rapidly enter another product
    query = '';
    results = [];
    searched = false;
    cancelPick();
  }

  const selectedVariant = $derived(
    variants.find((v) => v.id === selectedVariantId) ?? null
  );

  // Variants offered in the chip picker. Inactive variants are hidden by
  // default; includeInactive (進出單) keeps them so stock can be adjusted.
  const pickerVariants = $derived(
    includeInactive ? variants : variants.filter((v) => v.is_active)
  );
</script>

<!-- Search input -->
<div class="relative">
  <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-400">
    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
      <path stroke-linecap="round" stroke-linejoin="round"
            d="m21 21-4.3-4.3M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z" />
    </svg>
  </span>
  <input
    type="search"
    value={query}
    oninput={onQueryInput}
    placeholder={m.admin_order_create_items_search_placeholder()}
    autocomplete="off"
    class="w-full pl-9 pr-3 py-2 text-sm rounded-xl border border-gray-200 bg-white
           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900
           placeholder:text-gray-400" />
</div>

<!-- Search dropdown — hidden once a result is selected (the panel takes over) -->
{#if !selectedProduct && query.trim() !== '' && (searching || results.length > 0 || searched)}
  <div class="mt-2 border border-gray-200 rounded-xl bg-white overflow-hidden">
    {#if searching && results.length === 0}
      <div class="px-3 py-2 text-xs text-gray-400">…</div>
    {:else if results.length === 0}
      <div class="px-3 py-2 text-xs text-gray-400">{m.admin_order_create_items_no_results()}</div>
    {:else}
      <ul class="divide-y divide-gray-100 max-h-80 overflow-y-auto">
        {#each results as p (p.id)}
          {@const price = p.default_variant_price ?? p.min_price ?? null}
          {@const compareAt = p.default_variant_compare_at_price ?? p.min_compare_at_price ?? null}
          <li>
            <button type="button" onclick={() => pickProduct(p)}
                    class="w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors flex items-center gap-3">
              {#if p.primary_image_url}
                <img src={p.primary_image_url} alt="" class="w-10 h-10 rounded-lg object-cover bg-gray-100 flex-shrink-0" />
              {:else}
                <div class="w-10 h-10 rounded-lg bg-gray-100 flex-shrink-0"></div>
              {/if}
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium text-gray-900 truncate flex items-center gap-2">
                  <span class="truncate">{p.name}</span>
                  {#if p.kind === 'bundle'}
                    <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-50 text-indigo-700">
                      {m.admin_order_create_items_bundle_badge()}
                    </span>
                  {/if}
                  {#if includeInactive && p.status === 'inactive'}
                    <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 text-gray-500">
                      {m.admin_products_status_inactive()}
                    </span>
                  {/if}
                </p>
                <p class="text-xs text-gray-500 truncate flex items-center gap-1.5 flex-wrap">
                  {#if mode === 'order'}
                    {#if price != null}
                      {#if compareAt != null && compareAt > price}
                        <span class="text-gray-400 line-through">HK${compareAt.toFixed(2)}</span>
                        <span class="text-red-600 font-medium">HK${price.toFixed(2)}</span>
                      {:else}
                        <span>HK${price.toFixed(2)}</span>
                      {/if}
                    {:else}
                      <span class="text-gray-400">—</span>
                    {/if}
                    {#if p.default_variant_stock_qty != null}
                      <span>·</span>
                      <span>{p.default_variant_stock_qty > 0 ? m.admin_order_create_items_stock({ qty: String(p.default_variant_stock_qty) }) : m.admin_order_create_items_out_of_stock()}</span>
                    {/if}
                  {:else if p.default_variant_stock_qty != null}
                    <span>{p.default_variant_stock_qty > 0 ? m.admin_order_create_items_stock({ qty: String(p.default_variant_stock_qty) }) : m.admin_order_create_items_out_of_stock()}</span>
                  {/if}
                </p>
              </div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<!-- Inline expansion panel -->
{#if selectedProduct}
  <div class="mt-3 border border-gray-200 rounded-xl bg-gray-50/50 p-4">
    <div class="flex items-start gap-3 mb-3">
      {#if selectedProduct.primary_image_url}
        <img src={selectedProduct.primary_image_url} alt="" class="w-12 h-12 rounded-lg object-cover bg-white flex-shrink-0" />
      {:else}
        <div class="w-12 h-12 rounded-lg bg-white flex-shrink-0"></div>
      {/if}
      <div class="flex-1 min-w-0">
        <p class="text-sm font-medium text-gray-900 flex items-center gap-2">
          <span class="truncate">{selectedProduct.name}</span>
          {#if selectedProduct.kind === 'bundle'}
            <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-50 text-indigo-700">
              {m.admin_order_create_items_bundle_badge()}
            </span>
          {/if}
        </p>
        {#if selectedVariant}
          <p class="text-xs text-gray-500 mt-0.5">
            {selectedVariant.sku}
            {#if mode === 'order'}· HK${selectedVariant.price.toFixed(2)}{/if}
            {#if mode === 'variant-only' || selectedProduct.kind !== 'bundle'}
              · {selectedVariant.stock_qty > 0 ? m.admin_order_create_items_stock({ qty: String(selectedVariant.stock_qty) }) : m.admin_order_create_items_out_of_stock()}
            {/if}
          </p>
        {/if}
      </div>
      <button type="button" onclick={cancelPick}
              aria-label="close"
              class="text-gray-400 hover:text-gray-700 transition-colors">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    {#if loadingDetail}
      <p class="text-xs text-gray-400">…</p>
    {:else if variants.length === 0}
      <p class="text-xs text-red-500">{m.admin_order_create_items_no_results()}</p>
    {:else}
      <!-- Variant picker — shown for any non-bundle product with at least
           one active variant. With a single variant the chip is purely
           informational (the variant name is otherwise hidden behind the
           SKU and admins lose track of which size/colour they're adding).
           Bundles never show the chip: their "variant" is a wrapper around
           the real stocked components, picked implicitly. -->
      {#if selectedProduct.kind !== 'bundle' && pickerVariants.length >= 1}
        <div class="mb-3">
          <p class="text-xs font-medium text-gray-600 mb-1.5">{m.admin_order_create_items_select_variant()}</p>
          <div class="flex flex-wrap gap-1.5">
            {#each pickerVariants as v (v.id)}
              {@const oos = v.stock_qty <= 0}
              {@const disableOOS = oos && mode === 'order'}
              <button type="button"
                      disabled={disableOOS}
                      onclick={() => { selectedVariantId = v.id; }}
                      class="px-3 py-1.5 rounded-full text-xs border transition-colors
                             {selectedVariantId === v.id
                               ? 'bg-gray-900 border-gray-900 text-white'
                               : 'border-gray-200 text-gray-700 hover:border-gray-400'}
                             {disableOOS ? 'opacity-50 cursor-not-allowed line-through' : ''}">
                {v.name || v.sku}
                {#if !v.is_active}
                  <span class="ml-1 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 text-gray-500 align-middle">
                    {m.admin_products_status_inactive()}
                  </span>
                {/if}
                {#if mode === 'variant-only'}
                  <span class="ml-1 text-[10px] opacity-70">({v.stock_qty})</span>
                {/if}
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Bundle components (read-only) — shown in both modes so admins
           can preview what will be added. In order mode the bundle becomes
           one parent line item with these nested under it. In variant-only
           mode (進出單) each component becomes a child row whose qty is
           perParentQuantity × bundle qty. -->
      {#if selectedProduct.kind === 'bundle' && bundleItems.length > 0}
        <div class="mb-3 grid grid-cols-1 sm:grid-cols-2 gap-1.5">
          {#each bundleItems as bi (bi.id)}
            <div class="flex items-center gap-2 text-xs text-gray-700 bg-white border border-gray-200 rounded-lg px-2 py-1.5">
              {#if bi.component_primary_image_url}
                <img src={bi.component_primary_image_url} alt="" class="w-6 h-6 rounded object-cover bg-gray-100" />
              {/if}
              <span class="flex-1 truncate">{bi.display_name_override || bi.component_product_name}</span>
              <span class="text-gray-400">×{bi.quantity}</span>
            </div>
          {/each}
        </div>
      {/if}

      <!-- Qty + Add -->
      <div class="flex items-center gap-3">
        {#if showQuantity}
          <div class="flex items-center border border-gray-200 rounded-lg overflow-hidden">
            <button type="button" onclick={decQty}
                    class="px-2.5 py-1.5 text-gray-600 hover:bg-gray-50 transition-colors"
                    aria-label="decrease">−</button>
            <input type="number" min="1" value={qty} oninput={onQtyInput}
                   class="w-12 text-center text-sm py-1.5 focus:outline-none" />
            <button type="button" onclick={incQty}
                    class="px-2.5 py-1.5 text-gray-600 hover:bg-gray-50 transition-colors"
                    aria-label="increase">+</button>
          </div>
        {/if}
        <button type="button" onclick={handleAdd}
                disabled={!selectedVariantId}
                class="inline-flex items-center justify-center px-4 py-1.5 text-sm font-medium
                       bg-gray-900 text-white rounded-lg hover:bg-gray-700 transition-colors
                       disabled:opacity-50 disabled:cursor-not-allowed">
          {m.admin_order_create_items_add()}
        </button>
      </div>
    {/if}
  </div>
{/if}
