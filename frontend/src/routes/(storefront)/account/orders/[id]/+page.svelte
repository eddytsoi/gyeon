<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const order = $derived(data.order);
  const notices = $derived(data.notices ?? []);

  let sending = $state(false);
  let messageBody = $state('');

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
  <title>Order {order.order_number || `ORD-${order.number}`} — Gyeon</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <!-- Header -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <div class="flex items-start justify-between flex-wrap gap-4">
      <div>
        <a href="/account/orders" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">← Orders</a>
        <h1 class="text-xl font-bold text-gray-900 mt-1 font-mono">
          {order.order_number || `ORD-${order.number}`}
        </h1>
        <p class="text-sm text-gray-500 mt-0.5">
          Placed {new Date(order.created_at).toLocaleDateString('en-HK', { dateStyle: 'long' })}
        </p>
      </div>
      <span class="px-3 py-1.5 rounded-full text-sm font-medium capitalize border {statusColors[order.status] ?? 'bg-gray-100 text-gray-600 border-gray-200'}">
        {order.status}
      </span>
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
            <span class="text-xs text-gray-400 capitalize" style="width: 20%; text-align: center">{step}</span>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- Items -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">Items</h2>
    <div class="flex flex-col divide-y divide-gray-50">
      {#each order.items ?? [] as item}
        <div class="flex items-center justify-between py-3">
          <div class="flex-1">
            <p class="text-sm font-medium text-gray-900">{item.product_name}</p>
            <p class="text-xs text-gray-400 mt-0.5">SKU: {item.variant_sku} &middot; Qty: {item.quantity}</p>
          </div>
          <p class="text-sm font-semibold text-gray-900 ml-4">HK${item.line_total.toFixed(2)}</p>
        </div>
      {/each}
    </div>
  </div>

  <!-- Summary -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">Summary</h2>
    <div class="flex flex-col gap-2 text-sm">
      <div class="flex justify-between text-gray-600">
        <span>Subtotal</span>
        <span>HK${order.subtotal.toFixed(2)}</span>
      </div>
      {#if order.discount_amount > 0}
        <div class="flex justify-between text-green-600">
          <span>Discount</span>
          <span>−HK${order.discount_amount.toFixed(2)}</span>
        </div>
      {/if}
      <div class="flex justify-between text-gray-600">
        <span>Shipping</span>
        <span>{order.shipping_fee > 0 ? `HK$${order.shipping_fee.toFixed(2)}` : 'Free'}</span>
      </div>
      <div class="flex justify-between font-bold text-gray-900 pt-2 border-t border-gray-100 text-base">
        <span>Total</span>
        <span>HK${order.total.toFixed(2)}</span>
      </div>
    </div>
  </div>

  {#if order.notes}
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <h2 class="font-semibold text-gray-900 mb-2">Notes</h2>
      <p class="text-sm text-gray-600">{order.notes}</p>
    </div>
  {/if}

  <!-- Messages between customer and admin -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6">
    <h2 class="font-semibold text-gray-900 mb-4">Messages</h2>

    {#if notices.length === 0}
      <p class="text-sm text-gray-400 italic">No messages yet. Send one below if you have a question about this order.</p>
    {:else}
      <div class="flex flex-col gap-3">
        {#each notices as n (n.id)}
          {#if n.role === 'admin'}
            <div class="flex items-start gap-3">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-blue-50 text-blue-700 mt-0.5 shrink-0">
                Store
              </span>
              <div class="flex-1 min-w-0 bg-blue-50/40 rounded-lg px-3 py-2">
                <p class="text-sm text-gray-900 whitespace-pre-wrap break-words">{n.body}</p>
                <p class="text-xs text-gray-400 mt-1">{fmtNoticeTime(n.created_at)}</p>
              </div>
            </div>
          {:else if n.role === 'customer'}
            <div class="flex items-start gap-3 flex-row-reverse">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-green-50 text-green-700 mt-0.5 shrink-0">
                You
              </span>
              <div class="flex-1 min-w-0 bg-green-50/40 rounded-lg px-3 py-2">
                <p class="text-sm text-gray-900 whitespace-pre-wrap break-words">{n.body}</p>
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
      <label for="customer-message" class="text-xs font-medium text-gray-600">Send a message to the store</label>
      <textarea id="customer-message" name="body" rows="3" bind:value={messageBody}
                placeholder="Ask a question or share an update about this order"
                class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
      {#if form?.error}
        <p class="text-sm text-red-500">{form.error}</p>
      {/if}
      <button type="submit" disabled={sending || !messageBody.trim()}
              class="self-end inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-gray-900 text-white
                     text-sm font-medium rounded-lg hover:bg-gray-700 transition-colors
                     disabled:opacity-50">
        {sending ? 'Sending...' : 'Send'}
      </button>
    </form>
  </div>
</div>
