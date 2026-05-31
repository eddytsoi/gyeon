<script lang="ts">
  /*
   * P3 #29 — free-shipping progress banner. Reads threshold from
   * `free_shipping_threshold_hkd` (passed from layout `publicSettings`).
   * Hidden when threshold ≤ 0 (feature off).
   */
  import { cartStore } from '$lib/stores/cart.svelte';
  import { formatHKD } from '$lib/money';
  import * as m from '$lib/paraglide/messages';
  import { resolveFreeShippingThreshold, type SiteSetting } from '$lib/shippingThreshold';

  interface Props {
    settings: SiteSetting[];
    /** Customer role — picks the installer threshold when 'installer'. */
    role?: string | null;
    /** When true (cart page), show even after the threshold is met as a "you've unlocked" banner. */
    showUnlocked?: boolean;
  }

  let { settings, role = null, showUnlocked = false }: Props = $props();

  const resolved = $derived(resolveFreeShippingThreshold(settings, role));
  const enabled = $derived(resolved.enabled);
  const threshold = $derived(() => resolved.threshold);

  const remaining = $derived(Math.max(0, threshold() - cartStore.subtotal));
  const progress = $derived(
    threshold() === 0 ? 0 : Math.min(100, (cartStore.subtotal / threshold()) * 100)
  );
  const unlocked = $derived(threshold() > 0 && cartStore.subtotal >= threshold());
</script>

{#if enabled && threshold() > 0 && (unlocked ? showUnlocked : cartStore.subtotal > 0)}
  <div class="rounded-xl border border-gray-100 bg-white px-4 py-3 text-sm" role="status" aria-live="polite">
    {#if unlocked}
      <p class="text-emerald-700 font-medium">
        🎉 {m.free_shipping_unlocked()}
      </p>
    {:else}
      <p class="text-gray-700 mb-2">
        {m.free_shipping_remaining({ amount: formatHKD(remaining) })}
      </p>
      <div class="h-1.5 bg-gray-100 rounded-full overflow-hidden">
        <div class="h-full bg-gray-900 rounded-full transition-[width] duration-300"
             style="width: {progress}%;"></div>
      </div>
    {/if}
  </div>
{/if}
