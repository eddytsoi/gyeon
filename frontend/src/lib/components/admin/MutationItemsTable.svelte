<script lang="ts" module>
  // Row type for the Stock Management mutation editor.
  // - kind: 'bundle' rows are display-only parents; their `quantity` drives
  //   the children's effective qty (perParentQuantity × parent.quantity).
  //   currentStock / projected are blank on parent rows — stock impact lives
  //   on the children.
  // - kind: 'simple' (default) is a flat single-variant row.
  // beforeQty / afterQty are the immutable stock snapshot recorded when the
  // mutation was executed. When present (executed) they're shown verbatim;
  // when null (draft) the table falls back to live currentStock + projection
  // so an unsaved edit still previews "what happens if I execute now".
  export type MutationItemComponentRow = {
    variantId: string;
    productName: string;
    sku: string;
    variantName?: string | null;
    primaryImageUrl?: string | null;
    perParentQuantity: number;   // child qty for one bundle unit
    currentStock?: number | null;
    beforeQty?: number | null;   // on-hand snapshot at execute time
    afterQty?: number | null;    // resulting snapshot at execute time
  };
  export type MutationItemRow = {
    key: string;                 // stable id for the keyed each-block
    variantId: string;
    productName: string;
    sku: string;
    variantName?: string | null;
    primaryImageUrl?: string | null;
    quantity: number;            // always positive; signed by the parent mutation's type
    currentStock?: number | null;
    beforeQty?: number | null;   // on-hand snapshot at execute time
    afterQty?: number | null;    // resulting snapshot at execute time
    kind?: 'simple' | 'bundle';
    bundleProductId?: string | null;
    components?: MutationItemComponentRow[];
  };
</script>

<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import type { MutationItemRow as Row, MutationItemComponentRow as CompRow } from './MutationItemsTable.svelte';

  let { items, type, readonly = false, onChangeQty, onRemove }: {
    items: Row[];
    type: 'in' | 'out';
    readonly?: boolean;
    onChangeQty?: (key: string, qty: number) => void;
    onRemove?: (key: string) => void;
  } = $props();

  function dec(item: Row) {
    if (!onChangeQty) return;
    if (item.quantity > 1) onChangeQty(item.key, item.quantity - 1);
  }
  function inc(item: Row) {
    if (!onChangeQty) return;
    onChangeQty(item.key, item.quantity + 1);
  }
  function onQtyInput(item: Row, e: Event) {
    if (!onChangeQty) return;
    const n = parseInt((e.currentTarget as HTMLInputElement).value, 10);
    onChangeQty(item.key, isNaN(n) || n < 1 ? 1 : n);
  }

  // For top-level simple rows.
  function projected(item: Row): number | null {
    if (item.currentStock == null) return null;
    return type === 'in' ? item.currentStock + item.quantity : item.currentStock - item.quantity;
  }
  // For bundle component rows. parentQty drives the multiplier.
  function childQty(c: CompRow, parentQty: number): number {
    return c.perParentQuantity * parentQty;
  }
  function projectedChild(c: CompRow, parentQty: number): number | null {
    if (c.currentStock == null) return null;
    const q = childQty(c, parentQty);
    return type === 'in' ? c.currentStock + q : c.currentStock - q;
  }
</script>

