<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { adminDeleteOrder } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import type { Order } from '$lib/types';
  import type { PageData } from './$types';
  import { spotlight } from '$lib/actions/spotlight';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';
  import { orderStatusLabel } from '$lib/orderStatus';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<Order | null>(null);
  let deleting = $state(false);

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };

  async function confirmDelete() {
    if (!deleteTarget || !data.token) return;
    const target = deleteTarget;
    const shortId = target.order_number || `ORD-${target.number}`;
    deleting = true;
    try {
      await adminDeleteOrder(data.token, target.id);
      notify.success(m.admin_orders_deleted_success({ id: shortId }));
      deleteTarget = null;
      await invalidateAll();
    } catch (e) {
      notify.error(
        m.admin_orders_delete_failure({ id: shortId }),
        e instanceof Error ? e.message : m.admin_orders_delete_failure_default()
      );
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head><title>{m.admin_orders_title()}</title></svelte:head>

<h1 class="text-2xl font-bold text-gray-900 mb-8">{m.admin_orders_heading()}</h1>

<div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
     use:spotlight={{ selector: '.js-row' }}>
  <table class="w-full text-sm">
    <thead class="bg-gray-50 border-b border-gray-100">
      <tr>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_id()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">{m.admin_orders_col_date()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_status()}</th>
        <th class="text-right px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_total()}</th>
        <th class="px-5 py-3"></th>
      </tr>
    </thead>
    <tbody class="divide-y divide-gray-50">
      {#each data.orders as order}
        <tr class="js-row transition-colors">
          <td class="px-5 py-3 font-mono text-xs text-gray-700">
            <span class="inline-flex items-center gap-2">
              {order.order_number || `ORD-${order.number}`}
              {#if (data.unreadCounts?.[order.id] ?? 0) > 0}
                <span title={m.admin_orders_unread_aria()}
                      class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1.5
                             rounded-full bg-green-500 text-white text-[10px] font-bold leading-none">
                  {data.unreadCounts[order.id]}
                </span>
              {/if}
            </span>
          </td>
          <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">
            {new Date(order.created_at).toLocaleDateString('en-HK')}
          </td>
          <td class="px-5 py-3">
            <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                         {statusColour[order.status] ?? 'bg-gray-100 text-gray-500'}">
              {orderStatusLabel(order.status)}
            </span>
          </td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">
            HK${order.total.toFixed(2)}
          </td>
          <td class="px-5 py-3">
            <div class="flex items-center justify-end gap-1">
              <a href="/admin/orders/{order.id}"
                 title={m.admin_orders_action_details()}
                 aria-label={m.admin_orders_aria_details()}
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178Z"/>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
                </svg>
              </a>
              <button onclick={() => deleteTarget = order}
                      title={m.admin_orders_action_delete()}
                      aria-label={m.admin_orders_aria_delete()}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                </svg>
              </button>
            </div>
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan="5" class="px-5 py-10 text-center text-gray-400">{m.admin_orders_empty()}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<Pagination total={data.total} pageSize={data.pageSize} currentPage={data.page} />

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => { if (!deleting) deleteTarget = null; }}
         role="button" tabindex="-1" aria-label={m.admin_orders_aria_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_orders_delete_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_orders_delete_body_pre()}<span class="font-mono font-medium text-gray-700">{deleteTarget.order_number || `ORD-${deleteTarget.number}`}</span>{m.admin_orders_delete_body_post()}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null} disabled={deleting}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50">
          {m.common_cancel()}
        </button>
        <button onclick={confirmDelete} disabled={deleting}
                class="flex-1 px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                       hover:bg-red-600 transition-colors disabled:opacity-50">
          {deleting ? m.admin_orders_deleting() : m.admin_orders_delete_button()}
        </button>
      </div>
    </div>
  </div>
{/if}
