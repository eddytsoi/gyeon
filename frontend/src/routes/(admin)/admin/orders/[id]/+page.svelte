<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };

  const nextStatuses: Record<string, string[]> = {
    pending:    ['paid', 'cancelled'],
    paid:       ['processing', 'refunded'],
    processing: ['shipped', 'cancelled'],
    shipped:    ['delivered'],
    delivered:  ['refunded'],
    cancelled:  [],
    refunded:   [],
  };

  let updating = $state(false);
  const allowed = $derived(nextStatuses[data.order.status] ?? []);

  function formatAddress(a: NonNullable<typeof data.order.shipping_address>) {
    return [a.line1, a.line2, [a.city, a.state].filter(Boolean).join(', '), a.postal_code, a.country]
      .filter(Boolean)
      .join('\n');
  }
</script>

<svelte:head><title>Order {data.order.id.slice(0,8)} — Gyeon Admin</title></svelte:head>

<div>
  <div class="flex items-center gap-3 mb-8">
    <a href="/admin/orders" class="text-gray-400 hover:text-gray-700 transition-colors text-sm">
      ← Orders
    </a>
    <span class="text-gray-300">/</span>
    <span class="font-mono text-sm text-gray-700">{data.order.id.slice(0,8)}…</span>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">Status</p>
      <span class="inline-flex items-center px-2.5 py-1 rounded-full text-sm font-medium
                   {statusColour[data.order.status] ?? 'bg-gray-100 text-gray-500'}">
        {data.order.status}
      </span>
    </div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">Total</p>
      <p class="text-xl font-bold text-gray-900">HK${data.order.total.toFixed(2)}</p>
    </div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">Placed</p>
      <p class="text-sm font-medium text-gray-900">
        {new Date(data.order.created_at).toLocaleString('en-HK')}
      </p>
    </div>
  </div>

  <!-- Customer / Shipping cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
    <!-- Customer Info -->
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Customer</h3>
      {#if data.order.customer_name || data.order.customer_email}
        <div class="space-y-1.5 text-sm">
          {#if data.order.customer_name}
            {#if data.order.customer_id}
              <a href="/admin/customers/{data.order.customer_id}"
                 class="font-medium text-gray-900 hover:text-gray-600 transition-colors block">
                {data.order.customer_name}
              </a>
            {:else}
              <p class="font-medium text-gray-900">{data.order.customer_name}</p>
            {/if}
          {/if}
          {#if data.order.customer_email}
            <p class="text-gray-500 break-all">
              <a href="mailto:{data.order.customer_email}" class="hover:text-gray-900 transition-colors">
                {data.order.customer_email}
              </a>
            </p>
          {/if}
          {#if data.order.customer_phone}
            <p class="text-gray-500">
              <a href="tel:{data.order.customer_phone}" class="hover:text-gray-900 transition-colors">
                {data.order.customer_phone}
              </a>
            </p>
          {/if}
          {#if !data.order.customer_id}
            <p class="text-xs text-gray-400 italic pt-1">Guest checkout</p>
          {/if}
        </div>
      {:else}
        <p class="text-sm text-gray-400 italic">No customer info</p>
      {/if}
    </div>

    <!-- Shipping Info -->
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Shipping</h3>
      {#if data.order.shipping_address}
        {@const a = data.order.shipping_address}
        <div class="space-y-1.5 text-sm">
          <p class="font-medium text-gray-900">
            {[a.first_name, a.last_name].filter(Boolean).join(' ') || '—'}
          </p>
          <p class="text-gray-500 whitespace-pre-line leading-relaxed">{formatAddress(a)}</p>
          {#if a.phone}
            <p class="text-gray-500 pt-1">
              <a href="tel:{a.phone}" class="hover:text-gray-900 transition-colors">{a.phone}</a>
            </p>
          {/if}
        </div>
      {:else}
        <p class="text-sm text-gray-400 italic">No shipping address</p>
      {/if}
    </div>

  </div>

  <!-- Order items -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
    <div class="px-5 py-4 border-b border-gray-50">
      <h2 class="font-semibold text-gray-900">Items</h2>
    </div>
    <table class="w-full text-sm">
      <thead class="bg-gray-50">
        <tr>
          <th class="text-left px-5 py-3 font-medium text-gray-500">Product</th>
          <th class="text-right px-5 py-3 font-medium text-gray-500">Qty</th>
          <th class="text-right px-5 py-3 font-medium text-gray-500">Line Total</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.order.items ?? [] as item}
          <tr>
            <td class="px-5 py-3">
              <p class="font-medium text-gray-900">{item.product_name}</p>
              <p class="text-xs text-gray-400">SKU: {item.variant_sku}</p>
            </td>
            <td class="px-5 py-3 text-right text-gray-700">{item.quantity}</td>
            <td class="px-5 py-3 text-right font-medium text-gray-900">
              HK${item.line_total.toFixed(2)}
            </td>
          </tr>
        {:else}
          <tr><td colspan="3" class="px-5 py-6 text-center text-gray-400">No items</td></tr>
        {/each}
      </tbody>
      <tfoot class="border-t border-gray-100 bg-gray-50">
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">Subtotal</td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">HK${data.order.subtotal.toFixed(2)}</td>
        </tr>
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">Shipping</td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">HK${data.order.shipping_fee.toFixed(2)}</td>
        </tr>
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-bold text-gray-900">Total</td>
          <td class="px-5 py-3 text-right font-bold text-gray-900">HK${data.order.total.toFixed(2)}</td>
        </tr>
      </tfoot>
    </table>
  </div>

  <!-- Payment Info — right half on desktop, full width on mobile -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
    <div class="hidden md:block"></div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Payment</h3>
      <div class="space-y-1.5 text-sm">
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">Method</span>
          <span class="font-medium text-gray-900 capitalize">
            {data.order.payment_method ?? '—'}
          </span>
        </div>
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">Status</span>
          <span class="font-medium text-gray-900 capitalize">
            {data.order.payment_status?.replace(/_/g, ' ') ?? '—'}
          </span>
        </div>
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">Paid at</span>
          <span class="font-medium text-gray-900 text-right">
            {data.order.paid_at
              ? new Date(data.order.paid_at).toLocaleString('en-HK')
              : '—'}
          </span>
        </div>
      </div>
    </div>
  </div>

  <!-- Update status -->
  {#if allowed.length > 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <h2 class="font-semibold text-gray-900 mb-4">Update Status</h2>
      {#if form?.error}
        <p class="text-sm text-red-500 mb-3">{form.error}</p>
      {/if}
      <form method="POST" action="?/updateStatus"
            use:enhance={() => { updating = true; return async ({ update }) => { await update(); updating = false; }; }}>
        <div class="flex flex-col sm:flex-row gap-3">
          <select name="status"
                  class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                         focus:ring-2 focus:ring-gray-900 flex-1">
            {#each allowed as s}
              <option value={s}>{s}</option>
            {/each}
          </select>
          <input name="note" type="text" placeholder="Note (optional)"
                 class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                        focus:ring-2 focus:ring-gray-900 flex-1" />
          <button type="submit" disabled={updating}
                  class="px-5 py-2 bg-gray-900 text-white text-sm font-medium rounded-lg
                         hover:bg-gray-700 transition-colors disabled:opacity-50 whitespace-nowrap">
            {updating ? 'Updating…' : 'Update'}
          </button>
        </div>
      </form>
    </div>
  {/if}
</div>