{#if items.length === 0}
  <div class="border-2 border-dashed border-gray-200 rounded-xl px-6 py-10 text-center">
    <p class="text-sm font-medium text-gray-500">📦 {m.admin_stock_mutations_table_empty_title()}</p>
    <p class="text-xs text-gray-400 mt-1">{m.admin_stock_mutations_table_empty_hint()}</p>
  </div>
{:else}
  <div class="overflow-x-auto -mx-2">
    <table class="w-full text-sm">
      <thead>
        <tr class="text-left border-b border-gray-100">
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide">{m.admin_stock_mutations_table_col_product()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap">{m.admin_stock_mutations_table_col_current()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap">{m.admin_stock_mutations_table_col_qty()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap text-right">{m.admin_stock_mutations_table_col_after()}</th>
          {#if !readonly}<th class="px-2 py-2 w-8"></th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each items as item (item.key)}
          {@const isBundle = item.kind === 'bundle'}
          {@const onHand = isBundle ? null : (item.beforeQty ?? item.currentStock ?? null)}
          {@const after = isBundle ? null : (item.afterQty ?? projected(item))}
          {@const negative = after != null && after < 0}
          <tr class="border-b border-gray-100 last:border-b-0 align-top">
            <td class="px-2 py-3">
              <div class="flex items-start gap-2.5">
                {#if item.primaryImageUrl}
                  <img src={item.primaryImageUrl} alt="" class="w-10 h-10 rounded-lg object-cover bg-gray-100 flex-shrink-0" />
                {:else}
                  <div class="w-10 h-10 rounded-lg bg-gray-100 flex-shrink-0"></div>
                {/if}
                <div class="min-w-0">
                  <p class="font-medium text-gray-900 leading-snug flex items-center gap-2 flex-wrap">
                    <span>{item.productName}</span>
                    {#if isBundle}
                      <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-50 text-indigo-700">
                        {m.admin_order_create_items_bundle_badge()}
                      </span>
                    {/if}
                  </p>
                  {#if !isBundle}
                    <p class="text-xs text-gray-500 mt-0.5">
                      {item.sku}{#if item.variantName} · {item.variantName}{/if}
                    </p>
                  {/if}
                </div>
              </div>
            </td>
            <td class="px-2 py-3 text-gray-700 whitespace-nowrap">
              {isBundle ? '—' : (onHand ?? '—')}
            </td>
            <td class="px-2 py-3">
              {#if readonly}
                <span class="font-medium {type === 'in' ? 'text-emerald-700' : 'text-red-600'}">
                  {type === 'in' ? '+' : '−'}{item.quantity}
                </span>
              {:else}
                <div class="inline-flex items-center gap-2">
                  <span class="text-xs text-gray-400 select-none">{type === 'in' ? '+' : '−'}</span>
                  <div class="inline-flex items-center border border-gray-200 rounded-lg overflow-hidden">
                    <button type="button" onclick={() => dec(item)}
                            class="px-2 py-1 text-gray-600 hover:bg-gray-50 transition-colors"
                            aria-label="decrease">−</button>
                    <input type="number" min="1" value={item.quantity} oninput={(e) => onQtyInput(item, e)}
                           class="w-12 text-center text-sm py-1 focus:outline-none" />
                    <button type="button" onclick={() => inc(item)}
                            class="px-2 py-1 text-gray-600 hover:bg-gray-50 transition-colors"
                            aria-label="increase">+</button>
                  </div>
                </div>
              {/if}
            </td>
            <td class="px-2 py-3 text-right whitespace-nowrap font-medium {negative ? 'text-red-600' : 'text-gray-900'}">
              {#if isBundle}
                —
              {:else}
                {after ?? '—'}
                {#if negative}
                  <span class="ml-1 text-[10px] uppercase tracking-wide">{m.admin_stock_mutations_table_negative_warn()}</span>
                {/if}
              {/if}
            </td>
            {#if !readonly}
              <td class="px-2 py-3">
                <button type="button" onclick={() => onRemove?.(item.key)}
                        aria-label="remove row"
                        class="text-gray-400 hover:text-red-500 transition-colors">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                  </svg>
                </button>
              </td>
            {/if}
          </tr>
          {#if isBundle && item.components}
            {#each item.components as child, ci (item.key + '-' + ci)}
              {@const cQty = childQty(child, item.quantity)}
              {@const cOnHand = child.beforeQty ?? child.currentStock ?? null}
              {@const cAfter = child.afterQty ?? projectedChild(child, item.quantity)}
              {@const cNegative = cAfter != null && cAfter < 0}
              <tr class="border-b border-gray-100 last:border-b-0 align-top bg-gray-50/60">
                <td class="px-2 py-2 pl-10">
                  <div class="flex items-start gap-2.5">
                    {#if child.primaryImageUrl}
                      <img src={child.primaryImageUrl} alt="" class="w-8 h-8 rounded object-cover bg-white flex-shrink-0" />
                    {:else}
                      <div class="w-8 h-8 rounded bg-white flex-shrink-0"></div>
                    {/if}
                    <div class="min-w-0">
                      <p class="text-sm text-gray-700 leading-snug">↳ {child.productName}</p>
                      <p class="text-xs text-gray-400 mt-0.5">
                        {child.sku}{#if child.variantName} · {child.variantName}{/if}
                      </p>
                    </div>
                  </div>
                </td>
                <td class="px-2 py-2 text-gray-600 whitespace-nowrap text-xs">
                  {cOnHand ?? '—'}
                </td>
                <td class="px-2 py-2 text-xs">
                  <span class="font-medium {type === 'in' ? 'text-emerald-700' : 'text-red-600'}">
                    {type === 'in' ? '+' : '−'}{cQty}
                  </span>
                  <span class="ml-1 text-[10px] text-gray-400">({child.perParentQuantity}×{item.quantity})</span>
                </td>
                <td class="px-2 py-2 text-right whitespace-nowrap text-xs font-medium {cNegative ? 'text-red-600' : 'text-gray-700'}">
                  {cAfter ?? '—'}
                  {#if cNegative}
                    <span class="ml-1 text-[10px] uppercase tracking-wide">{m.admin_stock_mutations_table_negative_warn()}</span>
                  {/if}
                </td>
                {#if !readonly}<td class="px-2 py-2"></td>{/if}
              </tr>
            {/each}
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}
