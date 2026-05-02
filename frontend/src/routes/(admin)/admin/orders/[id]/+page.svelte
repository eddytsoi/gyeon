<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';

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
  let creatingShipment = $state(false);
  let requestingPickup = $state(false);
  const allowed = $derived(nextStatuses[data.order.status] ?? []);

  // Carrier override fields shown when an order pre-dates ShipAny enablement
  let carrierOverride = $state(data.order.selected_carrier ?? '');
  let serviceOverride = $state(data.order.selected_service ?? '');

  const canCreateShipment = $derived(
    !data.shipment &&
    (data.order.status === 'paid' || data.order.status === 'processing')
  );
  const pickupRequested = $derived(
    data.shipment?.status !== 'created'
  );

  function formatAddress(a: NonNullable<typeof data.order.shipping_address>) {
    return [a.line1, a.line2, [a.city, a.state].filter(Boolean).join(', '), a.postal_code, a.country]
      .filter(Boolean)
      .join('\n');
  }
</script>

<svelte:head><title>{data.order.order_number || `ORD-${data.order.number}`} — Gyeon Admin</title></svelte:head>

<div>
  <div class="flex items-center gap-3 mb-8">
    <a href="/admin/orders" class="text-gray-400 hover:text-gray-700 transition-colors text-sm">
      ← Orders
    </a>
    <span class="text-gray-300">/</span>
    <span class="font-mono text-sm text-gray-700">{data.order.order_number || `ORD-${data.order.number}`}</span>
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

  <!-- ShipAny shipment card -->
  {#if data.shipment || canCreateShipment}
    <div class="bg-white rounded-2xl border border-gray-100 p-5 mb-6">
      <div class="flex items-center justify-between gap-3 mb-3">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide">Shipment</h3>
        {#if data.shipment}
          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                       {data.shipment.status === 'delivered'
                         ? 'bg-green-50 text-green-700'
                         : data.shipment.status === 'in_transit'
                         ? 'bg-violet-50 text-violet-700'
                         : data.shipment.status === 'exception'
                         ? 'bg-red-50 text-red-700'
                         : 'bg-gray-100 text-gray-500'}">
            {data.shipment.status}
          </span>
        {/if}
      </div>

      {#if data.shipment}
        {@const s = data.shipment}
        <div class="space-y-2 text-sm">
          <div class="flex justify-between gap-2">
            <span class="text-gray-400">Carrier</span>
            <span class="font-medium text-gray-900 text-right">{s.carrier}</span>
          </div>
          <div class="flex justify-between gap-2">
            <span class="text-gray-400">Service</span>
            <span class="font-medium text-gray-900 text-right">{s.service}</span>
          </div>
          {#if s.tracking_number}
            <div class="flex justify-between gap-2">
              <span class="text-gray-400">Tracking #</span>
              <a href={s.tracking_url ?? '#'} target="_blank" rel="noopener"
                 class="font-mono text-gray-900 hover:text-gray-600 transition-colors text-right">
                {s.tracking_number} ↗
              </a>
            </div>
          {/if}
          <div class="flex justify-between gap-2">
            <span class="text-gray-400">Fee</span>
            <span class="font-medium text-gray-900 text-right">HK${s.fee_hkd.toFixed(2)}</span>
          </div>
        </div>

        <div class="flex gap-2 pt-4 mt-4 border-t border-gray-100">
          {#if s.label_url}
            <a href={s.label_url} target="_blank" rel="noopener"
               class="px-3 py-2 text-xs font-medium text-gray-700 border border-gray-200 rounded-lg
                      hover:bg-gray-50 transition-colors">
              Download Waybill PDF
            </a>
          {/if}
          {#if !pickupRequested}
            <form method="POST" action="?/requestPickup"
                  use:enhance={() => {
                    if (requestingPickup) return;
                    requestingPickup = true;
                    return async ({ update }) => { await update(); requestingPickup = false; };
                  }}>
              <SaveButton loading={requestingPickup}
                      class="inline-flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-700 border border-gray-200 rounded-lg
                             hover:bg-gray-50 transition-colors disabled:opacity-50">
                Request Pickup
              </SaveButton>
            </form>
          {:else}
            <span class="px-3 py-2 text-xs text-gray-400">Pickup already requested</span>
          {/if}
        </div>
      {:else}
        <!-- No shipment yet → Create button -->
        <form method="POST" action="?/createShipment"
              use:enhance={() => {
                if (creatingShipment) return;
                creatingShipment = true;
                return async ({ update }) => { await update(); creatingShipment = false; };
              }}
              class="flex flex-col gap-3">
          {#if !data.order.selected_carrier}
            <p class="text-xs text-gray-500 leading-relaxed">
              This order pre-dates ShipAny enablement and has no carrier selected.
              Pick one to create a shipment.
            </p>
            <div class="grid grid-cols-2 gap-2">
              <input name="carrier" bind:value={carrierOverride}
                     placeholder="cour_uid (UUID)"
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono
                            focus:outline-none focus:ring-2 focus:ring-gray-900" required />
              <input name="service" bind:value={serviceOverride}
                     placeholder="service plan name"
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" required />
            </div>
          {:else}
            <p class="text-xs text-gray-500">
              Customer chose: <span class="font-medium text-gray-700">{data.order.selected_carrier} / {data.order.selected_service}</span>
              {#if data.order.pickup_point_label}<br /><span class="text-gray-400">{data.order.pickup_point_label}</span>{/if}
            </p>
          {/if}

          {#if form?.error}
            <p class="text-sm text-red-500">{form.error}</p>
          {/if}

          <SaveButton loading={creatingShipment}
                  class="self-start inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-gray-900
                         text-white text-sm font-medium rounded-lg hover:bg-gray-700 transition-colors
                         disabled:opacity-50">
            Create Shipment
          </SaveButton>
        </form>
      {/if}
    </div>
  {/if}

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

  <!-- Order Management (status + note) + action bar share one form -->
  {#if allowed.length > 0}
    <form method="POST" action="?/updateStatus"
          use:enhance={() => {
            if (updating) return;
            updating = true;
            return async ({ update }) => { await update(); updating = false; };
          }}>
      <!-- Status & Notes — right half, matches Payment Info width -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div class="hidden md:block"></div>
        <div class="bg-white rounded-2xl border border-gray-100 p-5">
          <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">Order Management</h3>
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="status-select" class="text-xs font-medium text-gray-600">Status</label>
              <select id="status-select" name="status"
                      class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                             focus:ring-2 focus:ring-gray-900">
                {#each allowed as s}
                  <option value={s}>{s}</option>
                {/each}
              </select>
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="status-note" class="text-xs font-medium text-gray-600">Note</label>
              <input id="status-note" name="note" type="text" placeholder="Optional"
                     class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                            focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>
      </div>

      <!-- Action bar -->
      <div class="bg-white rounded-2xl border border-gray-100 p-4 flex items-center justify-end gap-4">
        {#if form?.error}
          <p class="text-sm text-red-500 mr-auto">{form.error}</p>
        {/if}
        <SaveButton loading={updating}
                class="inline-flex items-center justify-center gap-1.5 px-5 py-2 bg-gray-900 text-white
                       text-sm font-medium rounded-lg hover:bg-gray-700 transition-colors
                       disabled:opacity-50 whitespace-nowrap">
          Update
        </SaveButton>
      </div>
    </form>
  {/if}
</div>
