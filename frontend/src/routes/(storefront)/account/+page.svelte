<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const statusColors: Record<string, string> = {
    pending:    'bg-yellow-50 text-yellow-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-blue-50 text-blue-700',
    shipped:    'bg-indigo-50 text-indigo-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-600'
  };
</script>

<svelte:head>
  <title>{m.account_overview_title()}</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <!-- Welcome -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h1 class="text-xl font-bold text-gray-900">
      {m.account_overview_hello({ firstName: data.customer?.first_name ?? '' })}
    </h1>
    <p class="text-sm text-gray-500 mt-1">{data.customer?.email}</p>
  </div>

  <!-- Quick links -->
  <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
    {#each [
      { href: '/account/profile',   label: m.account_overview_quick_edit_profile(),  desc: m.account_overview_quick_edit_profile_desc() },
      { href: '/account/addresses', label: m.account_overview_quick_addresses(),     desc: m.account_overview_quick_addresses_desc() },
      { href: '/account/orders',    label: m.account_overview_quick_orders(), desc: m.account_overview_quick_orders_desc() }
    ] as link}
      <a
        href={link.href}
        class="bg-white rounded-2xl border border-gray-100 p-4 hover:border-gray-300 transition-colors"
      >
        <p class="font-semibold text-sm text-gray-900">{link.label}</p>
        <p class="text-xs text-gray-400 mt-0.5">{link.desc}</p>
      </a>
    {/each}
  </div>

  <!-- Recent orders -->
  {#if data.orders.length > 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <div class="flex items-center justify-between mb-4">
        <h2 class="font-semibold text-gray-900">{m.account_overview_recent_heading()}</h2>
        <a href="/account/orders" class="text-sm text-gray-500 hover:text-gray-900 transition-colors">
          {m.common_view_all_arrow()}
        </a>
      </div>
      <div class="flex flex-col gap-3">
        {#each data.orders as order}
          <a
            href="/account/orders/{order.id}"
            class="flex items-center justify-between py-3 border-t border-gray-50 hover:bg-gray-50 -mx-2 px-2 rounded-lg transition-colors"
          >
            <div>
              <p class="text-sm font-medium text-gray-900">#{order.id.slice(0, 8)}</p>
              <p class="text-xs text-gray-400 mt-0.5">{new Date(order.created_at).toLocaleDateString()}</p>
            </div>
            <div class="flex items-center gap-3">
              <span class="text-sm font-medium text-gray-900">
                HK${order.total.toFixed(2)}
              </span>
              <span class="px-2.5 py-1 rounded-full text-xs font-medium capitalize {statusColors[order.status] ?? 'bg-gray-100 text-gray-600'}">
                {order.status}
              </span>
            </div>
          </a>
        {/each}
      </div>
    </div>
  {/if}
</div>
