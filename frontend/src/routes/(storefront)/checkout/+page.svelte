<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { formatHKD, roundAmount } from '$lib/money';
  import { checkout, getVariantByID, quoteOrder, type QuoteResult } from '$lib/api';
  import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
  import type { PageData } from './$types';
  import type { SavedPaymentMethod, Variant } from '$lib/types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';
  import { productDisplayName } from '$lib/variant';
  import { resolveFreeShippingThreshold } from '$lib/shippingThreshold';
  import AppliedPromotions from '$lib/components/AppliedPromotions.svelte';
  import PendingOrderBanner from '$lib/components/shop/PendingOrderBanner.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import { slide } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';

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
  let country = $state(data.shippingCountries[0] ?? 'HK');
  let saveAddress = $state(false);

  const cityListId = 'checkout-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);

  // ── Logistics (section 3) ─────────────────────────────────────
  // Customers no longer pick a courier — checkout always ships via the
  // admin-configured default. The block path triggers when admin hasn't
  // filled in shipany_default_courier / shipany_default_service.
  const shippingConfigured = $derived(
    !data.shipanyEnabled || (data.shippingDefault?.configured ?? false)
  );
  // Only brand the row with the SF logo when the configured courier is actually SF.
  const isSFCourier = $derived(/sf|順豐/i.test(data.shippingDefault?.courier_name ?? ''));

  // ── Remark (section 4) ────────────────────────────────────────
  let notes = $state('');

  // ── Coupon ────────────────────────────────────────────────────
  // couponCode is the user's input field; appliedCouponCode is the code
  // we've actually submitted to the backend's /orders/quote. Splitting
  // them lets the user keep editing without re-quoting until they click
  // Apply. Empty applied code means "no coupon" — campaigns still apply.
  let couponCode = $state('');
  let appliedCouponCode = $state('');
  let validatingCoupon = $state(false);

  // ── Saved cards (section 4) ──────────────────────────────────
  const savedCards: SavedPaymentMethod[] = data.savedCards ?? [];
  const hasSavedCards = savedCards.length > 0;
  type CardMode = 'saved' | 'new';
  let cardMode = $state<CardMode>(hasSavedCards ? 'saved' : 'new');
  let selectedCardID = $state<string>(
    savedCards.find((c) => c.is_default)?.id ?? savedCards[0]?.id ?? ''
  );
  let saveCard = $state(false);

  // ── Stripe Payment Element (section 4) ────────────────────────
  let stripe: Stripe | null = $state(null);
  let elements: StripeElements | null = $state(null);
  let paymentReady = $state(false);
  let paymentMounting = $state(false);
  let pendingClientSecret = $state<string | null>(null);
  let pendingOrderID = $state<string | null>(null);
  let pendingOrderNumber = $state('');
  let paymentElementMounted = $state(false);
  // Set when the backend returns a SetupIntent for saving the card
  let pendingSetupClientSecret = $state<string | null>(null);

  // ── T&C (section 5) ───────────────────────────────────────────
  let tcAccepted = $state(false);
  let tcExpanded = $state(false);

  // ── Submit state ──────────────────────────────────────────────
  let placing = $state(false);
  let error = $state('');

  const activeCart = $derived(cartStore.cart);

  // ── Server quote ──────────────────────────────────────────────
  // The backend's POST /orders/quote runs the exact same pricing rules as
  // Checkout (campaigns + coupon + free-shipping threshold + tax) and
  // returns the breakdown so this page can show the discount line and
  // promotion descriptions BEFORE payment. Falls back to the client-side
  // subtotal only until the first quote arrives, to avoid a "—" flash.
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

  // Coupon UI state derives from the quote. If we've applied a code and
  // the server returned an applied_coupon (no coupon_error), it's valid.
  const couponValid = $derived(
    appliedCouponCode !== '' && !!quote?.applied_coupon && !(quote?.coupon_error)
  );
  const couponDiscountAmount = $derived(quote?.applied_coupon?.amount ?? 0);
  const couponErrorMsg = $derived(() => {
    if (appliedCouponCode === '' || !quote || !quote.coupon_error) return null;
    if (quote.coupon_error_code === 'wrong_role') return m.storefront_coupon_wrong_role();
    return quote.coupon_error || m.checkout_invalid_coupon();
  });

  // Build the merged promotions list for the description block. Campaigns
  // first (the order they applied), coupon last. Each item gets the kind
  // the AppliedPromotions component will use to label it generically.
  const appliedPromotionsList = $derived(
    [
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
    ]
  );

  // Fallback free-shipping decision used only until the first quote loads.
  // This client-side calc uses pre-discount subtotal (legacy behaviour);
  // once quote arrives we use the server's post-discount value so the
  // displayed SF label matches what the order will actually be charged.
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
      // Race-guard: if a newer quote was kicked off while this was in
      // flight, drop this stale response.
      if (v === quoteVersion) quote = res;
    } catch {
      // Network blip — keep the last good quote rather than flashing the
      // client-side fallback.
    }
  }

  $effect(() => {
    // Track the cart items signature + applied coupon so any change to
    // either re-quotes. Reads are intentional: they register dependencies.
    const items = cartStore.cart?.items ?? [];
    const sig = items.map((i) => `${i.variant_id}:${i.quantity}`).join('|');
    void sig;
    void appliedCouponCode;
    const t = setTimeout(refreshQuote, 200);
    return () => clearTimeout(t);
  });

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

  const formValid = $derived(customerValid && shippingValid && shippingConfigured && tcAccepted);

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
    const code = couponCode.trim();
    if (!code) return;
    validatingCoupon = true;
    try {
      // Set the applied code first so refreshQuote sends it to the
      // backend. The /orders/quote response carries validity + the
      // discount amount, replacing the old standalone /validate-coupon
      // call.
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

  // Step 1: create the order + PaymentIntent, mount Payment Element
  async function continueToPayment() {
    if (!activeCart || !stripe) {
      error = stripe ? m.checkout_cart_empty_error() : m.checkout_no_payment_setup();
      return;
    }
    if (!formValid) return;

    paymentMounting = true;
    error = '';
    try {
      // When using a saved card skip the payment element entirely —
      // the backend will confirm the intent with the saved payment method.
      const usingSavedCard = data.saveCardsEnabled && hasSavedCards && cardMode === 'saved' && selectedCardID;
      const selectedCard = usingSavedCard ? savedCards.find((c) => c.id === selectedCardID) : null;

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
                country: country.trim() || 'HK'
              }
            : undefined,
        saveAddress: addressMode === 'new' && saveAddress,
        shippingFee: 0,
        couponCode: couponValid ? appliedCouponCode : undefined,
        notes: notes.trim() || undefined,
        saveCard: data.saveCardsEnabled && cardMode === 'new' && saveCard && !!data.customer,
        savedPaymentMethodId: selectedCard?.stripe_pm_id
      });

      pendingClientSecret = result.client_secret;
      pendingOrderID = result.order.id;
      pendingOrderNumber = result.order.order_number || `ORD-${result.order.number}`;
      pendingSetupClientSecret = result.setup_client_secret ?? null;

      if (!usingSavedCard) {
        const stripeCountry = data.paymentConfig?.country || 'HK';
        elements = stripe.elements({
          clientSecret: result.client_secret,
          appearance: { theme: 'stripe', variables: { colorPrimary: '#111827' } }
        });
        const paymentElement = elements.create('payment', {
          layout: 'tabs',
          defaultValues: {
            billingDetails: { address: { country: stripeCountry } }
          }
        });
        paymentElement.mount('#payment-element');
        paymentElementMounted = true;
      }
      paymentReady = true;

      // Scroll to payment section
      requestAnimationFrame(() => {
        document.getElementById('payment-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    } catch (e) {
      error = e instanceof Error ? e.message : m.checkout_payment_start_failed();
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
        error = stripeError.message ?? m.checkout_payment_failed();
      }
    } catch (e) {
      error = e instanceof Error ? e.message : m.checkout_payment_failed();
    } finally {
      placing = false;
    }
  }
