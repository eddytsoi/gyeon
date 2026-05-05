<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

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
  let addingNote = $state(false);
  let sendingMessage = $state(false);
  let refunding = $state(false);
  let showRefundModal = $state(false);
  let refundAmount = $state('');
  let refundReason = $state('');
  const refundedAmount = $derived(data.order.refund_amount ?? 0);
  const refundableRemaining = $derived(Math.max(0, data.order.total - refundedAmount));
  const canRefund = $derived(
    refundableRemaining > 0 &&
    ['paid', 'processing', 'shipped', 'delivered'].includes(data.order.status)
  );

  function openRefundModal() {
    refundAmount = refundableRemaining.toFixed(2);
    refundReason = '';
    showRefundModal = true;
  }
  let internalNoteBody = $state('');
  let adminMessageBody = $state('');
  const allowed = $derived(nextStatuses[data.order.status] ?? []);

  function fmtNoticeTime(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleString('en-HK', { dateStyle: 'medium', timeStyle: 'short' });
  }

  // Carrier override fields shown when an order pre-dates ShipAny enablement.
  // Fall back to Logistics defaults from site settings so admins don't have to
  // re-type the same courier UID and service plan on every legacy order.
  let carrierOverride = $state(data.order.selected_carrier || data.defaultCarrier || '');
  let serviceOverride = $state(data.order.selected_service || data.defaultService || '');

  // Build a uid → courier map so we can show human-readable courier names while
  // still submitting cour_uid to the ShipAny API.
  const courierByUid = $derived(new Map((data.couriers ?? []).map((c) => [c.uid, c])));
  const courierLabel = (uid: string | null | undefined) => {
    if (!uid) return m.admin_products_dash();
    return courierByUid.get(uid)?.name ?? uid;
  };

  const BRAND_LABELS: Record<string, string> = {
    visa: 'Visa', mastercard: 'Mastercard', amex: 'Amex', discover: 'Discover',
    jcb: 'JCB', diners: 'Diners', unionpay: 'UnionPay', unknown: 'Card'
  };
  function formatPaymentMethod(o: { card_brand?: string; card_last4?: string; payment_method?: string }): string {
    if (o.card_brand && o.card_last4) {
      const label = BRAND_LABELS[o.card_brand.toLowerCase()] ?? o.card_brand;
      return `${label} •••• ${o.card_last4}`;
    }
    if (o.payment_method) return o.payment_method;
    return m.admin_products_dash();
  }
  const selectedCourierPlans = $derived(
    courierByUid.get(carrierOverride)?.cour_svc_plans ?? []
  );

  function onCarrierChange(uid: string) {
    carrierOverride = uid;
    const plans = courierByUid.get(uid)?.cour_svc_plans ?? [];
    if (!plans.some((p) => p.cour_svc_pl === serviceOverride)) {
      serviceOverride = '';
    }
  }

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
      {m.admin_order_back()}
    </a>
    <span class="text-gray-300">/</span>
    <span class="font-mono text-sm text-gray-700">{data.order.order_number || `ORD-${data.order.number}`}</span>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">{m.admin_order_card_status()}</p>
      <span class="inline-flex items-center px-2.5 py-1 rounded-full text-sm font-medium
                   {statusColour[data.order.status] ?? 'bg-gray-100 text-gray-500'}">
        {data.order.status}
      </span>
    </div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">{m.admin_order_card_total()}</p>
      <p class="text-xl font-bold text-gray-900">HK${data.order.total.toFixed(2)}</p>
    </div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <p class="text-xs text-gray-400 font-medium mb-1">{m.admin_order_card_placed()}</p>
      <p class="text-sm font-medium text-gray-900">
        {new Date(data.order.created_at).toLocaleString('en-HK')}
      </p>
    </div>
  </div>

  <!-- Customer / Shipping cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
    <!-- Customer Info -->
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.admin_order_card_customer()}</h3>
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
            <p class="text-xs text-gray-400 italic pt-1">{m.admin_order_card_guest()}</p>
          {/if}
        </div>
      {:else}
        <p class="text-sm text-gray-400 italic">{m.admin_order_card_no_customer()}</p>
      {/if}
    </div>

    <!-- Shipping Info -->
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.admin_order_card_shipping()}</h3>
      {#if data.order.shipping_address}
        {@const a = data.order.shipping_address}
        <div class="space-y-1.5 text-sm">
          <p class="font-medium text-gray-900">
            {[a.first_name, a.last_name].filter(Boolean).join(' ') || m.admin_products_dash()}
          </p>
          <p class="text-gray-500 whitespace-pre-line leading-relaxed">{formatAddress(a)}</p>
          {#if a.phone}
            <p class="text-gray-500 pt-1">
              <a href="tel:{a.phone}" class="hover:text-gray-900 transition-colors">{a.phone}</a>
            </p>
          {/if}
        </div>
      {:else}
        <p class="text-sm text-gray-400 italic">{m.admin_order_card_no_shipping()}</p>
      {/if}
    </div>

  </div>

  <!-- Order items -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden mb-6">
    <div class="px-5 py-4 border-b border-gray-50">
      <h2 class="font-semibold text-gray-900">{m.admin_order_items_heading()}</h2>
    </div>
    <table class="w-full text-sm">
      <thead class="bg-gray-50">
        <tr>
          <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_order_items_col_product()}</th>
          <th class="text-right px-5 py-3 font-medium text-gray-500">{m.admin_order_items_col_qty()}</th>
          <th class="text-right px-5 py-3 font-medium text-gray-500">{m.admin_order_items_col_line_total()}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.order.items ?? [] as item}
          <tr>
            <td class="px-5 py-3">
              <p class="font-medium text-gray-900">{item.product_name}</p>
              <p class="text-xs text-gray-400">{m.admin_order_items_sku({ sku: item.variant_sku })}</p>
            </td>
            <td class="px-5 py-3 text-right text-gray-700">{item.quantity}</td>
            <td class="px-5 py-3 text-right font-medium text-gray-900">
              HK${item.line_total.toFixed(2)}
            </td>
          </tr>
        {:else}
          <tr><td colspan="3" class="px-5 py-6 text-center text-gray-400">{m.admin_order_items_empty()}</td></tr>
        {/each}
      </tbody>
      <tfoot class="border-t border-gray-100 bg-gray-50">
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">{m.admin_order_items_subtotal()}</td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">HK${data.order.subtotal.toFixed(2)}</td>
        </tr>
        {#if data.order.discount_amount > 0}
          <tr>
            <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-emerald-600">{m.admin_order_items_discount()}</td>
            <td class="px-5 py-3 text-right font-medium text-emerald-600">-HK${data.order.discount_amount.toFixed(2)}</td>
          </tr>
        {/if}
        {#if (data.order.tax_amount ?? 0) > 0}
          <tr>
            <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">{m.admin_order_items_tax()}</td>
            <td class="px-5 py-3 text-right font-medium text-gray-900">HK${(data.order.tax_amount ?? 0).toFixed(2)}</td>
          </tr>
        {/if}
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">{m.admin_order_items_shipping()}</td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">HK${data.order.shipping_fee.toFixed(2)}</td>
        </tr>
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-bold text-gray-900">{m.admin_order_items_total()}</td>
          <td class="px-5 py-3 text-right font-bold text-gray-900">HK${data.order.total.toFixed(2)}</td>
        </tr>
      </tfoot>
    </table>
  </div>

  <!-- ShipAny shipment card -->
  {#if data.shipment || canCreateShipment}
    <div class="bg-white rounded-2xl border border-gray-100 p-5 mb-6">
      <div class="flex items-center justify-between gap-3 mb-3">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_order_shipment_heading()}</h3>
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
            <span class="text-gray-400">{m.admin_order_shipment_carrier()}</span>
            <span class="font-medium text-gray-900 text-right">{courierLabel(s.carrier)}</span>
          </div>
          <div class="flex justify-between gap-2">
            <span class="text-gray-400">{m.admin_order_shipment_service()}</span>
            <span class="font-medium text-gray-900 text-right">{s.service}</span>
          </div>
          {#if s.tracking_number}
            <div class="flex justify-between gap-2">
              <span class="text-gray-400">{m.admin_order_shipment_tracking()}</span>
              <a href={s.tracking_url ?? '#'} target="_blank" rel="noopener"
                 class="font-mono text-gray-900 hover:text-gray-600 transition-colors text-right">
                {s.tracking_number} ↗
              </a>
            </div>
          {/if}
          <div class="flex justify-between gap-2">
            <span class="text-gray-400">{m.admin_order_shipment_fee()}</span>
            <span class="font-medium text-gray-900 text-right">HK${s.fee_hkd.toFixed(2)}</span>
          </div>
        </div>

        <div class="flex gap-2 pt-4 mt-4 border-t border-gray-100">
          {#if s.label_url}
            <a href={s.label_url} target="_blank" rel="noopener"
               class="px-3 py-2 text-xs font-medium text-gray-700 border border-gray-200 rounded-lg
                      hover:bg-gray-50 transition-colors">
              {m.admin_order_shipment_download_waybill()}
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
                {m.admin_order_shipment_request_pickup()}
              </SaveButton>
            </form>
          {:else}
            <span class="px-3 py-2 text-xs text-gray-400">{m.admin_order_shipment_pickup_already()}</span>
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
              {m.admin_order_shipment_legacy_intro()}
            </p>
            <div class="grid grid-cols-2 gap-2">
              {#if (data.couriers?.length ?? 0) > 0}
                <div class="flex flex-col gap-1">
                  <label for="carrier-select" class="text-xs font-medium text-gray-600">{m.admin_order_shipment_label_courier()}</label>
                  <select id="carrier-select" name="carrier"
                          value={carrierOverride}
                          onchange={(e) => onCarrierChange(e.currentTarget.value)}
                          class="border border-gray-200 rounded-lg px-3 py-2 text-sm bg-white
                                 focus:outline-none focus:ring-2 focus:ring-gray-900" required>
                    <option value="" disabled>{m.admin_order_shipment_select_courier()}</option>
                    {#each data.couriers as c}
                      <option value={c.uid}>{c.name}</option>
                    {/each}
                    {#if carrierOverride && !courierByUid.has(carrierOverride)}
                      <option value={carrierOverride}>{carrierOverride}</option>
                    {/if}
                  </select>
                </div>
                <div class="flex flex-col gap-1">
                  <label for="service-select" class="text-xs font-medium text-gray-600">{m.admin_order_shipment_label_service_plan()}</label>
                  {#if selectedCourierPlans.length > 0}
                    <select id="service-select" name="service" bind:value={serviceOverride}
                            class="border border-gray-200 rounded-lg px-3 py-2 text-sm bg-white
                                   focus:outline-none focus:ring-2 focus:ring-gray-900" required>
                      <option value="" disabled>{m.admin_order_shipment_select_service()}</option>
                      {#each selectedCourierPlans as p}
                        <option value={p.cour_svc_pl}>{p.cour_svc_pl}</option>
                      {/each}
                    </select>
                  {:else}
                    <input id="service-select" name="service" bind:value={serviceOverride}
                           placeholder={m.admin_order_shipment_service_placeholder()}
                           class="border border-gray-200 rounded-lg px-3 py-2 text-sm
                                  focus:outline-none focus:ring-2 focus:ring-gray-900" required />
                  {/if}
                </div>
              {:else}
                <div class="flex flex-col gap-1">
                  <label for="carrier-input" class="text-xs font-medium text-gray-600">{m.admin_order_shipment_label_courier()}</label>
                  <input id="carrier-input" name="carrier" bind:value={carrierOverride}
                         placeholder={m.admin_order_shipment_uid_placeholder()}
                         class="border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono
                                focus:outline-none focus:ring-2 focus:ring-gray-900" required />
                </div>
                <div class="flex flex-col gap-1">
                  <label for="service-input" class="text-xs font-medium text-gray-600">{m.admin_order_shipment_label_service_plan()}</label>
                  <input id="service-input" name="service" bind:value={serviceOverride}
                         placeholder={m.admin_order_shipment_service_placeholder()}
                         class="border border-gray-200 rounded-lg px-3 py-2 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" required />
                </div>
              {/if}
            </div>
          {:else}
            <p class="text-xs text-gray-500">
              {m.admin_order_shipment_customer_chose()}<span class="font-medium text-gray-700">{courierLabel(data.order.selected_carrier)} / {data.order.selected_service}</span>
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
            {m.admin_order_shipment_create()}
          </SaveButton>
        </form>
      {/if}
    </div>
  {/if}

  <!-- Notices: system events + admin/customer messages timeline -->
  <div class="bg-white rounded-2xl border border-gray-100 p-5 mb-6">
    <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">{m.admin_order_notices_heading()}</h3>

    {#if (data.notices?.length ?? 0) === 0}
      <p class="text-sm text-gray-400 italic">{m.admin_order_notices_empty()}</p>
    {:else}
      <div class="flex flex-col gap-3">
        {#each data.notices as n (n.id)}
          {#if n.role === 'system'}
            <div class="flex items-start gap-3 text-sm">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-gray-100 text-gray-500 mt-0.5 shrink-0">
                {m.admin_order_notices_system_badge()}
              </span>
              <div class="flex-1 min-w-0">
                <p class="text-gray-700 whitespace-pre-wrap break-words">{n.body}</p>
                <p class="text-xs text-gray-400 mt-1">
                  {#if n.status}
                    <span class="capitalize {statusColour[n.status] ?? 'bg-gray-100 text-gray-500'} inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium mr-2">{n.status}</span>
                  {/if}
                  {fmtNoticeTime(n.created_at)}
                </p>
              </div>
            </div>
          {:else if n.role === 'admin'}
            <div class="flex items-start gap-3 text-sm">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-blue-50 text-blue-700 mt-0.5 shrink-0">
                {m.admin_order_notices_admin_badge()}
              </span>
              <div class="flex-1 min-w-0">
                <p class="text-gray-900 whitespace-pre-wrap break-words">{n.body}</p>
                <p class="text-xs text-gray-400 mt-1">
                  {fmtNoticeTime(n.created_at)}
                  {#if !n.read_at}
                    <span class="ml-2 text-amber-600">{m.admin_order_notices_unread_marker()}</span>
                  {/if}
                </p>
              </div>
            </div>
          {:else}
            <div class="flex items-start gap-3 text-sm">
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide bg-green-50 text-green-700 mt-0.5 shrink-0">
                {m.admin_order_notices_customer_badge()}
              </span>
              <div class="flex-1 min-w-0">
                <p class="text-gray-900 whitespace-pre-wrap break-words">{n.body}</p>
                <p class="text-xs text-gray-400 mt-1">{fmtNoticeTime(n.created_at)}</p>
              </div>
            </div>
          {/if}
        {/each}
      </div>
    {/if}

    <div class="mt-6 pt-5 border-t border-gray-100 grid grid-cols-1 md:grid-cols-2 gap-4">
      <form method="POST" action="?/addInternalNote"
            use:enhance={() => {
              if (addingNote) return;
              addingNote = true;
              return async ({ update }) => {
                await update();
                addingNote = false;
                internalNoteBody = '';
              };
            }}
            class="flex flex-col gap-2">
        <label for="internal-note" class="text-xs font-medium text-gray-600">{m.admin_order_internal_note_label()}</label>
        <textarea id="internal-note" name="body" rows="3" bind:value={internalNoteBody}
                  placeholder={m.admin_order_internal_note_placeholder()}
                  class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
        <SaveButton loading={addingNote}
                class="self-start inline-flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium
                       text-gray-700 border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors
                       disabled:opacity-50">
          {m.admin_order_add_note()}
        </SaveButton>
      </form>

      <form method="POST" action="?/sendAdminMessage"
            use:enhance={() => {
              if (sendingMessage) return;
              sendingMessage = true;
              return async ({ update }) => {
                await update();
                sendingMessage = false;
                adminMessageBody = '';
              };
            }}
            class="flex flex-col gap-2">
        <label for="admin-message" class="text-xs font-medium text-gray-600">{m.admin_order_reply_label()}</label>
        <textarea id="admin-message" name="body" rows="3" bind:value={adminMessageBody}
                  placeholder={m.admin_order_reply_placeholder()}
                  class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900 resize-y"></textarea>
        <SaveButton loading={sendingMessage}
                class="self-start inline-flex items-center justify-center gap-1.5 px-3 py-2 bg-gray-900 text-white
                       text-xs font-medium rounded-lg hover:bg-gray-700 transition-colors
                       disabled:opacity-50">
          {m.admin_order_reply_send()}
        </SaveButton>
      </form>
    </div>
  </div>

  <!-- Payment Info — right half on desktop, full width on mobile -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
    <div class="hidden md:block"></div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.admin_order_payment_heading()}</h3>
      <div class="space-y-1.5 text-sm">
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">{m.admin_order_payment_method()}</span>
          <span class="font-medium text-gray-900 capitalize">
            {formatPaymentMethod(data.order)}
          </span>
        </div>
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">{m.admin_order_payment_status()}</span>
          <span class="font-medium text-gray-900 capitalize">
            {data.order.payment_status?.replace(/_/g, ' ') ?? m.admin_products_dash()}
          </span>
        </div>
        <div class="flex justify-between gap-2">
          <span class="text-gray-400">{m.admin_order_payment_paid_at()}</span>
          <span class="font-medium text-gray-900 text-right">
            {data.order.paid_at
              ? new Date(data.order.paid_at).toLocaleString('en-HK')
              : m.admin_products_dash()}
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
          <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">{m.admin_order_management_heading()}</h3>
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="status-select" class="text-xs font-medium text-gray-600">{m.admin_order_status_label()}</label>
              <select id="status-select" name="status"
                      class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                             focus:ring-2 focus:ring-gray-900">
                {#each allowed as s}
                  <option value={s}>{s}</option>
                {/each}
              </select>
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="status-note" class="text-xs font-medium text-gray-600">{m.admin_order_note_label()}</label>
              <input id="status-note" name="note" type="text" placeholder={m.admin_order_note_placeholder()}
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
          {m.admin_order_update()}
        </SaveButton>
      </div>
    </form>
  {/if}

  <!-- Refund section -->
  {#if refundedAmount > 0 || canRefund}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-6">
      <div class="hidden md:block"></div>
      <div class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">{m.admin_order_refund_heading()}</h3>
        {#if refundedAmount > 0}
          <div class="flex items-center justify-between mb-3 text-sm">
            <span class="text-gray-500">{m.admin_order_refund_label_refunded()}</span>
            <span class="font-mono font-semibold text-red-600">HK${refundedAmount.toFixed(2)} / HK${data.order.total.toFixed(2)}</span>
          </div>
          {#if data.order.refund_reason}
            <p class="text-xs text-gray-400 mb-3">{m.admin_order_refund_label_reason()}: {data.order.refund_reason}</p>
          {/if}
        {/if}
        {#if canRefund}
          <button type="button" onclick={openRefundModal}
                  class="w-full px-4 py-2.5 rounded-xl border border-red-200 text-sm font-medium
                         text-red-600 hover:bg-red-50 transition-colors">
            {refundedAmount > 0 ? m.admin_order_refund_issue_more() : m.admin_order_refund_issue()}
          </button>
        {:else if refundedAmount > 0}
          <p class="text-xs text-gray-400">{m.admin_order_refund_fully_refunded()}</p>
        {/if}
      </div>
    </div>
  {/if}
</div>

{#if showRefundModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => (showRefundModal = false)} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-md">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_order_refund_modal_title()}</h3>
      <p class="text-sm text-gray-500 mb-4">{m.admin_order_refund_modal_warning()}</p>

      <form method="POST" action="?/refund" class="space-y-4"
            use:enhance={() => {
              if (refunding) return;
              refunding = true;
              return async ({ result, update }) => {
                await update();
                refunding = false;
                if (result.type === 'success') showRefundModal = false;
              };
            }}>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_order_refund_modal_amount()}
          </label>
          <div class="flex items-center">
            <span class="px-3 py-2.5 bg-gray-50 border border-r-0 border-gray-200 rounded-l-xl text-sm text-gray-400 select-none">HK$</span>
            <input type="number" name="amount" bind:value={refundAmount}
                   min="0.01" max={refundableRemaining} step="0.01" required
                   class="w-full flex-1 px-3.5 py-2.5 border border-gray-200 rounded-r-xl text-sm font-mono
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <p class="text-xs text-gray-400 mt-1">
            {m.admin_order_refund_modal_remaining({ amount: refundableRemaining.toFixed(2) })}
          </p>
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_order_refund_modal_reason()}
          </label>
          <textarea name="reason" bind:value={refundReason} rows="3"
                    class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm resize-none
                           focus:outline-none focus:ring-2 focus:ring-gray-900"></textarea>
        </div>
        <div class="flex gap-3 pt-2">
          <button type="button" onclick={() => (showRefundModal = false)}
                  class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                         text-gray-700 hover:bg-gray-50 transition-colors">
            {m.common_cancel()}
          </button>
          <SaveButton loading={refunding}
                      class="flex-1 inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl
                             bg-red-500 text-white text-sm font-medium hover:bg-red-600 transition-colors disabled:opacity-50">
            {m.admin_order_refund_modal_submit()}
          </SaveButton>
        </div>
      </form>
    </div>
  </div>
{/if}
