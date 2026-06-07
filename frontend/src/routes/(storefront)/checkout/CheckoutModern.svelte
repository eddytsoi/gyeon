<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { formatHKD, roundAmount } from '$lib/money';
  import { checkout, getVariantByID, quoteOrder, type QuoteResult } from '$lib/api';
  import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
  import type { PageData } from './$types';
  import type { Variant } from '$lib/types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';
  import { productDisplayName } from '$lib/variant';
  import { resolveFreeShippingThreshold } from '$lib/shippingThreshold';
  import AppliedPromotions from '$lib/components/AppliedPromotions.svelte';
  import PendingOrderBanner from '$lib/components/shop/PendingOrderBanner.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import Eyebrow from '$lib/components/shop/Eyebrow.svelte';
  import SecurePaymentNote from '$lib/components/shop/SecurePaymentNote.svelte';
  import BankTransferNotice from '$lib/components/shop/BankTransferNotice.svelte';
  import { isBankTransferRole, resolveBankTransfer } from '$lib/bankTransfer';
  import { slide } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  // Payment method is fixed by the customer's role: installer / installer_v2 pay
  // by bank transfer (no Stripe), everyone else pays by Stripe. The backend
  // enforces this authoritatively from the auth token; this only picks the UI.
  const isBankTransfer = $derived(isBankTransferRole(data.customer?.role));
  const bankDetails = $derived(resolveBankTransfer(data.publicSettings ?? []));

  let variantMap = $state<Record<string, Variant>>({});
  let loadingVariants = $state(true);

  // ── Stepper ───────────────────────────────────────────────────
  // Two steps: 1 = contact & delivery, 2 = payment. Only one is expanded
  // at a time; completed steps collapse to a one-line summary + edit.
  let currentStep = $state<1 | 2>(1);
  // Mobile-only collapsible order summary (a sticky sidebar shows on lg+).
  let summaryOpen = $state(false);

  // ── Customer info ─────────────────────────────────────────────
  let firstName = $state(data.customer?.first_name ?? '');
  let lastName = $state(data.customer?.last_name ?? '');
  let email = $state(data.customer?.email ?? '');
  let phone = $state(data.customer?.phone ?? '');

  // ── Shipping ──────────────────────────────────────────────────
  type AddressMode = 'saved' | 'new';
  const hasSavedAddresses = (data.addresses?.length ?? 0) > 0;
  let addressMode = $state<AddressMode>(hasSavedAddresses ? 'saved' : 'new');
  let selectedAddressID = $state<string>(
    data.addresses?.find((a) => a.is_default)?.id ?? data.addresses?.[0]?.id ?? ''
  );
  let line1 = $state('');
  let city = $state('');
  let country = $state(data.shippingCountries[0] ?? 'HK');
  let saveAddress = $state(false);

  const cityListId = 'checkout-modern-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);

  // ── Logistics (read-only, admin default) ──────────────────────
  const shippingConfigured = $derived(
    !data.shipanyEnabled || (data.shippingDefault?.configured ?? false)
  );
  const isSFCourier = $derived(/sf|順豐/i.test(data.shippingDefault?.courier_name ?? ''));

  // ── Remark ────────────────────────────────────────────────────
  let notes = $state('');

  // ── Coupon ────────────────────────────────────────────────────
  let couponCode = $state('');
  let appliedCouponCode = $state('');
  let validatingCoupon = $state(false);

  // ── Stripe (deferred-intent inline Payment Element) ───────────
  // Unlike the classic two-step flow, the Payment Element is created in
  // Stripe's *deferred* mode (mode/amount/currency only — no PaymentIntent
  // yet) so the card field is visible inline. The Order + PaymentIntent are
  // created only when the shopper clicks Pay (see pay()), which then confirms
  // the intent. Needs no backend changes — /orders/checkout is unchanged.
  let stripe: Stripe | null = $state(null);
  let elements: StripeElements | null = $state(null);
  let paymentElementMounted = $state(false);
  // True once the shopper has selected/entered a complete payment method in the
  // Payment Element (e.g. confirmed the Link card via 使用這張卡). Gates the Pay
  // button so it can't look clickable before a card is actually chosen.
  let paymentComplete = $state(false);

  // ── Submit / pending state ────────────────────────────────────
  let tcAccepted = $state(false);
  let tcExpanded = $state(false);
  let placing = $state(false);
  let error = $state('');
  let pendingClientSecret = $state<string | null>(null);
  let pendingOrderID = $state<string | null>(null);
  let pendingOrderNumber = $state('');

  const activeCart = $derived(cartStore.cart);

  // ── Server quote (same pricing as Checkout) ───────────────────
  let quote = $state<QuoteResult | null>(null);
  let quoteVersion = 0;

  const clientSubtotal = $derived(
    activeCart?.items?.reduce((sum, item) => {
      const v = variantMap[item.variant_id];
      return sum + (v ? v.price * item.quantity : 0);
    }, 0) ?? 0
  );
  const subtotal = $derived(quote?.subtotal ?? clientSubtotal);
  const discount = $derived(quote?.total_discount ?? 0);
  const total = $derived(quote?.total ?? subtotal - discount);
  const totalCents = $derived(Math.round(total * 100));

  const couponValid = $derived(
    appliedCouponCode !== '' && !!quote?.applied_coupon && !(quote?.coupon_error)
  );
  const couponDiscountAmount = $derived(quote?.applied_coupon?.amount ?? 0);
  const couponErrorMsg = $derived(() => {
    if (appliedCouponCode === '' || !quote || !quote.coupon_error) return null;
    if (quote.coupon_error_code === 'wrong_role') return m.storefront_coupon_wrong_role();
    return quote.coupon_error || m.checkout_invalid_coupon();
  });

  const appliedPromotionsList = $derived([
    ...(quote?.applied_campaigns ?? []).map((c) => ({
      kind: 'campaign',
      id: c.id,
      name: c.name,
      description: c.description,
      amount: c.amount
    })),
    ...(quote?.applied_coupon
      ? [
          {
            kind: 'coupon',
            id: quote.applied_coupon.id,
            name: quote.applied_coupon.code,
            description: quote.applied_coupon.description,
            amount: quote.applied_coupon.amount
          }
        ]
      : [])
  ]);

  const resolvedFreeShipping = $derived(
    resolveFreeShippingThreshold(data.publicSettings ?? [], data.customer?.role ?? null)
  );
  const freeShippingEnabled = $derived(resolvedFreeShipping.enabled);
  const freeShippingThreshold = $derived(() => resolvedFreeShipping.threshold);
  const clientShippingFree = $derived(
    freeShippingEnabled && freeShippingThreshold() > 0 && subtotal >= freeShippingThreshold()
  );
  const shippingFree = $derived(quote?.shipping_free ?? clientShippingFree);

  async function refreshQuote() {
    const cart = cartStore.cart;
    if (!cart || cart.items.length === 0) {
      quote = null;
      return;
    }
    const v = ++quoteVersion;
    try {
      const res = await quoteOrder(cart.id, {
        couponCode: appliedCouponCode || undefined,
        customerID: data.customer?.id ?? undefined
      });
      if (v === quoteVersion) quote = res;
    } catch {
      /* keep last good quote */
    }
  }

  $effect(() => {
    const items = cartStore.cart?.items ?? [];
    const sig = items.map((i) => `${i.variant_id}:${i.quantity}`).join('|');
    void sig;
    void appliedCouponCode;
    const t = setTimeout(refreshQuote, 200);
    return () => clearTimeout(t);
  });

  // Keep the deferred Payment Element's displayed amount in sync with the
  // live quote total (e.g. after a coupon is applied) so the figure the
  // shopper sees matches the PaymentIntent created at Pay time.
  $effect(() => {
    const cents = totalCents;
    if (elements && paymentElementMounted && cents > 0) {
      elements.update({ amount: cents });
    }
  });

  // ── Validation ────────────────────────────────────────────────
  const customerValid = $derived(
    firstName.trim() !== '' && email.trim() !== '' && phone.trim() !== ''
  );
  const shippingValid = $derived(
    addressMode === 'saved'
      ? selectedAddressID !== ''
      : line1.trim() !== '' && country.trim() !== ''
  );
  const step1Valid = $derived(customerValid && shippingValid && shippingConfigured);
  const formValid = $derived(step1Valid && tcAccepted);

  // Collapsed step-1 summary shown once the shopper advances to payment.
  const selectedAddress = $derived(data.addresses?.find((a) => a.id === selectedAddressID));
  const step1Summary = $derived.by(() => {
    const name = `${firstName} ${lastName}`.trim();
    const addr =
      addressMode === 'saved'
        ? selectedAddress
          ? `${selectedAddress.line1}, ${selectedAddress.city}`
          : ''
        : [line1, city].filter(Boolean).join(', ');
    return [name, phone, addr].filter(Boolean).join(' · ');
  });

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

    // Skip Stripe entirely for bank-transfer (installer) checkout — no card.
    if (!isBankTransfer && data.paymentConfig?.publishable_key) {
      stripe = await loadStripe(data.paymentConfig.publishable_key);
    }
  });

  // Bank-transfer checkout: create the on-hold order (no Stripe) and go straight
  // to the confirmation page, which shows the transfer instructions.
  async function placeBankTransferOrder() {
    if (!activeCart) {
      error = m.checkout_cart_empty_error();
      return;
    }
    if (!formValid) return;
    placing = true;
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
            ? { line1: line1.trim(), city: city.trim(), country: country.trim() || 'HK' }
            : undefined,
        saveAddress: addressMode === 'new' && saveAddress,
        shippingFee: 0,
        couponCode: couponValid ? appliedCouponCode : undefined,
        notes: notes.trim() || undefined
      });
      await goto(`/checkout/success?order=${result.order.id}&method=bank_transfer`);
    } catch (e) {
      error = e instanceof Error ? e.message : m.checkout_payment_start_failed();
      placing = false;
    }
  }

  async function applyCoupon() {
    const code = couponCode.trim();
    if (!code) return;
    validatingCoupon = true;
    try {
      appliedCouponCode = code;
      await refreshQuote();
    } finally {
      validatingCoupon = false;
    }
  }
  function removeCoupon() {
    couponCode = '';
    appliedCouponCode = '';
  }

  // Create + mount the deferred Payment Element once we're on the payment
  // step in new-card mode. Safe to call repeatedly (guards on mounted flag).
  async function maybeMountPayment() {
    if (paymentElementMounted || !stripe || totalCents <= 0) return;
    const stripeCountry = data.paymentConfig?.country || 'HK';
    elements = stripe.elements({
      mode: 'payment',
      amount: totalCents,
      currency: 'hkd',
      appearance: { theme: 'stripe', variables: { colorPrimary: '#334977' } }
    });
    const paymentElement = elements.create('payment', {
      layout: 'tabs',
      // Pass the email so Stripe Link can recognise returning customers and
      // offer autofill / one-click checkout against their Link-saved cards.
      defaultValues: { billingDetails: { email: email.trim(), address: { country: stripeCountry } } }
    });
    paymentElement.mount('#payment-element-modern');
    paymentElement.on('change', (e) => {
      paymentComplete = e.complete;
    });
    paymentElementMounted = true;
  }

  async function goToPayment() {
    if (!step1Valid) return;
    currentStep = 2;
    await tick();
    if (!isBankTransfer) await maybeMountPayment();
    document.getElementById('checkout-step-2')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function editStep1() {
    currentStep = 1;
    error = '';
  }

  // Single-step pay: validate the card field, create the order + intent via
  // the existing /orders/checkout, then confirm the deferred intent.
  async function pay() {
    if (!activeCart || !stripe) {
      error = stripe ? m.checkout_cart_empty_error() : m.checkout_no_payment_setup();
      return;
    }
    if (!formValid) return;

    placing = true;
    error = '';
    try {
      // Validate the inline card element before creating the order.
      if (!elements) {
        error = m.checkout_no_payment_setup();
        return;
      }
      const { error: submitError } = await elements.submit();
      if (submitError) {
        error = submitError.message ?? m.checkout_payment_failed();
        return;
      }

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
            ? { line1: line1.trim(), city: city.trim(), country: country.trim() || 'HK' }
            : undefined,
        saveAddress: addressMode === 'new' && saveAddress,
        shippingFee: 0,
        couponCode: couponValid ? appliedCouponCode : undefined,
        notes: notes.trim() || undefined
      });

      pendingClientSecret = result.client_secret;
      pendingOrderID = result.order.id;
      pendingOrderNumber = result.order.order_number || `ORD-${result.order.number}`;

      const { error: confirmError } = await stripe.confirmPayment({
        elements: elements!,
        clientSecret: result.client_secret,
        confirmParams: {
          return_url: `${location.origin}/checkout/success?order=${result.order.id}`
        }
      });
      // confirmPayment redirects on success; reaching here means an error.
      if (confirmError) error = confirmError.message ?? m.checkout_payment_failed();
    } catch (e) {
      error = e instanceof Error ? e.message : m.checkout_payment_start_failed();
    } finally {
      placing = false;
    }
  }
