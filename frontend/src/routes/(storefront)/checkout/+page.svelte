<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { goto } from '$app/navigation';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { checkout, getShipanyQuote, getVariantByID, validateCoupon, type ShipanyPickupPoint, type ShipanyRateOption } from '$lib/api';
  import { loadStripe, type Stripe, type StripeElements } from '@stripe/stripe-js';
  import type { PageData } from './$types';
  import type { SavedPaymentMethod, Variant } from '$lib/types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';
  import PickupPointPicker from '$lib/components/PickupPointPicker.svelte';
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
  let addressState = $state('');
  let postalCode = $state('');
  let country = $state(data.shippingCountries[0] ?? 'HK');
  let saveAddress = $state(false);

  const cityListId = 'checkout-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);

  // ── ShipAny delivery method (section 3) ───────────────────────
  let rateOptions = $state<ShipanyRateOption[]>([]);
  let selectedRate = $state<ShipanyRateOption | null>(null);
  let quoteLoading = $state(false);
  let quoteError = $state('');
  let quoteFetched = $state(false);
  let pickupPoint = $state<ShipanyPickupPoint | null>(null);
  let pickupPickerOpen = $state(false);
  let quoteTimer: ReturnType<typeof setTimeout> | null = null;

  // ── Remark (section 4) ────────────────────────────────────────
  let notes = $state('');

  // ── Coupon ────────────────────────────────────────────────────
  let couponCode = $state('');
  let couponResult = $state<{
    valid: boolean;
    discount_amount?: number;
    message?: string;
  } | null>(null);
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
  let paymentElementMounted = $state(false);
  // Set when the backend returns a SetupIntent for saving the card
  let pendingSetupClientSecret = $state<string | null>(null);

  // ── T&C (section 5) ───────────────────────────────────────────
  let tcAccepted = $state(false);

  // ── Submit state ──────────────────────────────────────────────
  let placing = $state(false);
  let error = $state('');

  const activeCart = $derived(cartStore.cart);

  const subtotal = $derived(
    activeCart?.items?.reduce((sum, item) => {
      const v = variantMap[item.variant_id];
      return sum + (v ? v.price * item.quantity : 0);
    }, 0) ?? 0
  );
  const discount = $derived(couponResult?.valid ? (couponResult.discount_amount ?? 0) : 0);
  const total = $derived(subtotal - discount);

  const freeShippingThreshold = $derived(() => {
    const raw = (data.publicSettings ?? []).find((s) => s.key === 'free_shipping_threshold_hkd')?.value;
    const n = raw ? Number(raw) : 0;
    return Number.isFinite(n) && n > 0 ? n : 0;
  });
  const shippingFree = $derived(
    freeShippingThreshold() > 0 && subtotal >= freeShippingThreshold()
  );

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

  // Delivery method is only required when ShipAny is enabled. If the
  // selected rate needs a pickup point, a point must also be chosen.
  const deliveryValid = $derived(
    !data.shipanyEnabled
      ? true
      : selectedRate !== null && (!selectedRate.requires_pickup_point || pickupPoint !== null)
  );

  const formValid = $derived(customerValid && shippingValid && deliveryValid && tcAccepted);

  // Resolve the address payload that will be sent to the rate quoter.
  // For "saved" mode we look up the chosen address; for "new" mode we use
  // the live form values.
  const quoteAddress = $derived.by(() => {
    if (addressMode === 'saved') {
      const a = data.addresses?.find((x) => x.id === selectedAddressID);
      if (!a) return null;
      return {
        line1: a.line1, line2: a.line2 ?? undefined,
        city: a.city, district: a.city,
        postal_code: a.postal_code ?? '',
        country: a.country
      };
    }
    if (line1.trim() === '' || country.trim() === '') return null;
    return {
      line1: line1.trim(), city: city.trim(),
      district: city.trim(),
      postal_code: postalCode.trim(),
      country: country.trim() || 'HK'
    };
  });

  // Re-fetch quotes whenever the destination changes, debounced.
  $effect(() => {
    if (!data.shipanyEnabled || !activeCart) return;
    const addr = quoteAddress;
    if (!addr) return;
    untrack(() => { selectedRate = null; pickupPoint = null; });
    if (quoteTimer) clearTimeout(quoteTimer);
    quoteTimer = setTimeout(() => {
      void refreshQuote(addr);
    }, 500);
  });

  async function refreshQuote(addr: NonNullable<typeof quoteAddress>) {
    if (!activeCart) return;
    quoteLoading = true;
    quoteError = '';
    try {
      const rates = await getShipanyQuote(activeCart.id, {
        line1: addr.line1, line2: addr.line2,
        city: addr.city, district: addr.district,
        postal_code: addr.postal_code,
        country: addr.country
      });
      rateOptions = rates;
      quoteFetched = true;
    } catch (e) {
      quoteError = e instanceof Error ? e.message : m.checkout_quote_failed();
      rateOptions = [];
    } finally {
      quoteLoading = false;
    }
  }

  function handlePickupSelected(p: ShipanyPickupPoint) {
    pickupPoint = p;
    pickupPickerOpen = false;
  }

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
        : { valid: false, message: res.message ?? m.checkout_invalid_coupon() };
    } catch {
      couponResult = { valid: false, message: m.checkout_coupon_validate_failed() };
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
                state: addressState.trim() || undefined,
                postal_code: postalCode.trim(),
                country: country.trim() || 'HK'
              }
            : undefined,
        saveAddress: addressMode === 'new' && saveAddress,
        shippingFee: 0,
        couponCode: couponResult?.valid ? couponCode.trim() : undefined,
        notes: notes.trim() || undefined,
        saveCard: data.saveCardsEnabled && cardMode === 'new' && saveCard && !!data.customer,
        savedPaymentMethodId: selectedCard?.stripe_pm_id,
        selectedCarrier: selectedRate?.carrier,
        selectedService: selectedRate?.service,
        pickupPointId: pickupPoint?.id,
        pickupPointLabel: pickupPoint ? `${pickupPoint.name} — ${pickupPoint.address}` : undefined
      });

      pendingClientSecret = result.client_secret;
      pendingOrderID = result.order.id;
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
                  <label for="state" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_address_state()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
                  <input id="state" type="text" bind:value={addressState}
                         class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
                </div>
                <div>
                  <label for="postal" class="block text-xs font-medium text-gray-500 mb-1">{m.checkout_address_postal()}</label>
                  <input id="postal" type="text" bind:value={postalCode}
                         class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                                focus:outline-none focus:ring-2 focus:ring-gray-900" />
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

        <!-- ── 3. Delivery method (ShipAny) ─────────────────────── -->
        {#if data.shipanyEnabled}
          <section class="bg-white rounded-2xl border border-gray-100 p-6">
            <div class="flex items-center gap-3 mb-4">
              <span class="flex items-center justify-center w-7 h-7 rounded-full bg-gray-900 text-white text-xs font-semibold">3</span>
              <h2 class="font-semibold text-gray-900">{m.checkout_section_delivery()}</h2>
            </div>

            {#if !shippingValid}
              <p class="text-sm text-gray-400">{m.checkout_delivery_need_address()}</p>
            {:else if quoteLoading && rateOptions.length === 0}
              <div class="flex flex-col gap-2">
                {#each [0, 1, 2] as _}
                  <div class="h-14 bg-gray-100 animate-pulse rounded-xl"></div>
                {/each}
              </div>
            {:else if quoteError}
              <p class="text-sm text-amber-600">
                {m.checkout_delivery_quote_warning({ error: quoteError })}
              </p>
            {:else if quoteFetched && rateOptions.length === 0}
              <p class="text-sm text-gray-500">{m.checkout_delivery_no_options()}</p>
            {:else}
              <div class="flex flex-col gap-2">
                {#each rateOptions as rate}
                  <label class="flex items-start gap-3 cursor-pointer p-3 rounded-xl border
                                {selectedRate?.carrier === rate.carrier && selectedRate?.service === rate.service
                                  ? 'border-gray-900 bg-gray-50'
                                  : 'border-gray-100 hover:border-gray-300'}">
                    <input type="radio" name="shipany_rate"
                           value="{rate.carrier}::{rate.service}"
                           checked={selectedRate?.carrier === rate.carrier && selectedRate?.service === rate.service}
                           onchange={() => { selectedRate = rate; pickupPoint = null; }}
                           class="mt-0.5 accent-gray-900 flex-shrink-0" />
                    <div class="flex-1 text-sm">
                      <div class="flex items-baseline justify-between gap-3">
                        <span class="font-medium text-gray-900">{rate.carrier_name}</span>
                        <span class="font-semibold text-gray-900 whitespace-nowrap">HK${rate.fee_hkd.toFixed(2)}</span>
                      </div>
                      <p class="text-xs text-gray-500 mt-0.5">
                        {rate.service_name}{#if rate.eta_days} · {m.checkout_delivery_eta_days({ days: rate.eta_days })}{/if}
                      </p>
                      {#if selectedRate?.carrier === rate.carrier && selectedRate?.service === rate.service && rate.requires_pickup_point}
                        <div class="mt-2 flex items-center gap-3">
                          {#if pickupPoint}
                            <div class="text-xs text-gray-700">
                              <p class="font-medium">{pickupPoint.name}</p>
                              <p class="text-gray-500">{pickupPoint.address}</p>
                            </div>
                            <button type="button"
                                    onclick={(e) => { e.preventDefault(); pickupPickerOpen = true; }}
                                    class="text-xs text-gray-700 underline hover:text-gray-900">{m.checkout_delivery_change_pickup()}</button>
                          {:else}
                            <button type="button"
                                    onclick={(e) => { e.preventDefault(); pickupPickerOpen = true; }}
                                    class="text-xs text-gray-700 underline hover:text-gray-900">{m.checkout_delivery_pick_pickup()}</button>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </label>
                {/each}
              </div>
            {/if}
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
              {m.checkout_tc_text_pre()}<a href="/pages/terms-and-conditions" target="_blank"
                 class="text-gray-900 underline font-medium">{m.checkout_tc_link_label()}</a>
            </span>
          </label>
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
            {#if error}
              <p class="text-sm text-red-500 leading-relaxed">{error}</p>
            {/if}

            {#if paymentReady}
              <button type="button"
                      onclick={confirmPay}
                      disabled={placing || !tcAccepted}
                      class="{error ? 'mt-5' : ''} w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                             hover:bg-gray-700 transition-colors disabled:opacity-50">
                {placing ? m.checkout_pay_processing() : m.checkout_pay_button({ amount: total.toFixed(2) })}
              </button>
            {/if}
          </section>
        {/if}
      </div>

      <!-- Right: Order summary -->
      <div class="lg:w-2/5 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 sticky top-24">
          <h2 class="font-semibold text-gray-900">{m.checkout_summary_heading()}</h2>

          <div class="flex flex-col gap-3">
            {#if loadingVariants}
              <p class="text-sm text-gray-400">{m.common_loading()}</p>
            {:else if activeCart}
              {#each activeCart.items as item}
                {@const variant = variantMap[item.variant_id]}
                <div class="flex items-start justify-between gap-3 text-sm">
                  <div class="min-w-0">
                    <p class="font-medium text-gray-900 truncate">{variant?.product_name ?? variant?.sku ?? item.variant_id.slice(0, 8) + '…'}</p>
                    <p class="text-xs text-gray-400">{m.checkout_summary_qty({ quantity: item.quantity })}</p>
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
              {#if couponResult && !couponResult.valid}
                <p class="mt-1.5 text-xs text-red-500">{couponResult.message}</p>
              {/if}
            {/if}
          </div>

          <!-- Totals -->
          <div class="border-t border-gray-100 pt-3 flex flex-col gap-2">
            <div class="flex justify-between text-sm text-gray-600">
              <span>{m.checkout_summary_subtotal()}</span>
              <span>{loadingVariants ? '—' : `HK$${subtotal.toFixed(2)}`}</span>
            </div>
            {#if discount > 0}
              <div class="flex justify-between text-sm text-green-600">
                <span>{m.checkout_summary_discount()}</span>
                <span>−HK${discount.toFixed(2)}</span>
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
              <span>{loadingVariants ? '—' : `HK$${total.toFixed(2)}`}</span>
            </div>
          </div>

          <a href="/cart" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            {m.checkout_back_to_cart()}
          </a>
        </div>
      </div>
    </div>
  {/if}
</div>

{#if pickupPickerOpen && selectedRate}
  <PickupPointPicker
    carrier={selectedRate.carrier}
    carrierName={selectedRate.carrier_name}
    initialDistrict={addressMode === 'new' ? city.trim() : (data.addresses?.find((a) => a.id === selectedAddressID)?.city ?? '')}
    onSelect={handlePickupSelected}
    onClose={() => (pickupPickerOpen = false)} />
{/if}
