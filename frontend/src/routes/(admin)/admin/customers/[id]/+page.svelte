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

<svelte:head>
  <title>{data.customer ? `${data.customer.first_name} ${data.customer.last_name}` : 'Customer'} — Gyeon Admin</title>
</svelte:head>

<div class="max-w-4xl">
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/customers" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">← Customers</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">
      {data.customer ? `${data.customer.first_name} ${data.customer.last_name}` : 'Customer Not Found'}
    </h1>
  </div>

  {#if !data.customer}
    <div class="bg-white rounded-2xl border border-gray-100 p-8 text-center text-gray-400">
      Customer not found.
    </div>
  {:else}
    <!-- Profile Card -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
      <h2 class="font-semibold text-gray-900 mb-4">Profile</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Name</p>
          <p class="text-sm text-gray-900">{data.customer.first_name} {data.customer.last_name}</p>
        </div>
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Email</p>
          <p class="text-sm text-gray-900">{data.customer.email}</p>
        </div>
        {#if data.customer.phone}
          <div>
            <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Phone</p>
            <p class="text-sm text-gray-900">{data.customer.phone}</p>
          </div>
        {/if}
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Status</p>
          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                       {data.customer.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
            {data.customer.is_active ? 'Active' : 'Inactive'}
          </span>
        </div>
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Member Since</p>
          <p class="text-sm text-gray-900">{new Date(data.customer.created_at).toLocaleDateString('en-HK')}</p>
        </div>
      </div>
    </div>

    <!-- Order History -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-4 border-b border-gray-100">
        <h2 class="font-semibold text-gray-900">Order History</h2>
      </div>
      {#if data.orders.length === 0}
        <div class="px-6 py-8 text-center text-gray-400 text-sm">No orders yet.</div>
      {:else}
        <table class="w-full text-sm">
          <thead class="bg-gray-50 border-b border-gray-100">
            <tr>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Order ID</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Status</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Total</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">Date</th>
              <th class="px-5 py-3"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-50">
            {#each data.orders as order}
              <tr class="hover:bg-gray-50 transition-colors">
                <td class="px-5 py-3 font-mono text-xs text-gray-500">{order.id.slice(0, 8)}…</td>
                <td class="px-5 py-3">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                               {statusColour[order.status] ?? 'bg-gray-100 text-gray-600'}">
                    {order.status}
                  </span>
                </td>
                <td class="px-5 py-3 font-medium text-gray-900">
                  HK${(order.total ?? 0).toLocaleString('en-HK')}
                </td>
                <td class="px-5 py-3 text-gray-400 text-xs hidden sm:table-cell">
                  {new Date(order.created_at).toLocaleDateString('en-HK')}
                </td>
                <td class="px-5 py-3 text-right">
                  <a href="/admin/orders/{order.id}"
                     class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                    View
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</div>
