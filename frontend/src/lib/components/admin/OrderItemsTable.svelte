<script lang="ts" module>
  // Local row type — the page owns the source-of-truth list and passes it in.
  // Bundle parents include `components`; each component row carries its
  // per-parent quantity so the indented child rows can display `×N` next to
  // the bundle quantity stepper.
  export type OrderItemRow = {
    key: string; // stable id for the keyed each-block (variantId + a salt)
    variantId: string;
    productName: string;
    sku: string;
    unitPrice: number;
    quantity: number;
    primaryImageUrl?: string | null;
    kind: 'simple' | 'bundle' | string;
    components: Array<{
      variantId: string;
      productName: string;
      sku: string;
      unitPrice: number;
      perParentQuantity: number;
      primaryImageUrl?: string | null;
    }>;
  };
</script>

<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import type { OrderItemRow as Row } from './OrderItemsTable.svelte';

  let { items, onChangeQty, onRemove }: {
    items: Row[];
    onChangeQty: (key: string, qty: number) => void;
    onRemove: (key: string) => void;
  } = $props();

  function dec(item: Row) {
    if (item.quantity > 1) onChangeQty(item.key, item.quantity - 1);
  }
  function inc(item: Row) {
    onChangeQty(item.key, item.quantity + 1);
  }
  function onQtyInput(item: Row, e: Event) {
    const n = parseInt((e.currentTarget as HTMLInputElement).value, 10);
    onChangeQty(item.key, isNaN(n) || n < 1 ? 1 : n);
  }
</script>

{#if items.length === 0}
  <div class="border-2 border-dashed border-gray-200 rounded-xl px-6 py-10 text-center">
    <p class="text-sm font-medium text-gray-500">📦 {m.admin_order_create_items_empty_title()}</p>
    <p class="text-xs text-gray-400 mt-1">{m.admin_order_create_items_empty_hint()}</p>
  </div>
{:else}
  <div class="overflow-x-auto -mx-2">
    <table class="w-full text-sm">
      <thead>
        <tr class="text-left border-b border-gray-100">
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide">{m.admin_order_create_items_col_product()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap">{m.admin_order_create_items_col_price()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap">{m.admin_order_create_items_col_qty()}</th>
          <th class="px-2 py-2 font-medium text-gray-500 text-xs uppercase tracking-wide whitespace-nowrap text-right">{m.admin_order_create_items_col_subtotal()}</th>
          <th class="px-2 py-2 w-8"></th>
        </tr>
      </thead>
      <tbody>
        {#each items as item (item.key)}
          <tr class="border-b border-gray-100 last:border-b-0 align-top">
            <td class="px-2 py-3">
              <div class="flex items-start gap-2.5">
                {#if item.primaryImageUrl}
                  <img src={item.primaryImageUrl} alt="" class="w-10 h-10 rounded-lg object-cover bg-gray-100 flex-shrink-0" />
                {:else}
                  <div class="w-10 h-10 rounded-lg bg-gray-100 flex-shrink-0"></div>
                {/if}
                <div class="min-w-0">
                  <p class="font-medium text-gray-900 leading-snug flex items-center gap-1.5 flex-wrap">
                    <span>{item.productName}</span>
                    {#if item.kind === 'bundle'}
                      <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-50 text-indigo-700">
                        {m.admin_order_create_items_bundle_badge()}
                      </span>
                    {/if}
                  </p>
                  <p class="text-xs text-gray-500 mt-0.5">{item.sku}</p>
                  {#if item.kind === 'bundle' && item.components.length > 0}
                    <ul class="mt-1.5 space-y-0.5">
                      {#each item.components as c (c.variantId)}
                        <li class="text-xs text-gray-400 pl-3 truncate">↳ {c.productName} ({c.sku}) ×{c.perParentQuantity * item.quantity}</li>
                      {/each}
                    </ul>
                  {/if}
                </div>
              </div>
            </td>
            <td class="px-2 py-3 text-gray-700 whitespace-nowrap">HK${item.unitPrice.toFixed(2)}</td>
            <td class="px-2 py-3">
              <div class="inline-flex items-center border border-gray-200 rounded-lg overflow-hidden">
                <button type="button" onclick={() => dec(item)}
                        class="px-2 py-1 text-gray-600 hover:bg-gray-50 transition-colors"
                        aria-label="decrease">−</button>
                <input type="number" min="1" value={item.quantity} oninput={(e) => onQtyInput(item, e)}
                       class="w-10 text-center text-sm py-1 focus:outline-none" />
                <button type="button" onclick={() => inc(item)}
                        class="px-2 py-1 text-gray-600 hover:bg-gray-50 transition-colors"
                        aria-label="increase">+</button>
              </div>
            </td>
            <td class="px-2 py-3 text-right text-gray-900 font-medium whitespace-nowrap">
              HK${(item.unitPrice * item.quantity).toFixed(2)}
            </td>
            <td class="px-2 py-3">
              <button type="button" onclick={() => onRemove(item.key)}
                      aria-label={m.admin_order_create_items_remove_aria()}
                      class="text-gray-400 hover:text-red-500 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
