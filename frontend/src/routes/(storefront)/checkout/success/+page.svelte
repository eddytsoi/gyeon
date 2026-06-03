<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { formatHKD } from '$lib/money';
  import { trackPurchase } from '$lib/tracker';
  import * as m from '$lib/paraglide/messages';
  import { orderStatusLabel } from '$lib/orderStatus';
  import AppliedPromotions from '$lib/components/AppliedPromotions.svelte';
  import BankTransferNotice from '$lib/components/shop/BankTransferNotice.svelte';
  import { resolveBankTransfer } from '$lib/bankTransfer';

  let { data }: { data: PageData } = $props();

  const bankDetails = $derived(resolveBankTransfer(data.publicSettings ?? []));

  // Group order items by parent_item_id so bundle component rows render
  // indented under their bundle parent line (mirrors the account order page
  // and the PDF receipt). Without this, bundle children show up as flat
  // sibling rows and read like they were charged separately.
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

  // Cart is now cleared by the Stripe webhook (payment_intent.succeeded).
  // Refresh the local cart store so the header badge / cart page reflect
  // the empty cart as soon as the customer reaches the success page.
  onMount(() => {
    cartStore.init();
    // P3 #26 — purchase event. The success page is the natural place to fire
    // it: by the time the customer lands here Stripe has confirmed payment.
    // Bank-transfer orders are still on hold (awaiting the wire), so a
    // "purchase" conversion would be premature — skip it here.
    if (data.bankTransfer) return;
    const o = data.order;
    trackPurchase(
      o.id,
      o.total,
      (o.items ?? []).map((i: { variant_id?: string; product_name: string; unit_price: number; quantity: number }) => ({
        id: i.variant_id ?? i.product_name,
        name: i.product_name,
        price: i.unit_price,
        quantity: i.quantity
      }))
    );
  });
</script>

<svelte:head>
  <title>{m.checkout_success_title()}</title>
</svelte:head>

<div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
  <div class="text-center mb-10">
    <div class="w-16 h-16 {data.bankTransfer ? 'bg-yellow-100' : 'bg-green-100'} rounded-full flex items-center justify-center mx-auto mb-6">
      {#if data.bankTransfer}
        <svg class="w-8 h-8 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      {:else}
        <svg class="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      {/if}
    </div>
    <h1 class="text-3xl font-bold text-gray-900 mb-2">{data.bankTransfer ? m.bank_transfer_on_hold_heading() : m.checkout_success_heading()}</h1>
    <p class="text-gray-500">
      {@html m.checkout_success_body({ orderNumber: `<strong class="text-gray-900">${data.order.order_number || `ORD-${data.order.number}`}</strong>` })}
    </p>
    {#if data.bankTransfer}
      <p class="text-gray-500 mt-1">{m.bank_transfer_on_hold_body()}</p>
    {/if}
  </div>

  {#if data.bankTransfer}
    <div class="mb-6">
      <BankTransferNotice variant="plain" details={bankDetails} />
    </div>
  {/if}

  <!-- Order summary card -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">{m.checkout_success_summary()}</h2>

    <div class="flex flex-col gap-3 mb-4">
      {#each parentItems as item}
        <div class="flex flex-col gap-1">
          <div class="flex items-start justify-between gap-3 text-sm">
            <div class="min-w-0">
              <p class="font-medium text-gray-900 uppercase">{item.product_name}</p>
              {#if item.product_subtitle}
                <p class="text-xs text-gray-500 mt-0.5">{item.product_subtitle}</p>
              {/if}
              <p class="text-xs text-gray-400 mt-0.5">
                {#if item.variant_attrs && Object.keys(item.variant_attrs).length > 0}
                  {Object.values(item.variant_attrs).join(', ')} ·
                {/if}
                {m.checkout_success_qty({ quantity: item.quantity })}
              </p>
            </div>
            <span class="text-gray-900 font-medium flex-shrink-0">{formatHKD(item.line_total)}</span>
          </div>
          {#if childrenByParent[item.id]?.length}
            <ul class="mt-1 pl-4 border-l border-gray-100 flex flex-col gap-1">
              {#each childrenByParent[item.id] as child}
                <li class="flex items-start justify-between gap-3 text-xs text-gray-500">
                  <span class="min-w-0 truncate uppercase">↳ {child.product_name} <span class="text-gray-400">× {child.quantity}</span></span>
                  <span class="flex-shrink-0 text-gray-400">{m.order_item_included_in_bundle()}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}
    </div>

    <div class="border-t border-gray-100 pt-4 flex flex-col gap-2">
      <div class="flex justify-between text-sm text-gray-600">
        <span>{m.checkout_success_subtotal()}</span>
        <span>{formatHKD(data.order.subtotal)}</span>
      </div>
      {#if data.order.discount_amount > 0}
        <div class="flex justify-between text-sm text-green-600">
          <span>{m.checkout_success_discount()}</span>
          <span>−{formatHKD(data.order.discount_amount)}</span>
        </div>
      {/if}
      <div class="flex justify-between text-sm text-gray-600">
        <span>{m.checkout_success_shipping()}</span>
        <span>{data.order.shipping_free ? m.shipping_sf_free() : m.shipping_sf_cod()}</span>
      </div>
      <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900">
        <span>{m.checkout_success_total()}</span>
        <span>{formatHKD(data.order.total)}</span>
      </div>
      <AppliedPromotions promotions={data.order.applied_promotions ?? []} />
    </div>
  </div>

  <!-- Status badge -->
  <div class="bg-yellow-50 border border-yellow-100 rounded-2xl p-4 mb-8 flex items-center gap-3">
    <div class="w-2 h-2 rounded-full bg-yellow-400 flex-shrink-0"></div>
    <p class="text-sm text-yellow-800">
      {m.checkout_success_status_label()} <strong>{orderStatusLabel(data.order.status)}</strong>{m.checkout_success_status_text()}
    </p>
  </div>

  <!-- CTAs -->
  <div class="flex flex-col sm:flex-row gap-3 justify-center">
    {#if data.setupURL}
      <a href={data.setupURL}
         class="px-6 py-3 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors text-center">
        {m.checkout_success_create_account()}
      </a>
    {/if}
    <a href="/products"
       class="px-6 py-3 border border-gray-200 text-gray-700 font-medium rounded-xl hover:bg-gray-50 transition-colors text-center">
      {m.checkout_success_continue()}
    </a>
  </div>
</div>
