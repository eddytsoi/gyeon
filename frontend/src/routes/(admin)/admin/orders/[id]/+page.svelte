<script lang="ts">
  import { enhance } from '$app/forms';
  import { onMount } from 'svelte';
  import type { ActionData, PageData } from './$types';
  import type { OrderItem } from '$lib/types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';
  import { orderStatusLabel } from '$lib/orderStatus';
  import { renderNoticeBody } from '$lib/orderNotice';
  import { receiptCache } from '$lib/stores/receiptCache.svelte';
  import { notify } from '$lib/stores/notifications.svelte';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const receiptStatuses = ['paid', 'processing', 'prepared', 'shipped', 'delivered'];
  const receiptReady = $derived(receiptCache.isReady(data.order.id));
  let regenerating = $state(false);
  let printing = $state(false);

  // Fetch the initial cache state on mount so the icon reflects backend
  // reality on first paint. SSE will keep it fresh from there.
  onMount(() => {
    if (!receiptStatuses.includes(data.order.status)) return;
    fetch(`/admin/orders/${data.order.id}/receipt-cache-status`, {
      headers: { Accept: 'application/json' }
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((j: { available?: boolean } | null) => {
        if (j?.available) receiptCache.set(data.order.id, true);
      })
      .catch(() => {});
  });

  async function regenerateReceiptCache() {
    if (regenerating) return;
    regenerating = true;
    // Optimistically clear the icon; the worker will re-light it via SSE.
    receiptCache.set(data.order.id, false);
    try {
      const res = await fetch(`/admin/orders/${data.order.id}/receipt-regenerate`, {
        method: 'POST'
      });
      if (!res.ok) throw new Error(await res.text());
      notify.success(m.admin_order_receipt_regenerate_done());
    } catch {
      notify.error(m.admin_order_receipt_regenerate_failed());
    } finally {
      regenerating = false;
    }
  }

  async function printReceipt() {
    if (printing) return;
    printing = true;
    try {
      const res = await fetch(`/admin/orders/${data.order.id}/receipt-print`, {
        method: 'POST'
      });
      if (!res.ok) throw new Error(await res.text());
      notify.success(m.admin_order_receipt_print_queued());
    } catch {
      notify.error(m.admin_order_receipt_print_failed());
    } finally {
      printing = false;
    }
  }

  // Group order items by parent_item_id so bundle component rows appear
  // indented under their bundle parent line. Mirrors the storefront/account
  // page and PDF receipt — without this, components appear as flat sibling
  // rows with their own line_total, which double-reads against the parent.
  const parentItems = $derived((data.order.items ?? []).filter((it) => !it.parent_item_id));
  const childrenByParent = $derived.by(() => {
    const map: Record<string, typeof data.order.items> = {};
    for (const it of data.order.items ?? []) {
      if (it.parent_item_id) {
        (map[it.parent_item_id] ??= []).push(it);
      }
    }
    return map;
  });

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    prepared:   'bg-cyan-50 text-cyan-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };

  // "prepared" (已預備) is an optional admin packing step; processing can still
  // go straight to shipped (e.g. ShipAny automation), so it offers both.
  const nextStatuses: Record<string, string[]> = {
    pending:    ['paid', 'cancelled'],
    paid:       ['processing', 'refunded'],
    processing: ['prepared', 'shipped', 'cancelled'],
    prepared:   ['shipped', 'cancelled'],
    shipped:    ['delivered'],
    delivered:  ['refunded'],
    cancelled:  [],
    refunded:   [],
  };

  // Status-change audit log (狀態變更記錄), newest-first. The "from" of each
  // change is the chronologically previous entry's status (null for the first,
  // i.e. the order's initial status). data.statusHistory arrives oldest-first.
  const statusChanges = $derived(
    (data.statusHistory ?? []).map((h, i, arr) => ({
      from: i > 0 ? arr[i - 1].status : null,
      to: h.status,
      at: h.created_at,
      operator: h.actor_email,
      note: h.note
    })).reverse()
  );

  let updating = $state(false);
  let creatingShipment = $state(false);
  let requestingPickup = $state(false);
  let syncingStatus = $state(false);
  let editingAddress = $state(false);
  let savingAddress = $state(false);
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

  // Per-line restock selection for the refund modal. Keyed by order_item id →
  // units to return to stock. Defaults to all-0 (nothing restocked); the admin
  // opts items in. Only lines with a live variant and remaining unrestocked
  // units are pickable — damaged goods are simply left at 0.
  let restockQty = $state<Record<string, number>>({});
  const restockRemaining = (it: { quantity?: number; restocked_qty?: number }) =>
    (it.quantity ?? 0) - (it.restocked_qty ?? 0);
  const isRestockable = (it: { variant_id?: string | null; quantity?: number; restocked_qty?: number }) =>
    !!it.variant_id && restockRemaining(it) > 0;
  const restockableItems = $derived((data.order.items ?? []).filter(isRestockable));
  const restockedTotal = $derived(
    (data.order.items ?? []).reduce((n, it) => n + (it.restocked_qty ?? 0), 0)
  );
  // Serialised {order_item_id, quantity} pairs (qty > 0) for the hidden field.
  const restockPayload = $derived(
    JSON.stringify(
      Object.entries(restockQty)
        .map(([order_item_id, quantity]) => ({ order_item_id, quantity: Number(quantity) || 0 }))
        .filter((x) => x.quantity > 0)
    )
  );

  function setRestockQty(id: string, val: number, max: number) {
    let n = Math.floor(val);
    if (!Number.isFinite(n) || n < 0) n = 0;
    if (n > max) n = max;
    restockQty[id] = n;
  }
  function bumpRestock(id: string, delta: number, max: number) {
    setRestockQty(id, (restockQty[id] ?? 0) + delta, max);
  }
  function restockAll() {
    const next: Record<string, number> = {};
    for (const it of restockableItems) next[it.id] = restockRemaining(it);
    restockQty = next;
  }
  function restockClear() {
    restockQty = {};
  }

  function openRefundModal() {
    refundAmount = refundableRemaining.toFixed(2);
    refundReason = '';
    restockQty = {}; // default: restock nothing — admin opts items in
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

  function formatPaymentMethod(o: { card_brand?: string; card_last4?: string; payment_method?: string }): string {
    // All card-backed payments — direct card (brand + last4), Stripe Link, and
    // the bare stripe/card gateway values from imports — read as a plain
    // "信用卡". HK shoppers don't recognise "link"/"stripe", so a uniform card
    // label is clearer than the raw value or brand/last4.
    const pm = o.payment_method?.toLowerCase();
    if ((o.card_brand && o.card_last4) || pm === 'card' || pm === 'link' || pm === 'stripe') {
      return m.payment_method_credit_card();
    }
    if (o.payment_method === 'bank_transfer') return m.bank_transfer_radio_label();
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

<svelte:head><title>{m.admin_order_detail_title({ number: data.order.order_number || `ORD-${data.order.number}` })}</title></svelte:head>

<div>
  <div class="flex flex-wrap items-center justify-between gap-3 mb-8">
    <div class="flex items-center gap-3">
      <a href="/admin/orders" class="text-gray-400 hover:text-gray-700 transition-colors text-sm">
        {m.admin_order_back()}
      </a>
      <span class="text-gray-300">/</span>
      <span class="font-mono text-sm text-gray-700">{data.order.order_number || `ORD-${data.order.number}`}</span>
    </div>
    {#if receiptStatuses.includes(data.order.status)}
      <div class="flex flex-wrap items-center gap-2">
        <a href="/admin/orders/{data.order.id}/receipt.pdf"
           target="_blank" rel="noopener"
           class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg
                  border border-gray-200 text-gray-700
                  hover:bg-gray-50 transition-colors whitespace-nowrap"
           title={receiptReady ? m.admin_order_receipt_cached_tooltip() : m.admin_order_receipt_download()}>
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
          {m.admin_order_receipt_download()}
        </a>
        <button type="button" onclick={regenerateReceiptCache} disabled={regenerating}
                class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg
                       border border-gray-200 text-gray-500
                       hover:bg-gray-50 hover:text-gray-700 transition-colors
                       disabled:opacity-50 whitespace-nowrap"
                title={m.admin_order_receipt_regenerate()}>
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="23 4 23 10 17 10"/>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
          </svg>
          {regenerating ? m.admin_order_receipt_regenerating() : m.admin_order_receipt_regenerate()}
        </button>
        <button type="button" onclick={printReceipt} disabled={printing}
                class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg
                       border border-gray-200 text-gray-500
                       hover:bg-gray-50 hover:text-gray-700 transition-colors
                       disabled:opacity-50 whitespace-nowrap"
                title={m.admin_order_receipt_print()}>
          <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 6 2 18 2 18 9"/>
            <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/>
            <rect x="6" y="14" width="12" height="8"/>
          </svg>
          {printing ? m.admin_order_receipt_printing() : m.admin_order_receipt_print()}
        </button>
      </div>
    {/if}
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 mb-8
              grid grid-cols-1 sm:grid-cols-3
              divide-y sm:divide-y-0 sm:divide-x divide-gray-100">
    <div class="p-5">
      <p class="text-xs text-gray-400 font-medium mb-1.5">{m.admin_order_card_status()}</p>
      <span class="inline-flex items-center px-2.5 py-1 rounded-full text-sm font-medium whitespace-nowrap
                   {statusColour[data.order.status] ?? 'bg-gray-100 text-gray-500'}">
        {orderStatusLabel(data.order.status)}
      </span>
    </div>
    <div class="p-5">
      <p class="text-xs text-gray-400 font-medium mb-1.5">{m.admin_order_card_total()}</p>
      <p class="text-xl font-bold text-gray-900">HK${data.order.total.toFixed(2)}</p>
    </div>
    <div class="p-5">
      <p class="text-xs text-gray-400 font-medium mb-1.5">{m.admin_order_card_placed()}</p>
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
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_order_card_shipping()}</h3>
        {#if !editingAddress}
          <button
            type="button"
            class="text-xs font-medium text-indigo-600 hover:text-indigo-800 transition-colors"
            onclick={() => (editingAddress = true)}
          >{m.admin_order_ship_edit()}</button>
        {/if}
      </div>

      {#if editingAddress}
        {@const a = data.order.shipping_address}
        <form
          method="POST"
          action="?/updateShippingAddress"
          use:enhance={() => {
            if (savingAddress) return;
            savingAddress = true;
            return async ({ result, update }) => {
              await update({ reset: false });
              savingAddress = false;
              if (result.type === 'success') editingAddress = false;
            };
          }}
          class="space-y-2.5 text-sm"
        >
          {#if data.shipment}
            <p class="rounded-lg bg-amber-50 text-amber-700 text-xs px-3 py-2 leading-relaxed">
              {m.admin_order_ship_waybill_warning()}
            </p>
          {/if}
          <div class="grid grid-cols-2 gap-2">
            <label class="block">
              <span class="text-xs text-gray-400">{m.admin_order_ship_first_name()}</span>
              <input name="first_name" value={a?.first_name ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
            </label>
            <label class="block">
              <span class="text-xs text-gray-400">{m.admin_order_ship_last_name()}</span>
              <input name="last_name" value={a?.last_name ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
            </label>
          </div>
          <label class="block">
            <span class="text-xs text-gray-400">{m.admin_order_ship_phone()}</span>
            <input name="phone" value={a?.phone ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
          </label>
          <label class="block">
            <span class="text-xs text-gray-400">{m.admin_order_ship_line1()}</span>
            <input name="line1" required value={a?.line1 ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
          </label>
          <label class="block">
            <span class="text-xs text-gray-400">{m.admin_order_ship_line2()}</span>
            <input name="line2" value={a?.line2 ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
          </label>
          <div class="grid grid-cols-2 gap-2">
            <label class="block">
              <span class="text-xs text-gray-400">{m.admin_order_ship_city()}</span>
              <input name="city" value={a?.city ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
            </label>
            <label class="block">
              <span class="text-xs text-gray-400">{m.admin_order_ship_postal()}</span>
              <input name="postal_code" value={a?.postal_code ?? ''} class="mt-0.5 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none" />
            </label>
          </div>
          <input type="hidden" name="state" value={a?.state ?? ''} />
          <input type="hidden" name="country" value={a?.country ?? 'HK'} />
          {#if form?.error}
            <p class="text-xs text-red-600">{form.error}</p>
          {/if}
          <div class="flex items-center gap-2 pt-1">
            <SaveButton loading={savingAddress}>{m.admin_order_ship_save()}</SaveButton>
            <button
              type="button"
              class="text-sm text-gray-500 hover:text-gray-800 transition-colors"
              onclick={() => (editingAddress = false)}
            >{m.admin_order_ship_cancel()}</button>
          </div>
        </form>
      {:else if data.order.shipping_address}
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
        {#each parentItems as item}
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
          {#each childrenByParent[item.id] ?? [] as child}
            <tr class="bg-gray-50/50">
              <td class="px-5 py-2 pl-10">
                <p class="text-sm text-gray-600">↳ {child.product_name}</p>
                <p class="text-xs text-gray-400">{m.admin_order_items_sku({ sku: child.variant_sku })}</p>
              </td>
              <td class="px-5 py-2 text-right text-gray-500">{child.quantity}</td>
              <td class="px-5 py-2 text-right text-gray-400 text-xs">
                {m.order_item_included_in_bundle()}
              </td>
            </tr>
          {/each}
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
        {#each data.order.applied_promotions ?? [] as promo}
          <tr>
            <td colspan="3" class="px-5 pb-2 pt-0 text-right text-xs leading-snug text-gray-500">
              <span class="text-gray-700">{promo.name}</span>{#if (promo.description ?? '').trim() !== ''}<span class="text-gray-400"> — </span>{promo.description}{/if}
            </td>
          </tr>
        {/each}
        {#if (data.order.tax_amount ?? 0) > 0}
          <tr>
            <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">{m.admin_order_items_tax()}</td>
            <td class="px-5 py-3 text-right font-medium text-gray-900">HK${(data.order.tax_amount ?? 0).toFixed(2)}</td>
          </tr>
        {/if}
        <tr>
          <td colspan="2" class="px-5 py-3 text-right text-sm font-medium text-gray-600">{m.admin_order_items_shipping()}</td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">{data.order.shipping_free ? m.shipping_sf_free() : m.shipping_sf_cod()}</td>
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
          <!-- Pull the live ShipAny state and advance the order — manual fallback
               for when the push webhook missed an event. -->
          <form method="POST" action="?/syncShipanyStatus"
                use:enhance={() => {
                  if (syncingStatus) return;
                  syncingStatus = true;
                  return async ({ update }) => { await update(); syncingStatus = false; };
                }}>
            <SaveButton loading={syncingStatus}
                    class="inline-flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-700 border border-gray-200 rounded-lg
                           hover:bg-gray-50 transition-colors disabled:opacity-50">
              {m.admin_order_shipment_sync_status()}
            </SaveButton>
          </form>
          <!-- TEMP: 預約取件 button hidden (per request 2026-05-21) -->
          {#if false}
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

    {#if data.order.notes}
      <div class="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-amber-700 mb-1">
          💬 {m.admin_order_checkout_remark_label()}
        </p>
        <p class="text-sm text-amber-900 whitespace-pre-wrap break-words">{data.order.notes}</p>
      </div>
    {/if}

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
                <p class="text-gray-700 whitespace-pre-wrap break-words">{@html renderNoticeBody(n.body)}</p>
                <p class="text-xs text-gray-400 mt-1">
                  {#if n.status}
                    <span class="{statusColour[n.status] ?? 'bg-gray-100 text-gray-500'} inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium mr-2">{orderStatusLabel(n.status)}</span>
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
                <p class="text-gray-900 whitespace-pre-wrap break-words">{@html renderNoticeBody(n.body)}</p>
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
                <p class="text-gray-900 whitespace-pre-wrap break-words">{@html renderNoticeBody(n.body)}</p>
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
          <span class="text-gray-400">{m.admin_order_payment_transaction_id()}</span>
          <span class="font-medium text-gray-900 text-right break-all">
            {data.order.transaction_id ?? m.admin_products_dash()}
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
                  <option value={s}>{orderStatusLabel(s)}</option>
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

      <!-- Action bar — matches Order Management width -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div class="hidden md:block"></div>
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
      </div>
    </form>
  {/if}

  <!-- Status Change Log (狀態變更記錄) — sits directly below Order Management
       so the status control and its audit trail read together. -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
    <div class="hidden md:block"></div>
    <div class="bg-white rounded-2xl border border-gray-100 p-5">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
        {m.admin_order_status_history_heading()}
      </h3>
      {#if statusChanges.length === 0}
        <p class="text-sm text-gray-400">{m.admin_order_status_history_empty()}</p>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-xs font-medium text-gray-500 border-b border-gray-100">
                <th class="text-left font-medium py-2 pr-3">{m.admin_order_status_history_col_change()}</th>
                <th class="text-left font-medium py-2 pr-3 whitespace-nowrap">{m.admin_order_status_history_col_time()}</th>
                <th class="text-left font-medium py-2 pr-3">{m.admin_order_status_history_col_operator()}</th>
                <th class="text-left font-medium py-2">{m.admin_order_status_history_col_note()}</th>
              </tr>
            </thead>
            <tbody>
              {#each statusChanges as c}
                <tr class="border-b border-gray-50 last:border-0 align-top">
                  <td class="py-2 pr-3">
                    <span class="inline-flex items-center gap-1.5 whitespace-nowrap">
                      {#if c.from}
                        <span class="px-2 py-0.5 rounded-md text-xs font-medium {statusColour[c.from] ?? 'bg-gray-100 text-gray-500'}">{orderStatusLabel(c.from)}</span>
                        <span class="text-gray-300">→</span>
                      {/if}
                      <span class="px-2 py-0.5 rounded-md text-xs font-medium {statusColour[c.to] ?? 'bg-gray-100 text-gray-500'}">{orderStatusLabel(c.to)}</span>
                    </span>
                  </td>
                  <td class="py-2 pr-3 text-gray-500 whitespace-nowrap">{fmtNoticeTime(c.at)}</td>
                  <td class="py-2 pr-3 text-gray-700">{c.operator ?? m.admin_order_status_history_system()}</td>
                  <td class="py-2 text-gray-700">{c.note ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>

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
          {#if restockedTotal > 0}
            <p class="text-xs text-gray-400 mb-3">{m.admin_order_refund_label_restocked({ n: restockedTotal })}</p>
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
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-lg">
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

        <!-- Restock selection — independent of the refund amount. Only ticked
             quantities go back to sellable stock; damaged goods stay at 0. -->
        <div class="border-t border-gray-100 pt-4">
          <div class="flex items-center justify-between gap-2 mb-1.5">
            <span class="block text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_order_refund_modal_restock_heading()}
            </span>
            <div class="flex gap-2">
              <button type="button" onclick={restockAll} disabled={restockableItems.length === 0}
                      class="text-xs font-medium text-gray-600 hover:text-gray-900 disabled:opacity-40 disabled:hover:text-gray-600">
                {m.admin_order_refund_modal_restock_all()}
              </button>
              <span class="text-gray-200">|</span>
              <button type="button" onclick={restockClear}
                      class="text-xs font-medium text-gray-600 hover:text-gray-900">
                {m.admin_order_refund_modal_restock_clear()}
              </button>
            </div>
          </div>
          <p class="text-xs text-gray-400 mb-2">{m.admin_order_refund_modal_restock_help()}</p>
          <div class="max-h-60 overflow-y-auto rounded-xl border border-gray-100 divide-y divide-gray-50">
            {#each parentItems as item}
              {#if (childrenByParent[item.id] ?? []).length > 0}
                <div class="px-3 py-2 bg-gray-50/60">
                  <p class="text-sm font-medium text-gray-700">{item.product_name}</p>
                </div>
                {#each childrenByParent[item.id] ?? [] as child}
                  {@render restockRow(child, true)}
                {/each}
              {:else}
                {@render restockRow(item, false)}
              {/if}
            {:else}
              <p class="px-3 py-4 text-center text-xs text-gray-400">{m.admin_order_items_empty()}</p>
            {/each}
          </div>
        </div>
        <input type="hidden" name="restock" value={restockPayload} />

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

{#snippet restockRow(item: OrderItem, indented: boolean)}
  {@const remaining = restockRemaining(item)}
  <div class="flex items-center gap-3 px-3 py-2 {indented ? 'pl-8' : ''}">
    <div class="min-w-0 flex-1">
      <p class="text-sm text-gray-800 truncate">{indented ? '↳ ' : ''}{item.product_name}</p>
      <p class="text-xs text-gray-400">{m.admin_order_items_sku({ sku: item.variant_sku })}</p>
    </div>
    {#if !item.variant_id}
      <span class="text-xs text-gray-400 whitespace-nowrap">{m.admin_order_refund_modal_restock_deleted()}</span>
    {:else if remaining <= 0}
      <span class="text-xs text-emerald-600 whitespace-nowrap">{m.admin_order_refund_modal_restock_done()}</span>
    {:else}
      <div class="flex items-center gap-2">
        {#if (item.restocked_qty ?? 0) > 0}
          <span class="text-xs text-gray-400 whitespace-nowrap">{m.admin_order_refund_modal_restock_already({ n: item.restocked_qty ?? 0, total: item.quantity })}</span>
        {/if}
        <div class="flex items-center">
          <button type="button" onclick={() => bumpRestock(item.id, -1, remaining)}
                  class="h-8 w-8 rounded-l-lg border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-40"
                  disabled={(restockQty[item.id] ?? 0) <= 0} aria-label="−">−</button>
          <input type="number" min="0" max={remaining} value={restockQty[item.id] ?? 0}
                 oninput={(e) => setRestockQty(item.id, e.currentTarget.valueAsNumber, remaining)}
                 class="h-8 w-12 border-y border-gray-200 text-center text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900 [appearance:textfield]
                        [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none" />
          <button type="button" onclick={() => bumpRestock(item.id, 1, remaining)}
                  class="h-8 w-8 rounded-r-lg border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-40"
                  disabled={(restockQty[item.id] ?? 0) >= remaining} aria-label="+">+</button>
        </div>
        <span class="text-xs text-gray-400 whitespace-nowrap">{m.admin_order_refund_modal_restock_of({ total: remaining })}</span>
      </div>
    {/if}
  </div>
{/snippet}
