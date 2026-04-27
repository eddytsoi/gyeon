<script lang="ts">
  import { onMount } from 'svelte';
  import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  let stripe: Stripe | null = $state(null);
  let elements: StripeElements | null = $state(null);
  let mounting = $state(true);
  let placing = $state(false);
  let error = $state('');
  let tcAccepted = $state(false);

  const order = $derived(data.info.order);
  const clientSecret = $derived(data.info.client_secret);

  onMount(async () => {
    if (!data.info.publishable_key) {
      error = '付款功能未設定，請聯絡店主。';
      mounting = false;
      return;
    }
    try {
      stripe = await loadStripe(data.info.publishable_key);
      if (!stripe) throw new Error('Stripe failed to load');

      elements = stripe.elements({
        clientSecret,
        appearance: { theme: 'stripe', variables: { colorPrimary: '#111827' } }
      });
      const paymentElement = elements.create('payment', { layout: 'tabs' });
      paymentElement.mount('#payment-element');
    } catch (e) {
      error = e instanceof Error ? e.message : '無法載入付款表單';
    } finally {
      mounting = false;
    }
  });

  async function confirmPay() {
    if (!stripe || !elements) return;
    placing = true;
    error = '';
    try {
      const { error: stripeError } = await stripe.confirmPayment({
        elements,
        confirmParams: {
          return_url: `${location.origin}/checkout/success?order=${order.id}`
        }
      });
      if (stripeError) {
        error = stripeError.message ?? '付款失敗';
      }
    } catch (e) {
      error = e instanceof Error ? e.message : '付款失敗';
    } finally {
      placing = false;
    }
  }
</script>

<svelte:head>
  <title>完成付款 — Gyeon</title>
</svelte:head>

<div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-2">完成付款</h1>
  <p class="text-sm text-gray-500 mb-8">
    訂單編號 <span class="font-mono text-gray-900">{order.id.slice(0, 8)}</span>
  </p>

  <div class="flex flex-col gap-6">
    <!-- Order summary -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6">
      <h2 class="font-semibold text-gray-900 mb-4">訂單摘要</h2>
      <div class="flex flex-col gap-3">
        {#each order.items ?? [] as item}
          <div class="flex items-start justify-between gap-3 text-sm">
            <div class="min-w-0">
              <p class="font-medium text-gray-900 truncate">{item.product_name}</p>
              <p class="text-xs text-gray-400">
                {item.variant_sku} · 數量 {item.quantity}
              </p>
            </div>
            <span class="text-gray-900 font-medium flex-shrink-0">
              HK${item.line_total.toFixed(2)}
            </span>
          </div>
        {/each}
      </div>
      <div class="border-t border-gray-100 mt-4 pt-3 flex flex-col gap-2 text-sm">
        <div class="flex justify-between text-gray-600">
          <span>小計</span>
          <span>HK${order.subtotal.toFixed(2)}</span>
        </div>
        {#if order.discount_amount > 0}
          <div class="flex justify-between text-green-600">
            <span>折扣</span>
            <span>−HK${order.discount_amount.toFixed(2)}</span>
          </div>
        {/if}
        <div class="flex justify-between text-gray-600">
          <span>運費</span>
          <span>{order.shipping_fee > 0 ? `HK$${order.shipping_fee.toFixed(2)}` : '免運費'}</span>
        </div>
        <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900 text-base">
          <span>總額</span>
          <span>HK${order.total.toFixed(2)}</span>
        </div>
      </div>
    </section>

    <!-- Payment -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6">
      <h2 class="font-semibold text-gray-900 mb-4">付款方式</h2>
      {#if mounting}
        <p class="text-sm text-gray-400">載入付款表單中…</p>
      {/if}
      <div id="payment-element" class={mounting ? 'hidden' : ''}></div>
    </section>

    <!-- T&C + Pay -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6">
      <label class="flex items-start gap-3 cursor-pointer">
        <input type="checkbox" bind:checked={tcAccepted}
               class="mt-0.5 accent-gray-900 flex-shrink-0" />
        <span class="text-sm text-gray-700 leading-relaxed">
          我已閱讀並同意網站的<a href="/pages/terms-and-conditions" target="_blank"
             class="text-gray-900 underline font-medium">〈條款與條件〉</a>
        </span>
      </label>

      {#if error}
        <p class="mt-4 text-sm text-red-500 leading-relaxed">{error}</p>
      {/if}

      <button type="button"
              onclick={confirmPay}
              disabled={mounting || placing || !tcAccepted || !stripe}
              class="mt-5 w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                     hover:bg-gray-700 transition-colors disabled:opacity-50">
        {placing ? '處理中…' : `付款 HK$${order.total.toFixed(2)}`}
      </button>
    </section>
  </div>
</div>
