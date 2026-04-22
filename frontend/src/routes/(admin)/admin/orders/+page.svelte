<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };
</script>

<svelte:head><title>Orders — Gyeon Admin</title></svelte:head>

<div class="max-w-5xl">
  <h1 class="text-2xl font-bold text-gray-900 mb-8">Orders</h1>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    <table class="w-full text-sm">
      <thead class="bg-gray-50 border-b border-gray-100">
        <tr>
          <th class="text-left px-5 py-3 font-medium text-gray-500">Order ID</th>
          <th class="text-left px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">Date</th>
          <th class="text-left px-5 py-3 font-medium text-gray-500">Status</th>
          <th class="text-right px-5 py-3 font-medium text-gray-500">Total</th>
          <th class="px-5 py-3"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.orders as order}
          <tr class="hover:bg-gray-50 transition-colors">
            <td class="px-5 py-3 font-mono text-xs text-gray-700">
              {order.id.slice(0, 8)}…
            </td>
            <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">
              {new Date(order.created_at).toLocaleDateString('en-HK')}
            </td>
            <td class="px-5 py-3">
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                           {statusColour[order.status] ?? 'bg-gray-100 text-gray-500'}">
                {order.status}
              </span>
            </td>
            <td class="px-5 py-3 text-right font-medium text-gray-900">
              HK${order.total.toFixed(2)}
            </td>
            <td class="px-5 py-3 text-right">
              <a href="/admin/orders/{order.id}"
                 class="text-xs text-gray-400 hover:text-gray-700 transition-colors">
                Details →
              </a>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="px-5 py-10 text-center text-gray-400">No orders yet.</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
