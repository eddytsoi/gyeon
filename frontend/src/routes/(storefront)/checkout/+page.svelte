<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { checkout, getVariantByID, validateCoupon } from '$lib/api';
  import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
  import type { PageData } from './$types';
  import type { Variant } from '$lib/types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';

  let { data }: { data: PageData } = $props();

  let variantMap = $state<Record<string, Variant>>({});
  let loadingVariants = $state(true);

  // ── Customer info (section 1) ─────────────────────────────────
  let firstName = $state(data.customer?.first_name ?? '');
  let lastName = $state(data.customer?.last_name ?? '');
  let email = $state(data.customer?.email ?? '');
  let phone = $state(data.customer?.phone ?? '');

  // ── Shipping (section 2) ──────────────────────────────────────
  type AddressMode = 'saved' | 'new';
  const hasSavedAddresses = (data.addresses?.length ?? 0) > 0;
  let addressMode = $state<AddressMode>(hasSavedAddresses ? 'saved' : 'new');
  let selectedAddressID = $state<string>(
    data.addresses?.find((a) => a.is_default)?.id ?? data.addresses?.[0]?.id ?? ''
  );
  let line1 = $state('');
  let city = $state('');
  let addressState = $state('');
  let postalCode = $state('');
  let country = $state(data.shippingCountries[0] ?? 'HK');
  let saveAddress = $state(false);

  const cityListId = 'checkout-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);

  // ── Remark (section 3) ────────────────────────────────────────
  let notes = $state('');

  // ── Coupon ────────────────────────────────────────────────────
  let couponCode = $state('');
  let couponResult = $state<{
    valid: boolean;
    discount_amount?: number;
    message?: string;
  } | null>(null);
  let validatingCoupon = $state(false);

  // ── Stripe Payment Element (section 4) ────────────────────────
  let stripe: Stripe | null = $state(null);
  let elements: StripeElements | null = $state(null);
  let paymentReady = $state(false);
  let paymentMounting = $state(false);
  let pendingClientSecret = $state<string | null>(null);
  let pendingOrderID = $state<string | null>(null);
  let paymentElementMounted = $state(false);

  // ── T&C (section 5) ───────────────────────────────────────────
  let tcAccepted = $state(false);

  // ── Submit state ──────────────────────────────────────────────
  let placing = $state(false);
  let error = $state('');

  const activeCart = $derived(cartStore.cart);

  const subtotal = $derived(
    activeCart?.items.reduce((sum, item) => {
      const v = variantMap[item.variant_id];
      return sum + (v ? v.price * item.quantity : 0);
    }, 0) ?? 0
  );
  const discount = $derived(couponResult?.valid ? (couponResult.discount_amount ?? 0) : 0);
  const shippingFee = 0;
  const total = $derived(subtotal - discount + shippingFee);

  const customerValid = $derived(
    firstName.trim() !== '' &&
      email.trim() !== '' &&
      phone.trim() !== ''
  );

  const shippingValid = $derived(
    addressMode === 'saved'
      ? selectedAddressID !== ''
      : line1.trim() !== '' && country.trim() !== ''
  );

  const formValid = $derived(customerValid && shippingValid && tcAccepted);

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

    // Load Stripe with the publishable key from server-side payment config.
    if (data.paymentConfig?.publishable_key) {
      stripe = await loadStripe(data.paymentConfig.publishable_key);
    }
  });

  async function applyCoupon() {
    if (!couponCode.trim()) return;
    validatingCoupon = true;
    couponResult = null;
    try {
      const res = await validateCoupon(couponCode.trim(), subtotal);
      couponResult = res.valid
        ? { valid: true, discount_amount: res.discount_amount }
        : { valid: false, message: res.message ?? 'Invalid coupon.' };
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

  // Step 1: create the order + PaymentIntent, mount Payment Element
  async function continueToPayment() {
    if (!activeCart || !stripe) {
      error = stripe ? 'Cart is empty.' : 'Payment is not configured. Please contact the site owner.';
      return;
    }
    if (!formValid) return;

    paymentMounting = true;
    error = '';
    try {
      const result = await checkout(activeCart.id, {
        customerID: data.customer?.id,
        customerInfo: {
          first_name: firstName.trim(),
          last_name: lastName.trim(),
          email: email.trim(),
          phone: phone.trim()
        },
        shippingAddressID: addressMode === 'saved' ? selectedAddressID : undefined,
        shippingAddress:
          addressMode === 'new'
            ? {
                line1: line1.trim(),
                city: city.trim(),
                state: addressState.trim() || undefined,
                postal_code: postalCode.trim(),
                country: country.trim() || 'HK'
              }
            : undefined,
        saveAddress: addressMode === 'new' && saveAddress,
        shippingFee,
        couponCode: couponResult?.valid ? couponCode.trim() : undefined,
        notes: notes.trim() || undefined
      });

      pendingClientSecret = result.client_secret;
      pendingOrderID = result.order.id;

      elements = stripe.elements({
        clientSecret: result.client_secret,
        appearance: { theme: 'stripe', variables: { colorPrimary: '#111827' } }
      });
      const paymentElement = elements.create('payment', { layout: 'tabs' });
      paymentElement.mount('#payment-element');
      paymentElementMounted = true;
      paymentReady = true;

      // Scroll to payment section
      requestAnimationFrame(() => {
        document.getElementById('payment-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not start payment. Please try again.';
    } finally {
      paymentMounting = false;
    }
  }

  // Step 2: confirm payment with Stripe
  async function confirmPay() {
    if (!stripe || !elements || !pendingOrderID) return;
    placing = true;
    error = '';
    try {
      const { error: stripeError } = await stripe.confirmPayment({
        elements,
        confirmParams: {
          return_url: `${location.origin}/checkout/success?order=${pendingOrderID}`
        }
      });
      // confirmPayment redirects on success; reaching here means an error.
      if (stripeError) {
        error = stripeError.message ?? 'Payment failed.';
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Payment failed.';
    } finally {
      placing = false;
    }
  }
</script>

<svelte:head>
  <title>結帳 — Gyeon</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-8">結帳</h1>

  {#if cartStore.loading && !activeCart}
    <div class="text-center py-20 text-gray-400">載入中…</div>

  {:else if !activeCart || activeCart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-gray-400">您的購物車是空的。</p>
      <a href="/products"
         class="mt-4 inline-block bg-gray-900 text-white font-medium px-8 py-3 rounded-full hover:bg-gray-700 transition-colors">
        繼續購物
      </a>
    </div>

  {:else}
    <div class="flex flex-col lg:flex-row gap-10">
      <!-- Left: Forms -->
      <div class="flex-1 flex flex-col gap-6">

        <!-- ── 1. Customer Info ────────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">1</span>
            <h2 class="font-semibold text-gray-900">聯絡資料</h2>
          </div>
          {#if data.customer}
            <p class="text-xs text-gray-400 mb-4">
              已登入：{data.customer.email}（如有需要可調整以下資料）
            </p>
          {/if}
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="first-name" class="block text-xs font-medium text-gray-500 mb-1">姓氏 <span class="text-red-400">*</span></label>
              <input id="first-name" type="text" bind:value={firstName} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="last-name" class="block text-xs font-medium text-gray-500 mb-1">名字</label>
              <input id="last-name" type="text" bind:value={lastName}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="phone" class="block text-xs font-medium text-gray-500 mb-1">電話 <span class="text-red-400">*</span></label>
              <input id="phone" type="tel" bind:value={phone} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="email" class="block text-xs font-medium text-gray-500 mb-1">電郵 <span class="text-red-400">*</span></label>
              <input id="email" type="email" bind:value={email} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
          {#if !data.customer}
            <p class="text-xs text-gray-400 mt-3">
              <a href="/account/login" class="text-gray-900 hover:underline font-medium">登入</a>
              即可使用已儲存地址與訂單記錄。
            </p>
          {/if}
        </section>

        <!-- ── 2. Shipping Address ─────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">2</span>
            <h2 class="font-semibold text-gray-900">送貨地址</h2>
          </div>

          {#if hasSavedAddresses}
            <div class="flex gap-2 mb-4">
              <button type="button"
                      onclick={() => (addressMode = 'saved')}
                      class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                             {addressMode === 'saved' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                已儲存地址
              </button>
              <button type="button"
                      onclick={() => (addressMode = 'new')}
                      class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                             {addressMode === 'new' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                使用新地址
              </button>
            </div>
          {/if}

          {#if addressMode === 'saved' && hasSavedAddresses}
            <div class="flex flex-col gap-3">
              {#each data.addresses ?? [] as addr}
                <label class="flex items-start gap-3 cursor-pointer p-3 rounded-xl border
                              {selectedAddressID === addr.id ? 'border-gray-900 bg-gray-50' : 'border-gray-100'}">
                  <input type="radio" name="shipping_address" value={addr.id}
                         bind:group={selectedAddressID}
                         class="mt-0.5 accent-gray-900 flex-shrink-0" />
                  <div class="text-sm leading-relaxed flex-1">
                    <span class="font-medium text-gray-900">{addr.first_name} {addr.last_name}</span>
                    {#if addr.is_default}
                      <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded-full">預設</span>
                    {/if}
                    <p class="text-gray-600 mt-0.5">
                      {addr.line1}{#if addr.line2}, {addr.line2}{/if}<br />
                      {addr.city}{#if addr.state}, {addr.state}{/if} {addr.postal_code}, {addr.country}
                    </p>
                  </div>
                </label>
              {/each}
            </div>
          {:else}
            <div class="flex flex-col gap-3">
              <div>
                <label for="line1" class="block text-xs font-medium text-gray-500 mb-1">詳細地址 <span class="text-red-400">*</span></label>
                <input id="line1" type="text" bind:value={line1} required placeholder="街道、門牌、樓層、單位"
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label for="city" class="block text-xs font-medium text-gray-500 mb-1">區域</label>
                  <input id="city" type="text" bind:value={city} list={cityOptions.length > 0 ? cityListId : undefined}
                         placeholder={country === 'HK' ? '例：九龍城區' : ''} autocomplete="off"
                         class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                  {#if cityOptions.length > 0}
                    <datalist id={cityListId}>
                      {#each cityOptions as opt}
                        <option value={opt}></option>
                      {/each}
                    </datalist>
                  {/if}
                </div>
                <div>
                  <label for="state" class="block text-xs font-medium text-gray-500 mb-1">州 / 省（可選）</label>
                  <input id="state" type="text" bind:value={addressState}
                         class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                </div>
                <div>
                  <label for="postal" class="block text-xs font-medium text-gray-500 mb-1">郵政編碼</label>
                  <input id="postal" type="text" bind:value={postalCode}
                         class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                </div>
                <div>
                  <label for="country" class="block text-xs font-medium text-gray-500 mb-1">國家 / 地區 <span class="text-red-400">*</span></label>
                  {#if data.shippingCountries.length === 1}
                    <input id="country" type="text" value="{COUNTRY_BY_CODE[data.shippingCountries[0]] ?? data.shippingCountries[0]} ({data.shippingCountries[0]})" readonly
                           class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-gray-50 text-gray-500" />
                  {:else}
                    <select id="country" bind:value={country} required
                            class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                                   focus:outline-none focus:ring-2 focus:ring-gray-900">
                      {#each data.shippingCountries as code}
                        <option value={code}>{COUNTRY_BY_CODE[code] ?? code} ({code})</option>
                      {/each}
                    </select>
                  {/if}
                </div>
              </div>
              {#if data.customer}
                <label class="flex items-center gap-2 mt-2 cursor-pointer">
                  <input type="checkbox" bind:checked={saveAddress} class="accent-gray-900" />
                  <span class="text-sm text-gray-600">儲存到我的地址</span>
                </label>
              {/if}
            </div>
          {/if}
        </section>

        <!-- ── 3. Remark ───────────────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">3</span>
            <h2 class="font-semibold text-gray-900">備註 <span class="font-normal text-gray-400 text-sm">（可選）</span></h2>
          </div>
          <textarea bind:value={notes}
                    placeholder="送貨指示、禮品包裝等"
                    rows="3"
                    class="w-full border border-gray-200 rounded-xl px-4 py-3 text-sm text-gray-900 placeholder:text-gray-300
                           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent resize-none"></textarea>
        </section>

        <!-- ── 4. Payment ──────────────────────────────────────── -->
        <section id="payment-section" class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">4</span>
            <h2 class="font-semibold text-gray-900">付款方式</h2>
          </div>
          {#if !paymentReady}
            <p class="text-xs text-gray-400 mb-4">填妥以上資料後，按「繼續付款」即會顯示付款表單。</p>
            <button type="button"
                    onclick={continueToPayment}
                    disabled={!formValid || paymentMounting || !stripe}
                    class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                           hover:bg-gray-700 transition-colors disabled:opacity-50">
              {paymentMounting ? '正在準備付款…' : '繼續付款'}
            </button>
            {#if !stripe && data.paymentConfig?.publishable_key === ''}
              <p class="mt-3 text-xs text-red-500">付款功能未設定，請聯絡店主。</p>
            {/if}
          {/if}
          <div id="payment-element" class="{paymentReady ? '' : 'hidden'}"></div>
        </section>

        <!-- ── 5. T&C + Pay ────────────────────────────────────── -->
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

          {#if paymentReady}
            <button type="button"
                    onclick={confirmPay}
                    disabled={placing || !tcAccepted}
                    class="mt-5 w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                           hover:bg-gray-700 transition-colors disabled:opacity-50">
              {placing ? '處理中…' : `付款 HK$${total.toFixed(2)}`}
            </button>
          {/if}
        </section>
      </div>

      <!-- Right: Order summary -->
      <div class="lg:w-2/5 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 sticky top-24">
          <h2 class="font-semibold text-gray-900">訂單摘要</h2>

          <div class="flex flex-col gap-3">
            {#if loadingVariants}
              <p class="text-sm text-gray-400">載入中…</p>
            {:else if activeCart}
              {#each activeCart.items as item}
                {@const variant = variantMap[item.variant_id]}
                <div class="flex items-start justify-between gap-3 text-sm">
                  <div class="min-w-0">
                    <p class="font-medium text-gray-900 truncate">{variant?.product_name ?? variant?.sku ?? item.variant_id.slice(0, 8) + '…'}</p>
                    <p class="text-xs text-gray-400">數量：{item.quantity}</p>
                  </div>
                  <span class="text-gray-900 font-medium flex-shrink-0">
                    {variant ? `HK$${(variant.price * item.quantity).toFixed(2)}` : '—'}
                  </span>
                </div>
              {/each}
            {/if}
          </div>

          <!-- Coupon -->
          <div class="border-t border-gray-100 pt-3">
            {#if couponResult?.valid}
              <div class="flex items-center justify-between bg-green-50 border border-green-100 rounded-xl px-3 py-2">
                <div>
                  <p class="text-xs font-medium text-green-800">{couponCode.trim()}</p>
                  <p class="text-[11px] text-green-600">−HK${(couponResult.discount_amount ?? 0).toFixed(2)}</p>
                </div>
                <button type="button" onclick={removeCoupon} class="text-xs text-green-600 hover:text-green-900">移除</button>
              </div>
            {:else}
              <div class="flex gap-2">
                <input type="text" bind:value={couponCode} placeholder="優惠券代碼"
                       class="flex-1 border border-gray-200 rounded-xl px-3 py-2 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900"
                       onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyCoupon())} />
                <button type="button" onclick={applyCoupon}
                        disabled={validatingCoupon || !couponCode.trim()}
                        class="px-3 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-gray-700 disabled:opacity-40">
                  {validatingCoupon ? '…' : '套用'}
                </button>
              </div>
              {#if couponResult && !couponResult.valid}
                <p class="mt-1.5 text-xs text-red-500">{couponResult.message}</p>
              {/if}
            {/if}
          </div>

          <!-- Totals -->
          <div class="border-t border-gray-100 pt-3 flex flex-col gap-2">
            <div class="flex justify-between text-sm text-gray-600">
              <span>小計</span>
              <span>{loadingVariants ? '—' : `HK$${subtotal.toFixed(2)}`}</span>
            </div>
            {#if discount > 0}
              <div class="flex justify-between text-sm text-green-600">
                <span>折扣</span>
                <span>−HK${discount.toFixed(2)}</span>
              </div>
            {/if}
            <div class="flex justify-between text-sm text-gray-600">
              <span>運費</span>
              <span class="text-green-600">免運費</span>
            </div>
            <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900 text-base">
              <span>總額</span>
              <span>{loadingVariants ? '—' : `HK$${total.toFixed(2)}`}</span>
            </div>
          </div>

          <a href="/cart" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            ← 返回購物車
          </a>
        </div>
      </div>
    </div>
  {/if}
</div>