</script>

<!-- ── Reusable order-summary block (mobile drawer + desktop sidebar) ── -->
{#snippet orderSummary()}
  <div class="flex flex-col gap-4">
    <div class="flex flex-col gap-3">
      {#if loadingVariants}
        <p class="text-sm text-ink-500">{m.common_loading()}</p>
      {:else if activeCart}
        {#each activeCart.items as item}
          {@const variant = variantMap[item.variant_id]}
          <div class="flex flex-col gap-1.5">
            <div class="flex items-start justify-between gap-3 text-sm">
              <div class="min-w-0">
                <p class="font-medium text-ink-900 truncate uppercase">{variant?.product_name ? productDisplayName(variant.product_name, variant.name) : variant?.sku ?? item.variant_id.slice(0, 8) + '…'}</p>
                {#if item.product_subtitle}
                  <p class="text-xs text-ink-500 truncate">{item.product_subtitle}</p>
                {/if}
                <p class="text-xs text-ink-300">{m.checkout_summary_qty({ quantity: item.quantity })}</p>
              </div>
              <span class="text-ink-900 font-medium flex-shrink-0 tabular-nums">
                {variant ? formatHKD(variant.price * item.quantity) : '—'}
              </span>
            </div>
            {#if item.children?.length}
              <ul class="pl-3 flex flex-col gap-0.5 border-l border-gray-100">
                {#each item.children as child}
                  <li class="flex items-center justify-between gap-3 text-xs text-ink-500">
                    <span class="truncate uppercase">↳ {productDisplayName(child.product_name, child.variant_name)}</span>
                    <span class="flex-shrink-0 tabular-nums">× {child.quantity}</span>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    <!-- Coupon -->
    <div class="border-t border-gray-100 pt-3">
      {#if couponValid}
        <div class="flex items-center justify-between bg-success/10 border border-success/20 rounded-xl px-3 py-2">
          <div>
            <p class="text-xs font-medium text-success">{appliedCouponCode}</p>
            <p class="text-[11px] text-success tabular-nums">−{formatHKD(couponDiscountAmount)}</p>
          </div>
          <button type="button" onclick={removeCoupon} class="text-xs text-success hover:opacity-70">{m.checkout_coupon_remove()}</button>
        </div>
      {:else}
        <div class="flex gap-2">
          <input type="text" bind:value={couponCode} placeholder={m.checkout_coupon_placeholder()}
                 class="w-full flex-1 border border-gray-200 rounded-xl px-3 py-2 text-sm
                        focus:outline-none focus:ring-2 focus:ring-navy-500"
                 onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyCoupon())} />
          <button type="button" onclick={applyCoupon}
                  disabled={validatingCoupon || !couponCode.trim()}
                  class="px-3 py-2 bg-navy-500 text-white text-sm font-medium rounded-xl hover:bg-navy-700 disabled:opacity-40">
            {validatingCoupon ? m.checkout_coupon_applying() : m.checkout_coupon_apply()}
          </button>
        </div>
        {#if couponErrorMsg()}
          <p class="mt-1.5 text-xs text-alert">{couponErrorMsg()}</p>
        {/if}
      {/if}
    </div>

    <!-- Totals -->
    <div class="border-t border-gray-100 pt-3 flex flex-col gap-2">
      <div class="flex justify-between text-sm text-ink-500">
        <span>{m.checkout_summary_subtotal()}</span>
        <span class="tabular-nums">{loadingVariants ? '—' : formatHKD(subtotal)}</span>
      </div>
      {#if discount > 0}
        <div class="flex justify-between text-sm text-success">
          <span>{m.checkout_summary_discount()}</span>
          <span class="tabular-nums">−{formatHKD(discount)}</span>
        </div>
      {/if}
      <div class="flex justify-between text-sm text-ink-500">
        <span>{m.checkout_summary_shipping()}</span>
        <span class="whitespace-nowrap {shippingFree ? 'text-success' : 'text-ink-900'}">
          {shippingFree ? m.shipping_sf_free() : m.shipping_sf_cod()}
        </span>
      </div>
      <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-ink-900 text-base">
        <span>{m.checkout_summary_total()}</span>
        <span class="tabular-nums">{loadingVariants ? '—' : formatHKD(total)}</span>
      </div>
      <AppliedPromotions promotions={appliedPromotionsList} />
    </div>
  </div>
{/snippet}

{#snippet stepBadge(n: number, done: boolean, active: boolean)}
  <span class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full border text-sm font-display font-bold transition-colors
               {done ? 'bg-navy-500 border-navy-500 text-white' : active ? 'border-navy-500 text-navy-500' : 'border-gray-200 text-ink-300'}">
    {#if done}✓{:else}{n}{/if}
  </span>
{/snippet}

<div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8 sm:py-10">
  <h1 class="font-display font-bold uppercase tracking-tight text-2xl sm:text-3xl text-ink-900 mb-6">{m.checkout_heading()}</h1>

  <PendingOrderBanner cartId={activeCart?.id ?? null} />

  {#if cartStore.loading && !activeCart}
    <div class="text-center py-20 text-ink-300">{m.common_loading()}</div>

  {:else if !activeCart || activeCart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-ink-300">{m.checkout_cart_empty()}</p>
      <a href="/products"
         class="mt-4 inline-block bg-navy-500 text-white font-medium px-8 py-3 rounded-full hover:bg-navy-700 transition-colors">
        {m.checkout_continue_shopping()}
      </a>
    </div>

  {:else}
    <!-- Mobile collapsible summary (sticky sidebar shows on lg+ instead) -->
    <div class="lg:hidden mb-5">
      <button type="button" onclick={() => (summaryOpen = !summaryOpen)}
              aria-expanded={summaryOpen} aria-controls="checkout-mobile-summary"
              class="w-full flex items-center justify-between gap-3 bg-white rounded-2xl border border-gray-100 px-4 py-3">
        <span class="flex items-center gap-2 text-sm font-medium text-ink-900">
          <svg class="h-4 w-4 text-ink-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4Z"/><path d="M3 6h18"/><path d="M16 10a4 4 0 0 1-8 0"/></svg>
          {m.checkout_summary_heading()}
        </span>
        <span class="flex items-center gap-2">
          <span class="text-sm font-semibold text-ink-900 tabular-nums">{loadingVariants ? '—' : formatHKD(total)}</span>
          <svg class="h-4 w-4 text-ink-500 transition-transform {summaryOpen ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 9 6 6 6-6"/></svg>
        </span>
      </button>
      {#if summaryOpen}
        <div id="checkout-mobile-summary" transition:slide={{ duration: 240, easing: cubicOut }}
             class="bg-white rounded-2xl border border-gray-100 border-t-0 rounded-t-none px-4 pb-4 pt-4">
          {@render orderSummary()}
        </div>
      {/if}
    </div>

    <div class="grid lg:grid-cols-[1fr_360px] gap-6 lg:gap-10 items-start">
      <!-- Left: stepped form -->
      <div class="flex flex-col gap-4 min-w-0">

        <!-- ── Step 1 — Contact & Delivery ─────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-5 sm:p-6">
          <div class="flex items-center justify-between gap-3 {currentStep === 1 ? 'mb-5' : ''}">
            <div class="flex items-center gap-3 min-w-0">
              {@render stepBadge(1, currentStep > 1, currentStep === 1)}
              <div class="min-w-0">
                <Eyebrow tone="muted">Step 1</Eyebrow>
                <h2 class="font-display font-bold uppercase tracking-tight text-ink-900">{m.checkout_modern_step_contact()}</h2>
              </div>
            </div>
            {#if currentStep > 1}
              <button type="button" onclick={editStep1}
                      class="text-sm font-medium text-navy-500 hover:text-navy-700 transition-colors flex-shrink-0">
                {m.common_edit()}
              </button>
            {/if}
          </div>

          {#if currentStep === 1}
            <div class="flex flex-col gap-5">
              <!-- Contact -->
              <div>
                {#if data.customer}
                  <p class="text-xs text-ink-300 mb-3">{m.checkout_logged_in_hint({ email: data.customer.email })}</p>
                {/if}
                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label for="m-first-name" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_label_first_name()} <span class="text-alert">*</span></label>
                    <input id="m-first-name" type="text" bind:value={firstName} required
                           class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                  </div>
                  <div>
                    <label for="m-last-name" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_label_last_name()}</label>
                    <input id="m-last-name" type="text" bind:value={lastName}
                           class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                  </div>
                  <div>
                    <label for="m-phone" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_label_phone()} <span class="text-alert">*</span></label>
                    <input id="m-phone" type="tel" bind:value={phone} required
                           class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                  </div>
                  <div>
                    <label for="m-email" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_label_email()} <span class="text-alert">*</span></label>
                    <input id="m-email" type="email" bind:value={email} required
                           class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                  </div>
                </div>
                {#if !data.customer}
                  <p class="text-xs text-ink-300 mt-3">
                    <a href="/account/login" class="text-navy-500 hover:underline font-medium">{m.checkout_login_hint_pre()}</a>{m.checkout_login_hint_text()}
                  </p>
                {/if}
              </div>

              <!-- Shipping address -->
              <div class="border-t border-gray-100 pt-5">
                <p class="text-xs font-semibold uppercase tracking-[0.12em] text-ink-500 mb-3">{m.checkout_section_shipping()}</p>
                {#if hasSavedAddresses}
                  <div class="flex gap-2 mb-4">
                    <button type="button" onclick={() => (addressMode = 'saved')}
                            class="px-4 py-2 rounded-xl text-sm font-medium transition-colors {addressMode === 'saved' ? 'bg-navy-500 text-white' : 'bg-gray-100 text-ink-500 hover:bg-gray-200'}">
                      {m.checkout_address_saved_tab()}
                    </button>
                    <button type="button" onclick={() => (addressMode = 'new')}
                            class="px-4 py-2 rounded-xl text-sm font-medium transition-colors {addressMode === 'new' ? 'bg-navy-500 text-white' : 'bg-gray-100 text-ink-500 hover:bg-gray-200'}">
                      {m.checkout_address_new_tab()}
                    </button>
                  </div>
                {/if}

                {#if addressMode === 'saved' && hasSavedAddresses}
                  <div class="flex flex-col gap-3">
                    {#each data.addresses ?? [] as addr}
                      <label class="flex items-start gap-3 cursor-pointer p-3 rounded-xl border {selectedAddressID === addr.id ? 'border-navy-500 bg-paper' : 'border-gray-100'}">
                        <input type="radio" name="m_shipping_address" value={addr.id} bind:group={selectedAddressID}
                               class="mt-0.5 accent-navy-500 flex-shrink-0" />
                        <div class="text-sm leading-relaxed flex-1">
                          <span class="font-medium text-ink-900">{addr.first_name} {addr.last_name}</span>
                          {#if addr.is_default}
                            <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-ink-500 text-xs rounded-full">{m.common_default()}</span>
                          {/if}
                          <p class="text-ink-500 mt-0.5">
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
                      <label for="m-line1" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_address_line1()} <span class="text-alert">*</span></label>
                      <input id="m-line1" type="text" bind:value={line1} required placeholder={m.checkout_address_line1_placeholder()}
                             class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                    </div>
                    <div class="grid grid-cols-2 gap-3">
                      <div>
                        <label for="m-city" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_address_city()}</label>
                        <input id="m-city" type="text" bind:value={city} list={cityOptions.length > 0 ? cityListId : undefined}
                               placeholder={country === 'HK' ? m.checkout_address_city_placeholder_hk() : ''} autocomplete="off"
                               class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-navy-500" />
                        {#if cityOptions.length > 0}
                          <datalist id={cityListId}>
                            {#each cityOptions as opt}<option value={opt}></option>{/each}
                          </datalist>
                        {/if}
                      </div>
                      <div>
                        <label for="m-country" class="block text-xs font-medium text-ink-500 mb-1">{m.checkout_address_country()} <span class="text-alert">*</span></label>
                        {#if data.shippingCountries.length === 1}
                          <input id="m-country" type="text" value="{COUNTRY_BY_CODE[data.shippingCountries[0]] ?? data.shippingCountries[0]} ({data.shippingCountries[0]})" readonly
                                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-paper text-ink-500" />
                        {:else}
                          <select id="m-country" bind:value={country} required
                                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-navy-500">
                            {#each data.shippingCountries as code}<option value={code}>{COUNTRY_BY_CODE[code] ?? code} ({code})</option>{/each}
                          </select>
                        {/if}
                      </div>
                    </div>
                    {#if data.customer}
                      <label class="flex items-center gap-2 mt-1 cursor-pointer">
                        <input type="checkbox" bind:checked={saveAddress} class="accent-navy-500" />
                        <span class="text-sm text-ink-500">{m.checkout_address_save_to_profile()}</span>
                      </label>
                    {/if}
                  </div>
                {/if}
              </div>

              <!-- Logistics (read-only admin default) -->
              {#if data.shipanyEnabled}
                <div class="border-t border-gray-100 pt-5">
                  <p class="text-xs font-semibold uppercase tracking-[0.12em] text-ink-500 mb-3">{m.checkout_section_logistics()}</p>
                  {#if data.shippingDefault?.configured}
                    <div class="flex items-center justify-between gap-3">
                      <span class="flex items-center gap-2 font-medium text-ink-900">
                        {#if isSFCourier}
                          <img src="/sf-express-logo.svg" alt={m.checkout_logistics_sf_name()} class="h-6 w-auto flex-shrink-0" />
                          {m.checkout_logistics_sf_name()}
                        {:else}
                          {data.shippingDefault.courier_name}
                        {/if}
                      </span>
                      <span class="text-sm whitespace-nowrap {shippingFree ? 'text-success' : 'text-ink-500'}">
                        {shippingFree ? m.checkout_logistics_free() : m.checkout_logistics_cod()}
                      </span>
                    </div>
                  {:else}
                    <p class="text-sm text-alert">{m.checkout_logistics_not_configured()}</p>
                  {/if}
                  <div class="rounded-xl border border-amber-300/40 bg-amber-300/10 p-3 mt-3">
                    <p class="text-sm leading-relaxed text-ink-700">{@html m.checkout_logistics_sf_locker_notice()}</p>
                  </div>
                </div>
              {/if}

              <!-- Remark -->
              <div class="border-t border-gray-100 pt-5">
                <p class="text-xs font-semibold uppercase tracking-[0.12em] text-ink-500 mb-3">{m.checkout_section_remark()} <span class="font-normal normal-case tracking-normal text-ink-300">{m.common_optional()}</span></p>
                <textarea bind:value={notes} placeholder={m.checkout_remark_placeholder()} rows="2"
                          class="w-full border border-gray-200 rounded-xl px-4 py-3 text-sm text-ink-900 placeholder:text-ink-300 focus:outline-none focus:ring-2 focus:ring-navy-500 resize-none"></textarea>
              </div>

              <button type="button" onclick={goToPayment} disabled={!step1Valid}
                      class="w-full py-3 bg-navy-500 text-white font-display font-bold uppercase tracking-[0.12em] text-sm rounded-xl hover:bg-navy-700 transition-colors disabled:opacity-40">
                {m.checkout_continue_to_payment()}
              </button>
            </div>
          {:else}
            <p class="text-sm text-ink-500 mt-1 truncate">{step1Summary}</p>
          {/if}
        </section>

        <!-- ── Step 2 — Payment ────────────────────────────────── -->
        <section id="checkout-step-2" class="bg-white rounded-2xl border border-gray-100 p-5 sm:p-6 {currentStep < 2 ? 'opacity-50' : ''}">
          <div class="flex items-center gap-3 {currentStep === 2 ? 'mb-5' : ''}">
            {@render stepBadge(2, false, currentStep === 2)}
            <div>
              <Eyebrow tone="muted">Step 2</Eyebrow>
              <h2 class="font-display font-bold uppercase tracking-tight text-ink-900">{m.checkout_section_payment()}</h2>
            </div>
          </div>

          {#if currentStep === 2}
            <div class="flex flex-col gap-5">
              {#if isBankTransfer}
                <BankTransferNotice variant="radio" details={bankDetails} />
              {:else}
                <!-- Inline deferred Payment Element -->
                <SecurePaymentNote />
                <div id="payment-element-modern" class="w-full min-w-0 {paymentElementMounted ? '' : 'hidden'}"></div>
                {#if !paymentElementMounted}
                  <p class="text-sm text-ink-300">{m.common_loading()}</p>
                {/if}
                {#if !stripe && data.paymentConfig?.publishable_key === ''}
                  <p class="text-xs text-alert">{m.checkout_no_payment_setup()}</p>
                {/if}
              {/if}

              <!-- Terms & conditions -->
              <div class="border-t border-gray-100 pt-4">
                <label class="flex items-start gap-3 cursor-pointer">
                  <input type="checkbox" bind:checked={tcAccepted} class="mt-0.5 accent-navy-500 flex-shrink-0" />
                  <span class="text-sm text-ink-700 leading-relaxed">
                    {m.checkout_tc_text_pre()}{#if data.termsPage}<button type="button" onclick={() => (tcExpanded = !tcExpanded)}
                       aria-expanded={tcExpanded} aria-controls="m-checkout-tc-content"
                       class="inline text-navy-500 underline font-medium">{m.checkout_tc_link_label()}</button>{:else}<a
                       href="/pages/terms-and-conditions" target="_blank" class="text-navy-500 underline font-medium">{m.checkout_tc_link_label()}</a>{/if}
                  </span>
                </label>
                {#if tcExpanded && data.termsPage}
                  <div id="m-checkout-tc-content" transition:slide={{ duration: 280, easing: cubicOut }}
                       class="mt-3 max-h-56 overflow-y-auto rounded-xl border border-gray-100 bg-paper px-4 py-3 text-sm text-ink-500 leading-relaxed">
                    <MarkdownContent content={data.termsPage.content} refs={data.termsRefs ?? undefined} />
                  </div>
                {/if}
              </div>

              <!-- Error / pending-order recovery -->
              {#if pendingOrderID && error}
                <div class="rounded-xl border border-amber-300/40 bg-amber-300/10 p-4">
                  <p class="font-semibold text-ink-900">{m.checkout_payment_failed_heading()}</p>
                  <p class="text-sm text-ink-700 mt-1">{m.checkout_payment_failed_order({ orderNumber: pendingOrderNumber })}</p>
                  <p class="text-sm text-ink-700 mt-1">{error}</p>
                  <p class="text-sm text-ink-700 mt-2">{m.checkout_payment_failed_body()}</p>
                  <a href={`/pay/${pendingOrderID}?cs=${pendingClientSecret}`} class="inline-block mt-3 text-sm font-medium text-navy-700 underline">
                    {m.checkout_complete_payment_later()}
                  </a>
                </div>
              {:else if error}
                <p class="text-sm text-alert leading-relaxed">{error}</p>
              {/if}

              {#if isBankTransfer}
                <button type="button" onclick={placeBankTransferOrder} disabled={!formValid || placing}
                        class="w-full py-3.5 bg-navy-500 text-white font-display font-bold uppercase tracking-[0.12em] text-sm rounded-xl hover:bg-navy-700 transition-colors disabled:opacity-40">
                  {placing ? m.checkout_pay_processing() : m.checkout_place_order_bank_transfer()}
                </button>
              {:else}
                {#if paymentElementMounted && !paymentComplete && !placing}
                  <p class="text-xs text-ink-300">{m.checkout_select_payment_hint()}</p>
                {/if}
                <button type="button" onclick={pay} disabled={!formValid || placing || !stripe || !paymentComplete}
                        class="w-full py-3.5 bg-navy-500 text-white font-display font-bold uppercase tracking-[0.12em] text-sm rounded-xl hover:bg-navy-700 transition-colors disabled:opacity-40">
                  {placing ? m.checkout_pay_processing() : m.checkout_pay_button({ amount: roundAmount(total) })}
                </button>
              {/if}
            </div>
          {/if}
        </section>

        <a href="/cart" class="text-center text-sm text-ink-300 hover:text-ink-700 transition-colors">{m.checkout_back_to_cart()}</a>
      </div>

      <!-- Right: sticky summary (lg+) -->
      <aside class="hidden lg:block">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 lg:sticky lg:top-24">
          <h2 class="font-display font-bold uppercase tracking-tight text-ink-900 mb-4">{m.checkout_summary_heading()}</h2>
          {@render orderSummary()}
        </div>
      </aside>
    </div>
  {/if}
</div>
