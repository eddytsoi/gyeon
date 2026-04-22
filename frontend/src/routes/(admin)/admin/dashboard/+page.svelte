<script lang="ts">
  import type { PageData } from './$types';
  let { data }: { data: PageData } = $props();
</script>

<svelte:head><title>Dashboard — Gyeon Admin</title></svelte:head>

<div class="max-w-5xl space-y-8">

  <!-- Greeting -->
  <div>
    <h2 class="text-2xl font-bold text-gray-900">Good day 👋</h2>
    <p class="text-sm text-gray-500 mt-1">Here's what's happening in your store.</p>
  </div>

  {#if data.stats}
    <!-- Stats grid — static class names so Tailwind includes them -->
    <div class="grid grid-cols-2 xl:grid-cols-4 gap-4">

      <!-- Total Products -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-violet-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.total_products}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">Total Products</p>
      </div>

      <!-- Total Orders -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-blue-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.total_orders}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">Total Orders</p>
      </div>

      <!-- Pending Orders -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-amber-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.pending_orders}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">Pending Orders</p>
      </div>

      <!-- Total Revenue -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-emerald-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-2xl font-bold text-gray-900 tabular-nums">
          HK${data.stats.total_revenue.toLocaleString('en-HK', { minimumFractionDigits: 0 })}
        </p>
        <p class="text-xs text-gray-400 font-medium mt-1">Total Revenue</p>
      </div>

    </div>

    <!-- Quick actions -->
    <div>
      <p class="text-xs font-semibold text-gray-400 uppercase tracking-widest mb-4">Quick Actions</p>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">

        <a href="/admin/products"
           class="group bg-white rounded-2xl border border-gray-100 p-5 shadow-sm
                  hover:shadow-md hover:border-gray-200 transition-all flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-gray-100 group-hover:bg-gray-900
                      flex items-center justify-center transition-colors flex-shrink-0">
            <svg class="w-5 h-5 text-gray-500 group-hover:text-white transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
            </svg>
          </div>
          <div class="flex-1 min-w-0">
            <p class="font-semibold text-gray-900 text-sm">Manage Products</p>
            <p class="text-xs text-gray-400 mt-0.5">Add, edit or remove products</p>
          </div>
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-600 group-hover:translate-x-0.5
                      transition-all flex-shrink-0"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5"/>
          </svg>
        </a>

        <a href="/admin/orders"
           class="group bg-white rounded-2xl border border-gray-100 p-5 shadow-sm
                  hover:shadow-md hover:border-gray-200 transition-all flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-gray-100 group-hover:bg-gray-900
                      flex items-center justify-center transition-colors flex-shrink-0">
            <svg class="w-5 h-5 text-gray-500 group-hover:text-white transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"/>
            </svg>
          </div>
          <div class="flex-1 min-w-0">
            <p class="font-semibold text-gray-900 text-sm">View Orders</p>
            <p class="text-xs text-gray-400 mt-0.5">Process and track orders</p>
          </div>
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-600 group-hover:translate-x-0.5
                      transition-all flex-shrink-0"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5"/>
          </svg>
        </a>

      </div>
    </div>

  {:else}
    <div class="h-40 flex items-center justify-center text-gray-400 text-sm">
      Could not load stats.
    </div>
  {/if}

</div>
