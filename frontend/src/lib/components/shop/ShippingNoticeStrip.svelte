<script lang="ts">
  /*
   * Second site-wide notice strip — rendered below AnnouncementStrip.
   * Inherits visibility from the Logistics-tab free-shipping threshold
   * (free_shipping_threshold_enabled + free_shipping_threshold_hkd).
   * Switches between "spend X more for free shipping" and
   * "your order ships free" based on the live cart subtotal.
   */
  import { onMount } from 'svelte';
  import { cartStore } from '$lib/stores/cart.svelte';
  import * as m from '$lib/paraglide/messages';

  interface Setting { key: string; value: string }
  let { settings = [] }: { settings?: Setting[] } = $props();

  const STORAGE_KEY = 'gy.shippingNotice.dismissed';

  let dismissed = $state(false);
  let mounted = $state(false);

  onMount(() => {
    mounted = true;
    try {
      dismissed = localStorage.getItem(STORAGE_KEY) === '1';
    } catch {
      // localStorage unavailable (private mode etc.) — show the strip.
    }
  });

  function settingValue(key: string): string {
    return (settings.find((s) => s.key === key)?.value ?? '').trim();
  }

  const enabled = $derived(settingValue('free_shipping_threshold_enabled') === 'true');
  const threshold = $derived(() => {
    const n = Number(settingValue('free_shipping_threshold_hkd'));
    return Number.isFinite(n) && n > 0 ? n : 0;
  });
  const bgColor = $derived(settingValue('shipping_notice_bg_color') || '#1F4E3D');
  const textColor = $derived(settingValue('shipping_notice_text_color') || '#FFFFFF');
  const textSizePx = $derived(Number(settingValue('shipping_notice_text_size')) || 14);

  const eligible = $derived(threshold() > 0 && cartStore.subtotal >= threshold());
  const message = $derived(
    eligible
      ? m.shop_shipping_notice_eligible()
      : m.shop_shipping_notice_threshold({ amount: String(threshold()) })
  );

  function dismiss() {
    dismissed = true;
    try { localStorage.setItem(STORAGE_KEY, '1'); } catch { /* ignore */ }
  }
</script>

{#if enabled && threshold() > 0 && (!mounted || !dismissed)}
  <div class="border-b border-ink-300/60"
       style="background-color: {bgColor}; color: {textColor};"
       role="region" aria-label="Free shipping notice">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2 flex items-center justify-center gap-3 relative">
      <p class="font-display font-semibold uppercase tracking-normal sm:tracking-[0.15em] text-center"
         style="font-size: {textSizePx}px;">
        {message}
      </p>
      <button type="button" onclick={dismiss}
              aria-label="Dismiss shipping notice"
              class="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 p-1.5 opacity-60 hover:opacity-100 transition-opacity"
              style="color: {textColor};">
        <svg class="w-3.5 h-3.5" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>
{/if}
