<script lang="ts">
  import { enhance } from '$app/forms';
  import * as m from '$lib/paraglide/messages';
  import type { ActionData, PageData } from './$types';
  import CustomerPicker, { type CustomerSelection } from '$lib/components/admin/CustomerPicker.svelte';
  import ProductPicker, { type ProductPickerAddPayload } from '$lib/components/admin/ProductPicker.svelte';
  import OrderItemsTable, { type OrderItemRow } from '$lib/components/admin/OrderItemsTable.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import { validateCoupon } from '$lib/api';
  import { adminImportOrderItemsCSV, type OrderCSVResolveItem } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';

  let { data, form }: { data: PageData & { token: string }; form: ActionData } = $props();

  // ── Customer ───────────────────────────────────────────────────────────
  let customerSel = $state<CustomerSelection>({ kind: 'none' });

  // ── Line items ─────────────────────────────────────────────────────────
  let items = $state<OrderItemRow[]>([]);
  let nextKey = 0;
  function addItem(p: ProductPickerAddPayload) {
    nextKey += 1;
    items = [
      ...items,
      {
        key: `${p.variant.id}-${nextKey}`,
        variantId: p.variant.id,
        productName: p.productName + (p.variant.name && p.productKind !== 'bundle' ? ` — ${p.variant.name}` : ''),
        sku: p.variant.sku,
        unitPrice: p.variant.price,
        quantity: p.quantity,
        primaryImageUrl: p.primaryImageUrl,
        kind: p.productKind,
        components: p.bundleItems.map((bi) => ({
          variantId: bi.component_variant_id,
          productName: bi.display_name_override || bi.component_product_name || '',
          sku: bi.component_sku || '',
          unitPrice: bi.component_price ?? 0,
          perParentQuantity: bi.quantity,
          primaryImageUrl: bi.component_primary_image_url ?? null
        }))
      }
    ];
  }
  function changeQty(key: string, qty: number) {
    items = items.map((it) => (it.key === key ? { ...it, quantity: qty } : it));
  }
  function removeItem(key: string) {
    items = items.filter((it) => it.key !== key);
  }

  function appendResolvedItem(it: OrderCSVResolveItem) {
    nextKey += 1;
    const variantSuffix =
      it.variant_name && it.product_kind !== 'bundle' ? ` — ${it.variant_name}` : '';
    items = [
      ...items,
      {
        key: `${it.variant_id}-${nextKey}`,
        variantId: it.variant_id,
        productName: it.product_name + variantSuffix,
        sku: it.sku,
        unitPrice: it.unit_price,
        quantity: it.quantity,
        primaryImageUrl: it.primary_image_url ?? null,
        kind: it.product_kind,
        components: (it.bundle_items ?? []).map((bi) => ({
          variantId: bi.component_variant_id,
          productName: bi.display_name_override || bi.component_product_name || '',
          sku: bi.component_sku || '',
          unitPrice: bi.component_price ?? 0,
          perParentQuantity: bi.quantity,
          primaryImageUrl: bi.component_primary_image_url ?? null
        }))
      }
    ];
  }

  // CSV import — hidden file input triggered by the visible Import button.
  // On pick we POST directly to the admin API (no SvelteKit form action),
  // mutate `items` client-side, and surface bad rows in an amber banner.
  let importing = $state(false);
  let importErrors = $state<{ row: number; message: string }[] | undefined>(undefined);
  let csvFileInputEl = $state<HTMLInputElement | null>(null);

  function openCSVPicker() {
    importErrors = undefined;
    csvFileInputEl?.click();
  }

  async function onCSVPicked() {
    const file = csvFileInputEl?.files?.[0];
    if (!file) return;
    importing = true;
    try {
      const result = await adminImportOrderItemsCSV(data.token, file);
      importErrors = result.errors;
      for (const it of result.items) appendResolvedItem(it);
      if (result.items.length > 0) {
        if (result.skipped > 0) {
          notify.error(
            m.admin_order_create_items_import_partial({
              ok: String(result.items.length),
              skip: String(result.skipped)
            })
          );
        } else {
          notify.success(m.admin_order_create_items_import_success({ n: String(result.items.length) }));
        }
      } else {
        notify.error(m.admin_order_create_items_import_zero_rows({ skip: String(result.skipped) }));
      }
    } catch (e) {
      notify.error(
        m.admin_order_create_items_import_failure(),
        e instanceof Error ? e.message : ''
      );
    } finally {
      importing = false;
      // Reset so picking the same file twice fires onchange again.
      if (csvFileInputEl) csvFileInputEl.value = '';
    }
  }

  // ── Shipping address ───────────────────────────────────────────────────
  let addressMode = $state<'saved' | 'new'>('new');
  let savedAddressId = $state<string | null>(null);
  let addrLine1 = $state('');
  let addrLine2 = $state('');
  let addrCity = $state('');
  let addrState = $state('');
  let addrPostal = $state('');
  let addrCountry = $state('HK');
  let saveAddressToProfile = $state(false);

  // When customer changes, pre-select the default saved address (if any).
  $effect(() => {
    if (customerSel.kind === 'existing' && customerSel.addresses.length > 0) {
      const def = customerSel.addresses.find((a) => a.is_default) ?? customerSel.addresses[0];
      addressMode = 'saved';
      savedAddressId = def.id;
    } else {
      addressMode = 'new';
      savedAddressId = null;
    }
  });

  // ── Coupon ─────────────────────────────────────────────────────────────
  let couponExpanded = $state(false);
  let couponCode = $state('');
  let couponState = $state<'idle' | 'validating' | 'valid' | 'invalid'>('idle');
  let couponDiscount = $state(0);
  let couponMessage = $state('');

  // ── Shipping fee ───────────────────────────────────────────────────────
  let useCustomShipping = $state(false);
  let shippingFeeInput = $state('0');

  // ── Notes ──────────────────────────────────────────────────────────────
  let notes = $state('');

  // ── Sidebar: status + payment-link + submit ────────────────────────────
  let initialStatus = $state<'pending' | 'processing' | 'cancelled'>('pending');
  let sendPaymentLink = $state(false);
  let submitting = $state(false);
  let clientError = $state<string | null>(null);

  // Derived totals (optimistic client-side; backend re-computes on submit)
  const subtotal = $derived(items.reduce((sum, it) => sum + it.unitPrice * it.quantity, 0));
  const discount = $derived(couponState === 'valid' ? couponDiscount : 0);
  const shippingFee = $derived.by(() => {
    if (!useCustomShipping) return 0;
    const n = parseFloat(shippingFeeInput);
    return isNaN(n) || n < 0 ? 0 : n;
  });
  const total = $derived(Math.max(0, subtotal - discount + shippingFee));
  const itemCount = $derived(items.reduce((sum, it) => sum + it.quantity, 0));

  const customerHasEmail = $derived(
    (customerSel.kind === 'existing' && !!customerSel.customer.email) ||
    (customerSel.kind === 'guest' && customerSel.email.trim() !== '')
  );
  // Auto-clear the payment-link toggle when customer email becomes unavailable.
  $effect(() => {
    if (!customerHasEmail && sendPaymentLink) sendPaymentLink = false;
  });

  async function applyCoupon() {
    const code = couponCode.trim();
    if (!code) return;
    couponState = 'validating';
    couponMessage = '';
    try {
      const role = customerSel.kind === 'existing' ? customerSel.customer.role : undefined;
      const isGuest = customerSel.kind !== 'existing';
      const res = await validateCoupon(code, subtotal, role, isGuest);
      if (res.valid) {
        couponState = 'valid';
        couponDiscount = res.discount_amount ?? 0;
      } else {
        couponState = 'invalid';
        couponMessage = (res.message_code === 'wrong_role'
          ? m.storefront_coupon_wrong_role()
          : res.message) || m.admin_order_create_discount_invalid();
        couponDiscount = 0;
      }
    } catch {
      couponState = 'invalid';
      couponMessage = m.admin_order_create_discount_invalid();
      couponDiscount = 0;
    }
  }
  function removeCoupon() {
    couponState = 'idle';
    couponCode = '';
    couponDiscount = 0;
    couponMessage = '';
  }

  // ── Submission payload ─────────────────────────────────────────────────
  function buildPayload(): Record<string, unknown> | null {
    if (customerSel.kind === 'none') {
      clientError = m.admin_order_create_customer_required();
      return null;
    }
    if (customerSel.kind === 'guest' && customerSel.email.trim() === '') {
      clientError = m.admin_order_create_customer_required();
      return null;
    }
    if (items.length === 0) {
      clientError = m.admin_order_create_items_required();
      return null;
    }

    const payload: Record<string, unknown> = {
      items: items.map((it) => ({ variant_id: it.variantId, quantity: it.quantity })),
      initial_status: initialStatus,
      email_payment_link: sendPaymentLink && customerHasEmail,
      coupon_code: couponState === 'valid' && couponCode.trim() ? couponCode.trim() : null,
      notes: notes.trim() ? notes : null,
      shipping_fee_override: useCustomShipping ? shippingFee : null
    };

    if (customerSel.kind === 'existing') {
      payload.customer_id = customerSel.customer.id;
    } else if (customerSel.kind === 'guest') {
      payload.customer_info = {
        first_name: customerSel.firstName,
        last_name: customerSel.lastName,
        email: customerSel.email,
        phone: customerSel.phone
      };
    }

    if (addressMode === 'saved' && savedAddressId) {
      payload.shipping_address_id = savedAddressId;
    } else {
      if (!addrLine1 || !addrCity || !addrPostal) {
        clientError = m.admin_order_create_address_required();
        return null;
      }
      payload.shipping_address = {
        line1: addrLine1,
        line2: addrLine2,
        city: addrCity,
        state: addrState,
        postal_code: addrPostal,
        country: addrCountry || 'HK'
      };
      if (saveAddressToProfile) payload.save_address = true;
    }

    return payload;
  }

  // Hidden input ref so we can set the serialized payload synchronously in
  // the submit handler — Svelte's reactivity flushes after the event returns,
  // which is too late for the form's own serialization.
  let bodyInputEl = $state<HTMLInputElement | null>(null);

  function onSubmitCheck(e: SubmitEvent) {
    clientError = null;
    const payload = buildPayload();
    if (!payload || !bodyInputEl) {
      e.preventDefault();
      return;
    }
    bodyInputEl.value = JSON.stringify(payload);
  }

  const serverError = $derived(form?.error ?? null);
