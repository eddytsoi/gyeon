<script lang="ts" module>
  // Local row type for the Stock Management mutation editor. Each row is a
  // single variant — no bundle expansion (stock mutations operate at the
  // concrete-variant level). currentStock + projectedAfter are display-only.
  export type MutationItemRow = {
    key: string;          // stable id for the keyed each-block
    variantId: string;
    productName: string;
    sku: string;
    variantName?: string | null;
    primaryImageUrl?: string | null;
    quantity: number;     // always positive; signed by the parent mutation's type
    currentStock?: number | null;
  };
</script>

<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import type { MutationItemRow as Row } from './MutationItemsTable.svelte';

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

  function projected(item: Row): number | null {
    if (item.currentStock == null) return null;
    return type === 'in' ? item.currentStock + item.quantity : item.currentStock - item.quantity;
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
          {@const after = projected(item)}
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
                  <p class="font-medium text-gray-900 leading-snug">{item.productName}</p>
                  <p class="text-xs text-gray-500 mt-0.5">
                    {item.sku}{#if item.variantName} · {item.variantName}{/if}
                  </p>
                </div>
              </div>
            </td>
            <td class="px-2 py-3 text-gray-700 whitespace-nowrap">
              {item.currentStock ?? '—'}
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
              {after ?? '—'}
              {#if negative}
                <span class="ml-1 text-[10px] uppercase tracking-wide">{m.admin_stock_mutations_table_negative_warn()}</span>
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
        {/each}
      </tbody>
    </table>
  </div>
{/if}
