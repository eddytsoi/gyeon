<script lang="ts">
  import type { StockMovementRow } from '$lib/api/admin';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    items: StockMovementRow[];
    /** Hide the Product/SKU column when the table is already scoped to one product. */
    hideProduct?: boolean;
  }

  let { items, hideProduct = false }: Props = $props();

  function fmtDate(s: string): string {
    return new Date(s).toLocaleString();
  }

  // Pick a colour class from the reason: out-of-stock motions are red,
  // restocks (admin or order-return) are green, neutral admin edits are gray.
  function reasonClasses(reason: string, delta: number): string {
    if (delta < 0) return 'bg-red-50 text-red-700';
    if (reason.startsWith('order.')) return 'bg-emerald-50 text-emerald-700';
    return 'bg-gray-100 text-gray-700';
  }
</script>

{#if items.length === 0}
  <div class="flex flex-col items-center justify-center py-20 text-center">
    <p class="text-sm font-medium text-gray-400">{m.admin_stock_history_empty()}</p>
  </div>
{:else}
  <table class="w-full text-sm">
    <thead>
      <tr class="border-b border-gray-50">
        <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_time()}</th>
        {#if !hideProduct}
          <th class="text-left px-3 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_product()}</th>
        {/if}
        <th class="text-left px-3 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_change()}</th>
        <th class="text-left px-3 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_levels()}</th>
        <th class="text-left px-3 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_reason()}</th>
        <th class="text-left px-3 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_actor()}</th>
        <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_stock_history_col_order()}</th>
      </tr>
    </thead>
    <tbody class="divide-y divide-gray-50">
      {#each items as row}
        <tr class="transition-colors hover:bg-gray-50/40">
          <td class="px-6 py-3 text-gray-600 text-xs whitespace-nowrap">{fmtDate(row.created_at)}</td>
          {#if !hideProduct}
            <td class="px-3 py-3 text-gray-700 text-xs">
              {#if row.product_id}
                <a class="text-gray-900 hover:underline" href={`/admin/products/${row.product_id}`}>
                  {row.product_name ?? '—'}
                </a>
              {:else}
                <span class="text-gray-500">{row.product_name ?? '—'}</span>
              {/if}
              {#if row.variant_sku}
                <span class="block text-gray-400 font-mono">{row.variant_sku}</span>
              {/if}
            </td>
          {/if}
          <td class="px-3 py-3 text-xs font-semibold whitespace-nowrap"
              class:text-red-600={row.delta < 0}
              class:text-emerald-600={row.delta > 0}>
            {row.delta > 0 ? `+${row.delta}` : row.delta}
          </td>
          <td class="px-3 py-3 text-xs text-gray-600 whitespace-nowrap font-mono">
            {row.before_qty} → {row.after_qty}
          </td>
          <td class="px-3 py-3 text-xs">
            <span class="inline-flex items-center px-2 py-0.5 rounded-full font-mono text-[11px] {reasonClasses(row.reason, row.delta)}">
              {row.reason}
            </span>
          </td>
          <td class="px-3 py-3 text-gray-600 text-xs">
            {row.actor_email ?? m.admin_stock_history_actor_system()}
          </td>
          <td class="px-6 py-3 text-xs">
            {#if row.order_id}
              <a class="text-gray-900 hover:underline font-mono" href={`/admin/orders/${row.order_id}`}>
                {row.order_number || `ORD-${row.order_id.slice(0, 8)}`}
              </a>
            {:else}
              <span class="text-gray-400">—</span>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}