</script>

<svelte:head>
  <title>{m.checkout_title()}</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-8">{m.checkout_heading()}</h1>

  <PendingOrderBanner cartId={activeCart?.id ?? null} />

  {#if cartStore.loading && !activeCart}
    <div class="text-center py-20 text-gray-400">{m.common_loading()}</div>

  {:else if !activeCart || activeCart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-gray-400">{m.checkout_cart_empty()}</p>
      <a href="/products"
         class="mt-4 inline-block bg-gray-900 text-white font-medium px-8 py-3 rounded-full hover:bg-gray-700 transition-colors">
        {m.checkout_continue_shopping()}
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
            <h2 class="font-semibold text-gray-900">{m.checkout_section_customer()}</h2>
          </div>
          {#if data.customer}
            <p class="text-xs text-gray-400 mb-4">
              {m.checkout_logged_in_hint({ email: data.customer.email })}
            </p>
          {/if}
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="first-name" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_label_first_name()} <span class="text-red-400">*</span></label>
              <input id="first-name" type="text" bind:value={firstName} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="last-name" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_label_last_name()}</label>
              <input id="last-name" type="text" bind:value={lastName}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="phone" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_label_phone()} <span class="text-red-400">*</span></label>
              <input id="phone" type="tel" bind:value={phone} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div>
              <label for="email" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_label_email()} <span class="text-red-400">*</span></label>
              <input id="email" type="email" bind:value={email} required
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
          {#if !data.customer}
            <p class="text-xs text-gray-400 mt-3">
              <a href="/account/login" class="text-gray-900 hover:underline font-medium">{m.checkout_login_hint_pre()}</a>{m.checkout_login_hint_text()}
            </p>
          {/if}
        </section>

        <!-- ── 2. Shipping Address ─────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">2</span>
            <h2 class="font-semibold text-gray-900">{m.checkout_section_shipping()}</h2>
          </div>

          {#if hasSavedAddresses}
            <div class="flex gap-2 mb-4">
              <button type="button"
                      onclick={() => (addressMode = 'saved')}
                      class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                             {addressMode === 'saved' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                {m.checkout_address_saved_tab()}
              </button>
              <button type="button"
                      onclick={() => (addressMode = 'new')}
                      class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                             {addressMode === 'new' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                {m.checkout_address_new_tab()}
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
                      <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded-full">{m.common_default()}</span>
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
                <label for="line1" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_address_line1()} <span class="text-red-400">*</span></label>
                <input id="line1" type="text" bind:value={line1} required placeholder={m.checkout_address_line1_placeholder()}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label for="city" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_address_city()}</label>
                  <input id="city" type="text" bind:value={city} list={cityOptions.length > 0 ? cityListId : undefined}
                         placeholder={country === 'HK' ? m.checkout_address_city_placeholder_hk() : ''} autocomplete="off"
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
                  <label for="country" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_address_country()} <span class="text-red-400">*</span></label>
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
                  <span class="text-sm text-gray-600">{m.checkout_address_save_to_profile()}</span>
                </label>
              {/if}
            </div>
          {/if}
        </section>

        <!-- ── 3. Logistics (read-only, admin default) ─────────── -->
        {#if data.shipanyEnabled}
          <section class="bg-white rounded-2xl border border-gray-100 p-6">
            <div class="flex items-center gap-3 mb-4">
              <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">3</span>
              <h2 class="font-semibold text-gray-900">{m.checkout_section_logistics()}</h2>
            </div>

            {#if data.shippingDefault?.configured}
              <div class="flex items-center justify-between gap-3">
                <span class="flex items-center gap-2 font-medium text-gray-900">
                  {#if isSFCourier}
                    <img src="/sf-express-logo.svg" alt="SF Express" class="h-6 w-auto flex-shrink-0" />
                  {/if}
                  {data.shippingDefault.courier_name}
                </span>
                <span class="text-sm whitespace-nowrap {shippingFree ? 'text-green-600' : 'text-gray-600'}">
                  {shippingFree ? m.checkout_logistics_free() : m.checkout_logistics_cod()}
                </span>
              </div>
            {:else}
              <p class="text-sm text-red-500">{m.checkout_logistics_not_configured()}</p>
            {/if}

            <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 mt-4">
              <p class="text-sm leading-relaxed text-amber-800">{@html m.checkout_logistics_sf_locker_notice()}</p>
            </div>
          </section>
        {/if}

        <!-- ── 4. Remark ───────────────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">{data.shipanyEnabled ? '4' : '3'}</span>
            <h2 class="font-semibold text-gray-900">{m.checkout_section_remark()} <span class="font-normal text-gray-400 text-sm">{m.common_optional()}</span></h2>
          </div>
          <textarea bind:value={notes}
                    placeholder={m.checkout_remark_placeholder()}
                    rows="3"
                    class="w-full border border-gray-200 rounded-xl px-4 py-3 text-sm text-gray-900 placeholder:text-gray-300
                           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent resize-none"></textarea>
        </section>

        <!-- ── T&C ─────────────────────────────────────────────── -->
        <section class="bg-white rounded-2xl border border-gray-100 p-6">
          <label class="flex items-start gap-3 cursor-pointer">
            <input type="checkbox" bind:checked={tcAccepted}
                   class="mt-0.5 accent-gray-900 flex-shrink-0" />
            <span class="text-sm text-gray-700 leading-relaxed">
              {m.checkout_tc_text_pre()}{#if data.termsPage}<button type="button"
                 onclick={() => (tcExpanded = !tcExpanded)}
                 aria-expanded={tcExpanded} aria-controls="checkout-tc-content"
                 class="inline text-gray-900 underline font-medium">{m.checkout_tc_link_label()}</button>{:else}<a
                 href="/pages/terms-and-conditions" target="_blank"
                 class="text-gray-900 underline font-medium">{m.checkout_tc_link_label()}</a>{/if}
            </span>
          </label>
          {#if tcExpanded && data.termsPage}
            <div id="checkout-tc-content"
                 transition:slide={{ duration: 280, easing: cubicOut }}
                 class="mt-3 max-h-56 overflow-y-auto rounded-xl border border-gray-100 bg-gray-50
                        px-4 py-3 text-sm text-gray-600 leading-relaxed">
              <MarkdownContent content={data.termsPage.content} refs={data.termsRefs ?? undefined} />
            </div>
          {/if}
        </section>

        <!-- ── 5. Payment ──────────────────────────────────────── -->
        <section id="payment-section" class="bg-white rounded-2xl border border-gray-100 p-6">
          <div class="flex items-center gap-3 mb-4">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">{data.shipanyEnabled ? '5' : '4'}</span>
            <h2 class="font-semibold text-gray-900">{m.checkout_section_payment()}</h2>
          </div>

          {#if !paymentReady}
            <!-- Saved cards tabs (only for logged-in customers with save_cards enabled) -->
            {#if data.saveCardsEnabled && data.customer && hasSavedCards}
              <div class="flex gap-2 mb-4">
                <button type="button"
                        onclick={() => (cardMode = 'saved')}
                        class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                               {cardMode === 'saved' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                  {m.checkout_card_saved_tab()}
                </button>
                <button type="button"
                        onclick={() => (cardMode = 'new')}
                        class="px-4 py-2 rounded-xl text-sm font-medium transition-colors
                               {cardMode === 'new' ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
                  {m.checkout_card_new_tab()}
                </button>
              </div>
            {/if}

            <!-- Saved card list -->
            {#if data.saveCardsEnabled && data.customer && hasSavedCards && cardMode === 'saved'}
              <div class="flex flex-col gap-2 mb-4">
                {#each savedCards as card}
                  <label class="flex items-center gap-3 cursor-pointer p-3 rounded-xl border
                                {selectedCardID === card.id ? 'border-gray-900 bg-gray-50' : 'border-gray-100'}">
                    <input type="radio" name="saved_card" value={card.id}
                           bind:group={selectedCardID}
                           class="accent-gray-900 flex-shrink-0" />
                    <div class="flex-1 text-sm">
                      <span class="font-medium text-gray-900 capitalize">{card.brand}</span>
                      <span class="text-gray-500 ml-1">•••• {card.last4}</span>
                      {#if card.is_default}
                        <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded-full">{m.common_default()}</span>
                      {/if}
                      <p class="text-gray-400 text-xs mt-0.5">{m.checkout_card_expires({ month: card.exp_month, year: card.exp_year })}</p>
                    </div>
                  </label>
                {/each}
              </div>
            {/if}

            <!-- Save card checkbox (new card mode, logged-in, feature enabled) -->
            {#if data.saveCardsEnabled && data.customer && cardMode === 'new'}
              <label class="flex items-center gap-2 mb-4 cursor-pointer">
                <input type="checkbox" bind:checked={saveCard} class="accent-gray-900" />
                <span class="text-sm text-gray-600">{m.checkout_card_save_for_later()}</span>
              </label>
            {/if}

            <p class="text-xs text-gray-400 mb-4">
              {cardMode === 'saved' && data.saveCardsEnabled && data.customer && hasSavedCards
                ? m.checkout_payment_hint_saved()
                : m.checkout_payment_hint_new()}
            </p>
            <button type="button"
                    onclick={continueToPayment}
                    disabled={!formValid || paymentMounting || !stripe}
                    class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                           hover:bg-gray-700 transition-colors disabled:opacity-50">
              {paymentMounting ? m.checkout_preparing_payment() : m.checkout_continue_to_payment()}
            </button>
            {#if !stripe && data.paymentConfig?.publishable_key === ''}
              <p class="mt-3 text-xs text-red-500">{m.checkout_no_payment_setup()}</p>
            {/if}
          {/if}

          <!-- Stripe Payment Element (hidden when using saved card) -->
          <div id="payment-element" class="{paymentReady && paymentElementMounted ? '' : 'hidden'}"></div>

          <!-- Saved card confirmation message -->
          {#if paymentReady && !paymentElementMounted}
            {@const card = savedCards.find((c) => c.id === selectedCardID)}
            {#if card}
              <div class="flex items-center gap-3 p-3 rounded-xl bg-gray-50 border border-gray-200 text-sm text-gray-700">
                <span class="capitalize font-medium">{card.brand}</span>
                <span>•••• {card.last4}</span>
                <span class="text-gray-400 ml-auto">{m.checkout_card_expires({ month: card.exp_month, year: card.exp_year })}</span>
              </div>
            {/if}
          {/if}
        </section>

        <!-- ── Pay ─────────────────────────────────────────────── -->
        {#if error || paymentReady}
          <section class="bg-white rounded-2xl border border-gray-100 p-6">
            {#if pendingOrderID && error}
              <!-- Payment failed AFTER the order was created. Reassure the
                   shopper the order is reserved (not lost) and give a durable
                   way to finish paying, so they don't think nothing happened. -->
              <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 mb-5">
                <p class="font-semibold text-amber-900">{m.checkout_payment_failed_heading()}</p>
                <p class="text-sm text-amber-800 mt-1">{m.checkout_payment_failed_order({ orderNumber: pendingOrderNumber })}</p>
                <p class="text-sm text-amber-800 mt-1">{error}</p>
                <p class="text-sm text-amber-800 mt-2">{m.checkout_payment_failed_body()}</p>
                <a href={`/pay/${pendingOrderID}?cs=${pendingClientSecret}`}
                   class="inline-block mt-3 text-sm font-medium text-amber-900 underline">
                  {m.checkout_complete_payment_later()}
                </a>
              </div>
            {:else if error}
              <p class="text-sm text-red-500 leading-relaxed">{error}</p>
            {/if}

            {#if paymentReady}
              <button type="button"
                      onclick={confirmPay}
                      disabled={placing || !tcAccepted}
                      class="{error ? 'mt-5' : ''} w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                             hover:bg-gray-700 transition-colors disabled:opacity-50">
                {placing ? m.checkout_pay_processing() : m.checkout_pay_button({ amount: roundAmount(total) })}
              </button>
            {/if}
          </section>
        {/if}
      </div>

      <!-- Right: Order summary -->
      <div class="lg:w-2/5 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 lg:sticky lg:top-24">
          <h2 class="font-semibold text-gray-900">{m.checkout_summary_heading()}</h2>

          <div class="flex flex-col gap-3">
            {#if loadingVariants}
              <p class="text-sm text-gray-400">{m.common_loading()}</p>
            {:else if activeCart}
              {#each activeCart.items as item}
                {@const variant = variantMap[item.variant_id]}
                <div class="flex flex-col gap-1.5">
                  <div class="flex items-start justify-between gap-3 text-sm">
                    <div class="min-w-0">
                      <p class="font-medium text-gray-900 truncate uppercase">{variant?.product_name ? productDisplayName(variant.product_name, variant.name) : variant?.sku ?? item.variant_id.slice(0, 8) + '…'}</p>
                      {#if item.product_subtitle}
                        <p class="text-xs text-gray-500 truncate">{item.product_subtitle}</p>
                      {/if}
                      <p class="text-xs text-gray-400">{m.checkout_summary_qty({ quantity: item.quantity })}</p>
                    </div>
                    <span class="text-gray-900 font-medium flex-shrink-0 tabular-nums">
                      {variant ? formatHKD(variant.price * item.quantity) : '—'}
                    </span>
                  </div>
                  {#if item.children?.length}
                    <ul class="pl-3 flex flex-col gap-0.5 border-l border-gray-100">
                      {#each item.children as child}
                        <li class="flex items-center justify-between gap-3 text-xs text-gray-500">
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
              <div class="flex items-center justify-between bg-green-50 border border-green-100 rounded-xl px-3 py-2">
                <div>
                  <p class="text-xs font-medium text-green-800">{appliedCouponCode}</p>
                  <p class="text-[11px] text-green-600 tabular-nums">−{formatHKD(couponDiscountAmount)}</p>
                </div>
                <button type="button" onclick={removeCoupon} class="text-xs text-green-600 hover:text-green-900">{m.checkout_coupon_remove()}</button>
              </div>
            {:else}
              <div class="flex gap-2">
                <input type="text" bind:value={couponCode} placeholder={m.checkout_coupon_placeholder()}
                       class="w-full flex-1 border border-gray-200 rounded-xl px-3 py-2 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900"
                       onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyCoupon())} />
                <button type="button" onclick={applyCoupon}
                        disabled={validatingCoupon || !couponCode.trim()}
                        class="px-3 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-gray-700 disabled:opacity-40">
                  {validatingCoupon ? m.checkout_coupon_applying() : m.checkout_coupon_apply()}
                </button>
              </div>
              {#if couponErrorMsg()}
                <p class="mt-1.5 text-xs text-red-500">{couponErrorMsg()}</p>
              {/if}
            {/if}
          </div>

          <!-- Totals -->
          <div class="border-t border-gray-100 pt-3 flex flex-col gap-2">
            <div class="flex justify-between text-sm text-gray-600">
              <span>{m.checkout_summary_subtotal()}</span>
              <span class="tabular-nums">{loadingVariants ? '—' : formatHKD(subtotal)}</span>
            </div>
            {#if discount > 0}
              <div class="flex justify-between text-sm text-green-600">
                <span>{m.checkout_summary_discount()}</span>
                <span class="tabular-nums">−{formatHKD(discount)}</span>
              </div>
            {/if}
            <div class="flex justify-between text-sm text-gray-600">
              <span>{m.checkout_summary_shipping()}</span>
              <span class="whitespace-nowrap {shippingFree ? 'text-green-600' : 'text-gray-900'}">
                {shippingFree ? m.shipping_sf_free() : m.shipping_sf_cod()}
              </span>
            </div>
            <div class="border-t border-gray-100 pt-2 flex justify-between font-semibold text-gray-900 text-base">
              <span>{m.checkout_summary_total()}</span>
              <span class="tabular-nums">{loadingVariants ? '—' : formatHKD(total)}</span>
            </div>
            <AppliedPromotions promotions={appliedPromotionsList} />
          </div>

          <a href="/cart" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            {m.checkout_back_to_cart()}
          </a>
        </div>
      </div>
    </div>
  {/if}
</div>

