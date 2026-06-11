<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import * as m from '$lib/paraglide/messages';
  import { siteName } from '$lib/seo';
  import { orderStatusLabel } from '$lib/orderStatus';
  import { formatHKD } from '$lib/money';
  import { formatOrderDateTime } from '$lib/datetime';
  import { isBankTransferRole } from '$lib/bankTransfer';

  let { data }: { data: PageData } = $props();

  // Installer / installer_v2 pay only by bank transfer (no Stripe), so the
  // "立即付款" pay-now action never applies to them — hide it on their orders.
  const hidePayNow = $derived(isBankTransferRole(data.customer?.role));

  const statusColors: Record<string, string> = {
    pending:    'bg-yellow-50 text-yellow-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-blue-50 text-blue-700',
    prepared:   'bg-cyan-50 text-cyan-700',
    shipped:    'bg-indigo-50 text-indigo-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-600'
  };

  // Status filter chips — order matches $lib/orderStatus.
  const STATUSES = ['pending', 'paid', 'processing', 'prepared', 'shipped', 'delivered', 'cancelled', 'refunded'];

  // Build an /account/orders URL preserving the active status + search unless
  // overridden; offset resets to 0 by default so changing a filter/search
  // jumps back to the first page.
  function ordersUrl({ status = data.status, q = data.q, offset = 0 }: { status?: string; q?: string; offset?: number } = {}) {
    const sp = new URLSearchParams();
    if (status) sp.set('status', status);
    if (q) sp.set('q', q);
    if (offset) sp.set('offset', String(offset));
    const s = sp.toString();
    return '/account/orders' + (s ? `?${s}` : '');
  }

  const hasPrev = $derived(data.offset > 0);
  const hasNext = $derived(data.offset + data.orders.length < data.total);
  const shownFrom = $derived(data.total === 0 ? 0 : data.offset + 1);
  const shownTo = $derived(data.offset + data.orders.length);

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
  <title>{m.account_orders_title({ brand: siteName(data.publicSettings) })}</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <h1 class="text-xl font-bold text-gray-900">{m.account_orders_heading()}</h1>

  <!-- Status dropdown (left) + search by product name (right).
       Phones stack into two rows; ≥sm sits side-by-side. -->
  <div class="flex flex-col sm:flex-row gap-3">
    <!-- Status filter — native <select>, navigates on change -->
    <select
      value={data.status}
      onchange={(e) => goto(ordersUrl({ status: e.currentTarget.value }), { invalidateAll: true })}
      aria-label={m.account_orders_filter_label()}
      class="max-sm:w-full sm:w-48 shrink-0 border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
             focus:outline-none focus:ring-2 focus:ring-gray-900"
    >
      <option value="">{m.account_orders_filter_all()}</option>
      {#each STATUSES as st}
        <option value={st}>{orderStatusLabel(st)}</option>
      {/each}
    </select>

    <!-- Search by product name (fills remaining width) -->
    <form method="GET" class="relative flex-1">
      {#if data.status}<input type="hidden" name="status" value={data.status} />{/if}
      <svg class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
           fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-4.3-4.3m1.8-4.45a6.25 6.25 0 1 1-12.5 0 6.25 6.25 0 0 1 12.5 0Z" />
      </svg>
      <input
        type="search" name="q" value={data.q}
        onblur={(e) => {
          // Refresh on un-focus too (not just Enter), but only when the text changed.
          const v = e.currentTarget.value.trim();
          if (v !== (data.q ?? '').trim()) e.currentTarget.form?.requestSubmit();
        }}
        placeholder={m.account_orders_search_placeholder()}
        aria-label={m.account_orders_search_placeholder()}
        class="w-full pl-10 pr-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
      />
    </form>
  </div>

  {#if data.orders.length === 0}
    {#if data.status || data.q}
      <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
        <p class="text-gray-400 text-sm">{m.account_orders_no_results()}</p>
        <a href="/account/orders" class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline">
          {m.account_orders_clear()}
        </a>
      </div>
    {:else}
      <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
        <p class="text-gray-400 text-sm">{m.account_orders_empty()}</p>
        <a href="/products" class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline">
          {m.account_orders_start_shopping()}
        </a>
      </div>
    {/if}
  {:else}
    <div class="flex items-center justify-between">
      <p class="text-xs text-gray-400">{m.account_orders_count({ count: data.total })}</p>
      {#if data.status || data.q}
        <a href="/account/orders" class="text-xs text-gray-500 hover:text-gray-900 transition-colors">{m.account_orders_clear()}</a>
      {/if}
    </div>
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
            <p class="text-sm font-semibold text-gray-900 font-mono inline-flex items-center gap-2">
              {order.order_number || `ORD-${order.number}`}
              {#if (data.unreadCounts?.[order.id] ?? 0) > 0}
                <span title={m.account_orders_unread_aria()}
                      class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1.5
                             rounded-full bg-blue-500 text-white text-[10px] font-bold leading-none">
                  {data.unreadCounts[order.id]}
                </span>
              {/if}
            </p>
            <p class="text-xs text-gray-400">{formatOrderDateTime(order.created_at)}</p>
          </div>
          <div class="flex items-center gap-4">
            <span class="hidden sm:inline text-sm text-gray-600">{(order.items_count ?? 0) === 1 ? m.account_orders_items_one({ count: order.items_count ?? 0 }) : m.account_orders_items_many({ count: order.items_count ?? 0 })}</span>
            <span class="text-sm font-semibold text-gray-900">{formatHKD(order.total)}</span>
            <span class="px-2.5 py-1 rounded-full text-xs font-medium {statusColors[order.status] ?? 'bg-gray-100 text-gray-600'}">
              {orderStatusLabel(order.status)}
            </span>
            {#if order.status === 'pending' && order.payment_status !== 'succeeded' && !hidePayNow}
              <span class="px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-500 text-white">
                {m.account_order_pay_now()}
              </span>
            {/if}
            <svg class="w-4 h-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </a>
      {/each}
    </div>

    <!-- Pagination -->
    {#if hasPrev || hasNext}
      <div class="flex justify-between items-center gap-4">
        {#if hasPrev}
          <a href={ordersUrl({ offset: Math.max(0, data.offset - data.limit) })}
            class="text-sm text-gray-600 hover:text-gray-900 transition-colors">{m.common_previous_arrow()}</a>
        {:else}
          <span></span>
        {/if}
        <span class="text-xs text-gray-400 tabular-nums">{shownFrom}–{shownTo} / {data.total}</span>
        {#if hasNext}
          <a href={ordersUrl({ offset: data.offset + data.limit })}
            class="text-sm text-gray-600 hover:text-gray-900 transition-colors">{m.common_next_arrow()}</a>
        {:else}
          <span></span>
        {/if}
      </div>
    {/if}
  {/if}
</div>
