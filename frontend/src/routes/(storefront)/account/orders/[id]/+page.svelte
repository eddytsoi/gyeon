<script lang="ts">
  import { enhance } from '$app/forms';
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import type { ActionData, PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import { orderStatusLabel } from '$lib/orderStatus';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { renderNoticeBody } from '$lib/orderNotice';
  import AppliedPromotions from '$lib/components/AppliedPromotions.svelte';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const order = $derived(data.order);
  const notices = $derived(data.notices ?? []);

  // ── Receipt cache lightning-icon ────────────────────────────────────────
  // No customer-side SSE in this app, so we poll the cache-status endpoint
  // every ~3 s while the page is mounted and the order is in a receiptable
  // status. Stops polling as soon as the icon lights up (or the order is
  // not receiptable).
  let receiptReady = $state(false);
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  const receiptStatuses = ['paid', 'processing', 'shipped', 'delivered'];

  async function checkReceiptCache(orderId: string): Promise<boolean> {
    try {
      const res = await fetch(`/account/orders/${orderId}/receipt-cache-status`, {
        headers: { Accept: 'application/json' }
      });
      if (!res.ok) return false;
      const j = (await res.json()) as { available?: boolean };
      return j.available === true;
    } catch {
      return false;
    }
  }

  onMount(() => {
    if (!receiptStatuses.includes(order.status)) return;
    // Initial probe — if the queue worker has already finished by the time
    // the customer lands here, the icon shows immediately without waiting
    // for the first poll tick.
    checkReceiptCache(order.id).then((ready) => {
      if (ready) {
        receiptReady = true;
        return;
      }
      pollTimer = setInterval(async () => {
        if (!receiptStatuses.includes(order.status)) {
          stopPolling();
          return;
        }
        if (await checkReceiptCache(order.id)) {
          receiptReady = true;
          stopPolling();
        }
      }, 3000);
    });
  });

  onDestroy(() => stopPolling());

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  // Group order items by parent_item_id so bundle component rows appear
  // indented under their bundle parent line.
  const parentItems = $derived((order.items ?? []).filter((it) => !it.parent_item_id));
  const childrenByParent = $derived.by(() => {
    const map: Record<string, typeof order.items> = {};
    for (const it of order.items ?? []) {
      if (it.parent_item_id) {
        (map[it.parent_item_id] ??= []).push(it);
      }
    }
    return map;
  });

  let sending = $state(false);
  let messageBody = $state('');

  // Reorder: re-add every parent line back into the cart, then go to /cart.
  // Loop parent items only — the backend re-expands bundles, so adding bundle
  // child rows too would double-add.
  let reordering = $state(false);
  async function reorder() {
    if (reordering) return;
    reordering = true;
    try {
      if (!cartStore.cart) await cartStore.init();
      let added = 0;
      for (const item of parentItems) {
        if (!item.variant_id) continue;
        try {
          await cartStore.add(item.variant_id, item.quantity);
          added++;
        } catch {
          // per-item failure already surfaces via cartStore.error toast
        }
      }
      if (added > 0) await goto('/cart');
    } finally {
      reordering = false;
    }
  }

  const statusColors: Record<string, string> = {
    pending:    'bg-yellow-50 text-yellow-700 border-yellow-100',
    paid:       'bg-blue-50 text-blue-700 border-blue-100',
    processing: 'bg-blue-50 text-blue-700 border-blue-100',
    shipped:    'bg-indigo-50 text-indigo-700 border-indigo-100',
    delivered:  'bg-green-50 text-green-700 border-green-100',
    cancelled:  'bg-gray-100 text-gray-500 border-gray-200',
    refunded:   'bg-red-50 text-red-600 border-red-100'
  };

  const statusSteps = ['pending', 'paid', 'processing', 'shipped', 'delivered'];
  const currentStep = $derived(statusSteps.indexOf(order.status));

  function fmtNoticeTime(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleString('en-HK', { dateStyle: 'medium', timeStyle: 'short' });
  }
</script>

<svelte:head>
  <title>{m.account_order_title({ orderNumber: order.order_number || `ORD-${order.number}` })}</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <!-- Header -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <div class="flex items-start justify-between flex-wrap gap-4">
      <div>
        <a href="/account/orders" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">{m.account_order_back()}</a>
        <h1 class="text-xl font-bold text-gray-900 mt-1 font-mono">
          {order.order_number || `ORD-${order.number}`}
        </h1>
        <p class="text-sm text-gray-500 mt-0.5">
          {m.account_order_placed_on({ date: new Date(order.created_at).toLocaleDateString('en-HK', { dateStyle: 'long' }) })}
        </p>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        {#if order.status === 'delivered'}
          <button type="button" onclick={reorder} disabled={reordering}
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium
                         text-gray-700 border border-gray-200 hover:bg-gray-50 transition-colors
                         disabled:opacity-50">
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="23 4 23 10 17 10"/>
              <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
            </svg>
            {reordering ? m.account_order_reorder_adding() : m.account_order_reorder()}
          </button>
        {/if}
        {#if receiptStatuses.includes(order.status)}
          <a href="/account/orders/{order.id}/receipt.pdf"
             target="_blank" rel="noopener"
             class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium
                    text-gray-700 border border-gray-200 hover:bg-gray-50 transition-colors"
             title={receiptReady ? m.account_order_receipt_cached_tooltip() : m.account_order_receipt_download()}>
            {#if receiptReady}
              <!-- ⚡ cache-ready indicator -->
              <svg class="w-4 h-4 text-amber-500" viewBox="0 0 24 24"
                   fill="currentColor" aria-hidden="true">
                <path d="M13 2 4 14h6l-1 8 9-12h-6l1-8z"/>
              </svg>
            {/if}
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7 10 12 15 17 10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
            {m.account_order_receipt_download()}
          </a>
        {/if}
        <span class="px-3 py-1.5 rounded-full text-sm font-medium border {statusColors[order.status] ?? 'bg-gray-100 text-gray-600 border-gray-200'}">
          {orderStatusLabel(order.status)}
        </span>
      </div>
    </div>

    <!-- Progress bar (for non-terminal statuses) -->
    {#if currentStep >= 0}
      <div class="mt-6">
        <div class="flex items-center gap-0">
          {#each statusSteps as step, i}
            <div class="flex items-center {i < statusSteps.length - 1 ? 'flex-1' : ''}">
              <div class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold flex-shrink-0
                          {i <= currentStep ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-400'}">
                {i < currentStep ? '✓' : i + 1}
              </div>
              {#if i < statusSteps.length - 1}
                <div class="flex-1 h-0.5 mx-1 {i < currentStep ? 'bg-gray-900' : 'bg-gray-100'}"></div>
              {/if}
            </div>
          {/each}
        </div>
        <div class="flex justify-between mt-2">
          {#each statusSteps as step}
            <span class="text-xs text-gray-400" style="width: 20%; text-align: center">{orderStatusLabel(step)}</span>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- Items -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.account_order_items()}</h2>
    <div class="flex flex-col divide-y divide-gray-50">
      {#each parentItems as item}
        <div class="py-3">
          <div class="flex items-center justify-between">
            <div class="flex-1">
              <p class="text-sm font-medium text-gray-900">{item.product_name}</p>
              <p class="text-xs text-gray-400 mt-0.5">{m.account_order_item_meta({ sku: item.variant_sku, quantity: item.quantity })}</p>
            </div>
            <p class="text-sm font-semibold text-gray-900 ml-4">HK${item.line_total.toFixed(2)}</p>
          </div>
          {#if childrenByParent[item.id]?.length}
            <ul class="mt-2 pl-4 border-l border-gray-100 flex flex-col gap-1">
              {#each childrenByParent[item.id] as child}
                <li class="flex items-center justify-between text-xs text-gray-500">
                  <span class="truncate">↳ {child.product_name}</span>
                  <span class="flex-shrink-0 tabular-nums">× {child.quantity}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}
    </div>
  </div>

  <!-- Summary -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.account_order_summary()}</h2>
    <div class="flex flex-col gap-2 text-sm">
      <div class="flex justify-between text-gray-600">
        <span>{m.account_order_subtotal()}</span>
        <span>HK${order.subtotal.toFixed(2)}</span>
      </div>
      {#if order.discount_amount > 0}
        <div class="flex justify-between text-green-600">
          <span>{m.account_order_discount()}</span>
          <span>−HK${order.discount_amount.toFixed(2)}</span>
        </div>
      {/if}
      <div class="flex justify-between text-gray-600">
        <span>{m.account_order_shipping()}</span>
        <span>{order.shipping_free ? m.shipping_sf_free() : m.shipping_sf_cod()}</span>
      </div>
      <div class="flex justify-between font-bold text-gray-900 pt-2 border-t border-gray-100 text-base">
        <span>{m.account_order_total()}</span>
        <span>HK${order.total.toFixed(2)}</span>
      </div>
      <AppliedPromotions promotions={order.applied_promotions ?? []} />
    </div>
  </div>

  {#if order.notes}
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <h2 class="font-semibold text-gray-900 mb-2">{m.account_order_notes_heading()}</h2>
      <p class="text-sm text-gray-600">{order.notes}</p>
    </div>
  {/if}

  <!-- Messages between customer and admin -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.account_order_messages_heading()}</h2>

    {#if notices.length === 0}
      <p class="text-sm text-gray-400 italic">{m.account_order_no_messages()}</p>
    {:else}
      <div class="flex flex-col gap-3">
        {#each notices as n (n.id)}
          {#if n.role === 'admin'}
            <div class="flex items-start gap-3">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-blue-50 text-blue-700 mt-0.5 shrink-0">
                {m.account_order_msg_store_badge()}
              </span>
              <div class="flex-1 min-w-0 bg-blue-50/40 rounded-lg px-3 py-2">
                <p class="text-sm text-gray-900 whitespace-pre-wrap break-words">{@html renderNoticeBody(n.body)}</p>
                <p class="text-xs text-gray-400 mt-1">{fmtNoticeTime(n.created_at)}</p>
              </div>
            </div>
          {:else if n.role === 'customer'}
            <div class="flex items-start gap-3 flex-row-reverse">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-green-50 text-green-700 mt-0.5 shrink-0">
                {m.account_order_msg_you_badge()}
              </span>
              <div class="flex-1 min-w-0 bg-green-50/40 rounded-lg px-3 py-2">
                <p class="text-sm text-gray-900 whitespace-pre-wrap break-words">{@html renderNoticeBody(n.body)}</p>
                <p class="text-xs text-gray-400 mt-1 text-right">{fmtNoticeTime(n.created_at)}</p>
              </div>
            </div>
          {/if}
        {/each}
      </div>
    {/if}

    <form method="POST" action="?/sendMessage"
          use:enhance={() => {
            if (sending) return;
            sending = true;
            return async ({ update }) => {
              await update();
              sending = false;
              messageBody = '';
            };
          }}
          class="mt-6 pt-5 border-t border-gray-100 flex flex-col gap-2">
      <label for="customer-message" class="text-xs font-medium text-gray-600">{m.account_order_msg_send_label()}</label>
      <textarea id="customer-message" name="body" rows="3" bind:value={messageBody}
                placeholder={m.account_order_msg_placeholder()}
                class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
      {#if form?.error}
        <p class="text-sm text-red-500">{form.error}</p>
      {/if}
      <button type="submit" disabled={sending || !messageBody.trim()}
              class="self-end inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-gray-900 text-white
                     text-sm font-medium rounded-lg hover:bg-gray-700 transition-colors
                     disabled:opacity-50">
        {sending ? m.account_order_msg_sending() : m.account_order_msg_send()}
      </button>
    </form>
  </div>
</div>