</script>

<svelte:head><title>{m.admin_order_create_title()}</title></svelte:head>

<div>
  <div class="mb-6">
    <a href="/admin/orders" class="text-sm text-gray-500 hover:text-gray-900 transition-colors">
      {m.admin_order_create_back()}
    </a>
    <h1 class="text-2xl font-semibold text-gray-900 mt-1">{m.admin_order_create_heading()}</h1>
  </div>

  <form method="POST"
        onsubmit={onSubmitCheck}
        use:enhance={() => {
          submitting = true;
          return async ({ update }) => {
            await update();
            submitting = false;
          };
        }}
        class="grid grid-cols-1 lg:grid-cols-3 gap-6">
    <input type="hidden" name="body" bind:this={bodyInputEl} />

    <!-- ── Main column (left, 2/3) ─────────────────────────────────────── -->
    <div class="lg:col-span-2 space-y-6">

      <!-- ① Customer ────────────────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_customer_heading()}
        </h3>
        <CustomerPicker token={data.token} bind:value={customerSel} />
      </section>

      <!-- ② Line items ─────────────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            {m.admin_order_create_items_heading()}
          </h3>
          <button
            type="button"
            onclick={openCSVPicker}
            disabled={importing}
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
            </svg>
            {m.admin_order_create_items_import()}
          </button>
        </div>
        <input
          type="file"
          accept=".csv,text/csv"
          class="hidden"
          bind:this={csvFileInputEl}
          onchange={onCSVPicked}
        />
        {#if importErrors && importErrors.length > 0}
          <div class="mb-4 bg-amber-50 border border-amber-200 rounded-xl p-3 text-xs text-amber-800 space-y-1">
            <div class="font-medium">{m.admin_order_create_items_import_errors_heading({ n: String(importErrors.length) })}</div>
            <ul class="list-disc pl-5 space-y-0.5">
              {#each importErrors.slice(0, 20) as e}
                <li>Row {e.row}: {e.message}</li>
              {/each}
              {#if importErrors.length > 20}
                <li>… and {importErrors.length - 20} more</li>
              {/if}
            </ul>
          </div>
        {/if}
        <ProductPicker token={data.token} onAdd={addItem} />
        <div class="mt-4">
          <OrderItemsTable items={items} onChangeQty={changeQty} onRemove={removeItem} />
        </div>
        {#if items.length > 0}
          <p class="mt-3 text-xs text-gray-500 text-right">
            {m.admin_order_create_items_subtotal_strip({ count: String(itemCount), amount: subtotal.toFixed(2) })}
          </p>
        {/if}
      </section>

      <!-- ③ Shipping address ──────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_address_heading()}
        </h3>

        {#if customerSel.kind === 'existing' && customerSel.addresses.length > 0}
          <div class="space-y-2 mb-3">
            {#each customerSel.addresses as a (a.id)}
              <label class="block border rounded-xl px-3 py-2.5 cursor-pointer transition-colors
                            {addressMode === 'saved' && savedAddressId === a.id
                              ? 'border-gray-900 bg-gray-50'
                              : 'border-gray-200 hover:border-gray-400'}">
                <input type="radio" name="address-pick" class="sr-only"
                       checked={addressMode === 'saved' && savedAddressId === a.id}
                       onchange={() => { addressMode = 'saved'; savedAddressId = a.id; }} />
                <p class="text-sm font-medium text-gray-900">{a.first_name} {a.last_name}{a.is_default ? ' · 預設' : ''}</p>
                <p class="text-xs text-gray-500 mt-0.5">
                  {a.line1}{a.line2 ? `, ${a.line2}` : ''}, {a.city}{a.state ? `, ${a.state}` : ''} {a.postal_code} {a.country}
                </p>
              </label>
            {/each}
            <label class="block border rounded-xl px-3 py-2.5 cursor-pointer transition-colors
                          {addressMode === 'new' ? 'border-gray-900 bg-gray-50' : 'border-gray-200 hover:border-gray-400'}">
              <input type="radio" name="address-pick" class="sr-only"
                     checked={addressMode === 'new'}
                     onchange={() => { addressMode = 'new'; savedAddressId = null; }} />
              <p class="text-sm font-medium text-gray-900">{m.admin_order_create_address_use_new()}</p>
            </label>
          </div>
        {/if}

        {#if addressMode === 'new' || (customerSel.kind !== 'existing')}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 sm:col-span-2">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_line1()}</span>
              <input type="text" bind:value={addrLine1}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <label class="flex flex-col gap-1.5 sm:col-span-2">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_line2()}</span>
              <input type="text" bind:value={addrLine2}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_city()}</span>
              <input type="text" bind:value={addrCity}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_state()}</span>
              <input type="text" bind:value={addrState}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_postal()}</span>
              <input type="text" bind:value={addrPostal}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_address_country()}</span>
              <input type="text" bind:value={addrCountry} maxlength="2"
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            {#if customerSel.kind === 'existing'}
              <label class="flex items-center gap-2 sm:col-span-2 text-xs text-gray-600 mt-1">
                <input type="checkbox" bind:checked={saveAddressToProfile} class="rounded" />
                {m.admin_order_create_address_save_to_profile()}
              </label>
            {/if}
          </div>
        {/if}
      </section>

      <!-- ④ Discount ─────────────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_discount_heading()}
        </h3>
        {#if !couponExpanded && couponState !== 'valid'}
          <button type="button" onclick={() => { couponExpanded = true; }}
                  class="text-sm text-gray-700 hover:text-gray-900 transition-colors">
            {m.admin_order_create_discount_add()}
          </button>
        {:else if couponState === 'valid'}
          <div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-green-50 text-green-700 text-xs font-medium">
            <span>✓ {couponCode}</span>
            <span>{m.admin_order_create_discount_applied({ amount: couponDiscount.toFixed(2) })}</span>
            <button type="button" onclick={removeCoupon}
                    class="ml-1 text-green-700/70 hover:text-green-900 transition-colors">
              {m.admin_order_create_discount_remove()}
            </button>
          </div>
        {:else}
          <div class="flex flex-wrap items-end gap-3">
            <label class="flex flex-col gap-1.5 flex-1 min-w-[180px]">
              <span class="text-xs font-medium text-gray-600">{m.admin_order_create_discount_code_label()}</span>
              <input type="text" bind:value={couponCode}
                     class="border border-gray-200 rounded-lg px-3 py-2 text-sm uppercase focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </label>
            <button type="button" onclick={applyCoupon} disabled={couponState === 'validating' || !couponCode.trim()}
                    class="px-4 py-2 text-sm font-medium bg-gray-900 text-white rounded-lg hover:bg-gray-700 transition-colors disabled:opacity-50">
              {couponState === 'validating' ? m.admin_order_create_discount_applying() : m.admin_order_create_discount_apply()}
            </button>
          </div>
          {#if couponState === 'invalid'}
            <p class="mt-2 text-xs text-red-500">{couponMessage}</p>
          {/if}
        {/if}
      </section>

      <!-- ⑤ Shipping ─────────────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_shipping_heading()}
        </h3>
        <div class="flex items-center justify-between mb-3 text-sm">
          <span class="text-gray-500">{m.admin_order_create_shipping_method_label()}</span>
          <span class="font-medium text-gray-900">
            {m.admin_order_create_shipping_method_value()}
            <span class="ml-2 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-gray-100 text-gray-500">
              {m.admin_order_create_shipping_method_fixed()}
            </span>
          </span>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 mb-2">
          <input type="checkbox" bind:checked={useCustomShipping} class="rounded" />
          {m.admin_order_create_shipping_custom_toggle()}
        </label>
        {#if useCustomShipping}
          <label class="flex flex-col gap-1.5 max-w-[160px]">
            <span class="text-xs font-medium text-gray-600">{m.admin_order_create_shipping_fee_label()}</span>
            <input type="number" min="0" step="0.01" bind:value={shippingFeeInput}
                   class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </label>
        {:else}
          <p class="text-xs text-gray-400">{m.admin_order_create_shipping_default_hint()}</p>
        {/if}
      </section>

      <!-- ⑥ Notes ────────────────────────────────────────────────── -->
      <section class="bg-white rounded-2xl border border-gray-100 p-5">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_notes_heading()}
        </h3>
        <textarea rows="3" bind:value={notes}
                  placeholder={m.admin_order_create_notes_placeholder()}
                  class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"></textarea>
      </section>
    </div>

    <!-- ── Sidebar (right, 1/3) ───────────────────────────────────────── -->
    <aside class="lg:col-span-1">
      <div class="bg-white rounded-2xl border border-gray-100 p-5 lg:sticky lg:top-6">
        <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-4">
          {m.admin_order_create_summary_heading()}
        </h3>

        <dl class="space-y-2 text-sm">
          <div class="flex items-center justify-between">
            <dt class="text-gray-500">{m.admin_order_create_summary_subtotal()}</dt>
            <dd class="font-medium text-gray-900">HK${subtotal.toFixed(2)}</dd>
          </div>
          {#if discount > 0}
            <div class="flex items-center justify-between">
              <dt class="text-gray-500">{m.admin_order_create_summary_discount()}{couponCode ? ` (${couponCode})` : ''}</dt>
              <dd class="font-medium text-red-600">−HK${discount.toFixed(2)}</dd>
            </div>
          {/if}
          <div class="flex items-center justify-between">
            <dt class="text-gray-500">{m.admin_order_create_summary_shipping()}</dt>
            <dd class="font-medium text-gray-900">HK${shippingFee.toFixed(2)}</dd>
          </div>
          <div class="border-t border-gray-100 pt-2 flex items-center justify-between">
            <dt class="font-semibold text-gray-900">{m.admin_order_create_summary_total()}</dt>
            <dd class="text-lg font-semibold text-gray-900">HK${total.toFixed(2)}</dd>
          </div>
        </dl>

        <hr class="my-4 border-gray-100" />

        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-gray-600">{m.admin_order_create_summary_status_label()}</span>
          <select bind:value={initialStatus}
                  class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="pending">{m.order_status_pending()}</option>
            <option value="processing">{m.order_status_processing()}</option>
            <option value="cancelled">{m.order_status_cancelled()}</option>
          </select>
        </label>

        <label class="mt-3 flex items-start gap-2 text-xs">
          <input type="checkbox" bind:checked={sendPaymentLink} disabled={!customerHasEmail}
                 class="mt-0.5 rounded" />
          <span class="{customerHasEmail ? 'text-gray-700' : 'text-gray-400'}">
            {m.admin_order_create_summary_send_payment_link()}
            {#if !customerHasEmail}
              <span class="block text-[11px] text-gray-400 mt-0.5">
                {m.admin_order_create_summary_send_payment_link_disabled()}
              </span>
            {/if}
          </span>
        </label>

        {#if clientError || serverError}
          <p class="mt-3 text-xs text-red-500">{clientError ?? serverError}</p>
        {/if}

        <div class="mt-5 flex items-center gap-2">
          <a href="/admin/orders"
             class="flex-1 inline-flex items-center justify-center px-3 py-2 text-sm font-medium text-gray-700 border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors">
            {m.admin_order_create_cancel()}
          </a>
          <SaveButton loading={submitting}
                      class="flex-1 inline-flex items-center justify-center gap-1.5 px-3 py-2 text-sm font-medium bg-gray-900 text-white rounded-lg hover:bg-gray-700 transition-colors disabled:opacity-50">
            {submitting ? m.admin_order_create_submitting() : m.admin_order_create_submit()}
          </SaveButton>
        </div>
      </div>
    </aside>
  </form>
</div>
