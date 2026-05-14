<script lang="ts">
  /*
   * P3 #29 — free-shipping progress banner. Reads threshold from
   * `free_shipping_threshold_hkd` (passed from layout `publicSettings`).
   * Hidden when threshold ≤ 0 (feature off).
   */
  import { cartStore } from '$lib/stores/cart.svelte';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    settings: Array<{ key: string; value: string }>;
    /** When true (cart page), show even after the threshold is met as a "you've unlocked" banner. */
    showUnlocked?: boolean;
  }

  let { settings, showUnlocked = false }: Props = $props();

  const threshold = $derived(() => {
    const raw = settings.find((s) => s.key === 'free_shipping_threshold_hkd')?.value;
    const n = raw ? Number(raw) : 0;
    return Number.isFinite(n) && n > 0 ? n : 0;
  });

  const remaining = $derived(Math.max(0, threshold() - cartStore.subtotal));
  const progress = $derived(
    threshold() === 0 ? 0 : Math.min(100, (cartStore.subtotal / threshold()) * 100)
  );
  const unlocked = $derived(threshold() > 0 && cartStore.subtotal >= threshold());
</script>

{#if threshold() > 0 && (unlocked ? showUnlocked : cartStore.subtotal > 0)}
  <div class="rounded-xl border border-gray-100 bg-white px-4 py-3 text-sm" role="status" aria-live="polite">
    {#if unlocked}
      <p class="text-emerald-700 font-medium">
        🎉 {m.free_shipping_unlocked()}
      </p>
    {:else}
      <p class="text-gray-700 mb-2">
        {m.free_shipping_remaining({ amount: `HK$${remaining.toFixed(0)}` })}
      </p>
      <div class="h-1.5 bg-gray-100 rounded-full overflow-hidden">
        <div class="h-full bg-gray-900 rounded-full transition-[width] duration-300"
             style="width: {progress}%;"></div>
      </div>
    {/if}
  </div>
{/if}
