<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { cartStore } from '$lib/stores/cart.svelte';

  let { data }: { data: PageData } = $props();

  // Cart is now cleared by the Stripe webhook (payment_intent.succeeded).
  // Refresh the local cart store so the header badge / cart page reflect
  // the empty cart as soon as the customer reaches the success page.
  onMount(() => { cartStore.init(); });
</script>

<svelte:head>
  <title>Order Confirmed — Gyeon</title>
</svelte:head>

<div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
  <div class="text-center mb-10">
    <div class="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-6">
      <svg class="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>
    </div>
    <h1 class="text-3xl font-bold text-gray-900 mb-2">Order Confirmed!</h1>
    <p class="text-gray-500">
      Your order <strong class="text-gray-900">#{data.order.order_number || `ORD-${data.order.number}`}</strong> has been placed successfully.
    </p>
  </div>

  <!-- Order summary card -->
  <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
    <h2 class="font-semibold text-gray-900 mb-4">Order Summary</h2>

    <div class="flex flex-col gap-3 mb-4">
      {#each data.order.items as item}
        <div class="flex items-start justify-between gap-3 text-sm">
          <div class="min-w-0">
            <p class="font-medium text-gray-900">{item.product_name}</p>
            <p class="text-xs text-gray-400 mt-0.5">
              {item.variant_sku}
              {#if item.variant_attrs && Object.keys(item.variant_attrs).length > 0}
                · {Object.values(item.variant_attrs).join(', ')}
              {/if}
              · Qty {item.quantity}
            </p>
          </div>
          <span class="text-gray-900 font-medium flex-shrink-0">${item.line_total.toFixed(2)}</span>
        </div>
      {/each}
    </div>

    <div class="border-t border-gray-100 pt-4 flex flex-col gap-2">
      <div class="flex justify-between text-sm text-gray-600">
        <span>Subtotal</span>
        <span>${data.order.subtotal.toFixed(2)}</span>
      </div>
      {#if data.order.discount_amount > 0}
        <div class="flex justify-between text-sm text-green-600">
          <span>Discount</span>
          <span>−${data.order.discount_amount.toFixed(2)}</span>
        </div>
      {/if}
      <div class="flex justify-between text-sm text-gray-600">
        <span>Shipping</span>
        <span>{data.order.shipping_fee > 0 ? `$${data.order.shipping_fee.toFixed(2)}` : 'Free'}</span>
      </div>
      <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900">
        <span>Total</span>
        <span>${data.order.total.toFixed(2)}</span>
      </div>
    </div>
  </div>

  <!-- Status badge -->
  <div class="bg-yellow-50 border border-yellow-100 rounded-2xl p-4 mb-8 flex items-center gap-3">
    <div class="w-2 h-2 rounded-full bg-yellow-400 flex-shrink-0"></div>
    <p class="text-sm text-yellow-800">
      Status: <strong class="capitalize">{data.order.status}</strong> — we'll process your order shortly.
    </p>
  </div>

  <!-- CTAs -->
  <div class="flex flex-col sm:flex-row gap-3 justify-center">
    {#if data.setupURL}
      <a href={data.setupURL}
         class="px-6 py-3 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors text-center">
        Create account to track orders
      </a>
    {/if}
    <a href="/products"
       class="px-6 py-3 border border-gray-200 text-gray-700 font-medium rounded-xl hover:bg-gray-50 transition-colors text-center">
      Continue Shopping
    </a>
  </div>
</div>
