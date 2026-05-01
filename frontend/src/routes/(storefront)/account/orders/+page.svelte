<script lang="ts">
  import type { PageData } from './$types';

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

  // ── Order list magnetic spotlight ───────────────────────────────
  let listEl = $state<HTMLElement | undefined>();
  let spotlight = $state({ visible: false, top: 0, left: 0, width: 0, height: 0 });

  function moveSpotlightTo(item: Element | null) {
    if (!item || !listEl || !listEl.contains(item)) {
      spotlight.visible = false;
      return;
    }
    const listRect = listEl.getBoundingClientRect();
    const itemRect = item.getBoundingClientRect();
    spotlight = {
      visible: true,
      top: itemRect.top - listRect.top + listEl.scrollTop,
      left: itemRect.left - listRect.left + listEl.scrollLeft,
      width: itemRect.width,
      height: itemRect.height
    };
  }

  function onListMouseMove(e: MouseEvent) {
    moveSpotlightTo((e.target as HTMLElement | null)?.closest('.js-order-row') ?? null);
  }

  function onListMouseLeave() {
    spotlight.visible = false;
  }
</script>

<svelte:head>
  <title>Order History — Gyeon</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <h1 class="text-xl font-bold text-gray-900">Order History</h1>

  {#if data.orders.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">No orders yet.</p>
      <a href="/products" class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline">
        Start shopping →
      </a>
    </div>
  {:else}
    <div bind:this={listEl}
         onmousemove={onListMouseMove}
         onmouseleave={onListMouseLeave}
         class="relative bg-white rounded-2xl border border-gray-100 divide-y divide-gray-50 overflow-hidden">
      <!-- Magnetic spotlight: glides under the cursor and snaps to the hovered row -->
      <div aria-hidden="true"
           class="pointer-events-none absolute z-0 bg-gray-50
                  transition-[transform,width,height,opacity] duration-[80ms] ease-out
                  {spotlight.visible ? 'opacity-100' : 'opacity-0'}"
           style="top: 0; left: 0; transform: translate3d({spotlight.left}px, {spotlight.top}px, 0); width: {spotlight.width}px; height: {spotlight.height}px;">
      </div>

      {#each data.orders as order}
        <a
          href="/account/orders/ORD-{order.number}"
          class="js-order-row relative z-10 flex items-center justify-between px-6 py-4 transition-colors"
        >
          <div class="flex flex-col gap-0.5">
            <p class="text-sm font-semibold text-gray-900 font-mono">{order.order_number || `ORD-${order.number}`}</p>
            <p class="text-xs text-gray-400">{new Date(order.created_at).toLocaleDateString()}</p>
          </div>
          <div class="flex items-center gap-4">
            <span class="text-sm text-gray-600">{order.items_count ?? 0} item{(order.items_count ?? 0) !== 1 ? 's' : ''}</span>
            <span class="text-sm font-semibold text-gray-900">HK${order.total.toFixed(2)}</span>
            <span class="px-2.5 py-1 rounded-full text-xs font-medium capitalize {statusColors[order.status] ?? 'bg-gray-100 text-gray-600'}">
              {order.status}
            </span>
            <svg class="w-4 h-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </a>
      {/each}
    </div>

    <!-- Pagination -->
    {#if data.orders.length === 20 || data.offset > 0}
      <div class="flex justify-between">
        {#if data.offset > 0}
          <a href="?offset={Math.max(0, data.offset - 20)}"
            class="text-sm text-gray-600 hover:text-gray-900 transition-colors">← Previous</a>
        {:else}
          <span></span>
        {/if}
        {#if data.orders.length === 20}
          <a href="?offset={data.offset + 20}"
            class="text-sm text-gray-600 hover:text-gray-900 transition-colors">Next →</a>
        {/if}
      </div>
    {/if}
  {/if}
</div>
