<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { checkout, getVariantByID, validateCoupon } from '$lib/api';
  import type { PageData } from './$types';
  import type { Variant } from '$lib/types';

  let { data }: { data: PageData } = $props();

  let variantMap = $state<Record<string, Variant>>({});
  let loadingVariants = $state(true);

  let selectedAddressID = $state<string>(
    data.addresses?.find((a) => a.is_default)?.id ?? data.addresses?.[0]?.id ?? ''
  );
  let couponCode = $state('');
  let couponResult = $state<{
    valid: boolean;
    discount_amount?: number;
    message?: string;
  } | null>(null);
  let validatingCoupon = $state(false);
  let notes = $state('');
  let placing = $state(false);
  let error = $state('');

  const subtotal = $derived(
    activeCart?.items.reduce((sum, item) => {
      const v = variantMap[item.variant_id];
      return sum + (v ? v.price * item.quantity : 0);
    }, 0) ?? 0
  );

  const discount = $derived(
    couponResult?.valid ? (couponResult.discount_amount ?? 0) : 0
  );

  const shippingFee = 0;

  const total = $derived(subtotal - discount + shippingFee);

  const activeCart = $derived(cartStore.cart);

  onMount(async () => {
    if (!cartStore.cart) await cartStore.init();
    if (!cartStore.cart || cartStore.cart.items.length === 0) {
      goto('/cart');
      return;
    }
    const items = cartStore.cart.items;
    const results = await Promise.allSettled(items.map((i) => getVariantByID(i.variant_id)));
    const map: Record<string, Variant> = {};
    items.forEach((item, idx) => {
      const r = results[idx];
      if (r.status === 'fulfilled') map[item.variant_id] = r.value;
    });
    variantMap = map;
    loadingVariants = false;
  });

  async function applyCoupon() {
    if (!couponCode.trim()) return;
    validatingCoupon = true;
    couponResult = null;
    try {
      const res = await validateCoupon(couponCode.trim(), subtotal);
      if (res.valid) {
        couponResult = { valid: true, discount_amount: res.discount_amount };
      } else {
        couponResult = { valid: false, message: res.message ?? 'Invalid coupon.' };
      }
    } catch {
      couponResult = { valid: false, message: 'Failed to validate coupon.' };
    } finally {
      validatingCoupon = false;
    }
  }

  function removeCoupon() {
    couponCode = '';
    couponResult = null;
  }

  async function placeOrder() {
    if (!activeCart) return;
    placing = true;
    error = '';
    try {
      const order = await checkout(activeCart.id, {
        customerID: data.customer?.id,
        shippingAddressID: selectedAddressID || undefined,
        shippingFee,
        couponCode: couponResult?.valid ? couponCode.trim() : undefined,
        notes: notes.trim() || undefined
      });
      await cartStore.init();
      if (data.customer) {
        goto(`/account/orders/${order.id}`);
      } else {
        goto(`/checkout/success?order=${order.id}`);
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Order placement failed. Please try again.';
    } finally {
      placing = false;
    }
  }
</script>

<svelte:head>
  <title>Checkout — Gyeon</title>
</svelte:head>

<div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-8">Checkout</h1>

  {#if cartStore.loading && !activeCart}
    <div class="text-center py-20 text-gray-400">Loading…</div>

  {:else if !activeCart || activeCart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-gray-400">Your cart is empty.</p>
      <a href="/products"
         class="mt-4 inline-block bg-gray-900 text-white font-medium px-8 py-3 rounded-full hover:bg-gray-700 transition-colors">
        Continue Shopping
      </a>
    </div>

  {:else}
    <div class="flex flex-col lg:flex-row gap-10">

      <!-- Left: Forms -->
      <div class="flex-1 flex flex-col gap-6">

        <!-- Customer info -->
        {#if data.customer}
          <div class="bg-white rounded-2xl border border-gray-100 p-6">
            <p class="text-xs text-gray-400 uppercase tracking-wide font-medium mb-1">Checking out as</p>
            <p class="font-semibold text-gray-900">{data.customer.first_name} {data.customer.last_name}</p>
            <p class="text-sm text-gray-500">{data.customer.email}</p>
          </div>
        {:else}
          <div class="bg-white rounded-2xl border border-gray-100 p-6">
            <p class="font-semibold text-gray-900 mb-1">Guest Checkout</p>
            <p class="text-sm text-gray-500">
              <a href="/account/login" class="text-gray-900 hover:underline font-medium">Sign in</a>
              {' '}to use saved addresses and view order history.
            </p>
          </div>
        {/if}

        <!-- Shipping address (logged-in only) -->
        {#if data.customer}
          <div class="bg-white rounded-2xl border border-gray-100 p-6">
            <h2 class="font-semibold text-gray-900 mb-4">Shipping Address</h2>
            {#if data.addresses && data.addresses.length > 0}
              <div class="flex flex-col gap-4">
                {#each data.addresses as addr}
                  <label class="flex items-start gap-3 cursor-pointer">
                    <input
                      type="radio"
                      name="shipping_address"
                      value={addr.id}
                      bind:group={selectedAddressID}
                      class="mt-0.5 accent-gray-900 flex-shrink-0"
                    />
                    <div class="text-sm leading-relaxed">
                      <span class="font-medium text-gray-900">{addr.first_name} {addr.last_name}</span>
                      {#if addr.is_default}
                        <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded-full">Default</span>
                      {/if}
                      <p class="text-gray-600 mt-0.5">
                        {addr.line1}{#if addr.line2}, {addr.line2}{/if}<br />
                        {addr.city}{#if addr.state}, {addr.state}{/if} {addr.postal_code}, {addr.country}
                      </p>
                      {#if addr.phone}
                        <p class="text-gray-400 mt-0.5">{addr.phone}</p>
                      {/if}
                    </div>
                  </label>
                {/each}
                <label class="flex items-center gap-3 cursor-pointer">
                  <input
                    type="radio"
                    name="shipping_address"
                    value=""
                    bind:group={selectedAddressID}
                    class="accent-gray-900"
                  />
                  <span class="text-sm text-gray-400">No shipping address</span>
                </label>
              </div>
              <a href="/account/addresses/new"
                 class="mt-4 inline-block text-sm text-gray-400 hover:text-gray-900 transition-colors">
                + Add new address
              </a>
            {:else}
              <p class="text-sm text-gray-500 mb-3">No saved addresses yet.</p>
              <a href="/account/addresses/new"
                 class="text-sm font-medium text-gray-900 hover:underline">
                Add an address →
              </a>
            {/if}
          </div>
        {/if}

        <!-- Coupon code -->
        <div class="bg-white rounded-2xl border border-gray-100 p-6">
          <h2 class="font-semibold text-gray-900 mb-4">Coupon Code</h2>
          {#if couponResult?.valid}
            <div class="flex items-center justify-between bg-green-50 border border-green-100 rounded-xl px-4 py-3">
              <div>
                <p class="text-sm font-medium text-green-800">{couponCode.trim()}</p>
                <p class="text-xs text-green-600">−${(couponResult.discount_amount ?? 0).toFixed(2)} discount applied</p>
              </div>
              <button onclick={removeCoupon} class="text-xs text-green-600 hover:text-green-900 font-medium transition-colors">
                Remove
              </button>
            </div>
          {:else}
            <div class="flex gap-2">
              <input
                type="text"
                bind:value={couponCode}
                placeholder="Enter coupon code"
                class="flex-1 border border-gray-200 rounded-xl px-4 py-2.5 text-sm text-gray-900 placeholder:text-gray-300
                       focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"
                onkeydown={(e) => e.key === 'Enter' && applyCoupon()}
              />
              <button
                onclick={applyCoupon}
                disabled={validatingCoupon || !couponCode.trim()}
                class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-gray-700
                       transition-colors disabled:opacity-40"
              >
                {validatingCoupon ? '…' : 'Apply'}
              </button>
            </div>
            {#if couponResult && !couponResult.valid}
              <p class="mt-2 text-sm text-red-500">{couponResult.message}</p>
            {/if}
          {/if}
        </div>

        <!-- Notes -->
        <div class="bg-white rounded-2xl border border-gray-100 p-6">
          <h2 class="font-semibold text-gray-900 mb-1">
            Order Notes
            <span class="font-normal text-gray-400 text-sm ml-1">(optional)</span>
          </h2>
          <p class="text-xs text-gray-400 mb-3">Special instructions, delivery preferences, etc.</p>
          <textarea
            bind:value={notes}
            placeholder="E.g. Leave at door, gift wrap, etc."
            rows="3"
            class="w-full border border-gray-200 rounded-xl px-4 py-3 text-sm text-gray-900 placeholder:text-gray-300
                   focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent resize-none"
          ></textarea>
        </div>

      </div>

      <!-- Right: Order summary + CTA -->
      <div class="lg:w-80 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 sticky top-24">
          <h2 class="font-semibold text-gray-900">Order Summary</h2>

          <!-- Items -->
          <div class="flex flex-col gap-3">
            {#if loadingVariants}
              <p class="text-sm text-gray-400">Loading…</p>
            {:else}
              {#each activeCart!.items as item}
                {@const variant = variantMap[item.variant_id]}
                <div class="flex items-start justify-between gap-3 text-sm">
                  <div class="min-w-0">
                    <p class="font-medium text-gray-900 truncate">{variant?.sku ?? item.variant_id.slice(0, 8) + '…'}</p>
                    <p class="text-xs text-gray-400">Qty: {item.quantity}</p>
                  </div>
                  <span class="text-gray-900 font-medium flex-shrink-0">
                    {variant ? `$${(variant.price * item.quantity).toFixed(2)}` : '—'}
                  </span>
                </div>
              {/each}
            {/if}
          </div>

          <!-- Totals -->
          <div class="border-t border-gray-100 pt-3 flex flex-col gap-2">
            <div class="flex justify-between text-sm text-gray-600">
              <span>Subtotal</span>
              <span>{loadingVariants ? '—' : `$${subtotal.toFixed(2)}`}</span>
            </div>
            {#if discount > 0}
              <div class="flex justify-between text-sm text-green-600">
                <span>Discount</span>
                <span>−${discount.toFixed(2)}</span>
              </div>
            {/if}
            <div class="flex justify-between text-sm text-gray-600">
              <span>Shipping</span>
              <span class="text-green-600">Free</span>
            </div>
            <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900 text-base">
              <span>Total</span>
              <span>{loadingVariants ? '—' : `$${total.toFixed(2)}`}</span>
            </div>
          </div>

          {#if error}
            <p class="text-xs text-red-500 leading-relaxed">{error}</p>
          {/if}

          <button
            onclick={placeOrder}
            disabled={placing || loadingVariants}
            class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700
                   transition-colors disabled:opacity-50"
          >
            {placing ? 'Placing order…' : 'Place Order'}
          </button>

          <a href="/cart" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            ← Back to cart
          </a>
        </div>
      </div>

    </div>
  {/if}
</div>
