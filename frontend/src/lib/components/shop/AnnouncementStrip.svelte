<script lang="ts">
  /*
   * Slim site-wide announcement bar — pinned above the header.
   * Reads `free_shipping_threshold_hkd` from publicSettings; renders nothing
   * when threshold ≤ 0 or the user has dismissed it (localStorage flag).
   * Design system: gyeon-project-design-system §5.1.
   */
  import { onMount } from 'svelte';

  interface Setting { key: string; value: string }
  let { settings = [] }: { settings?: Setting[] } = $props();

  const STORAGE_KEY = 'gy.announcement.dismissed';

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

  const threshold = $derived(() => {
    const raw = settings.find((s) => s.key === 'free_shipping_threshold_hkd')?.value;
    const n = raw ? Number(raw) : 0;
    return Number.isFinite(n) && n > 0 ? n : 0;
  });

  function dismiss() {
    dismissed = true;
    try { localStorage.setItem(STORAGE_KEY, '1'); } catch { /* ignore */ }
  }
</script>

{#if threshold() > 0 && (!mounted || !dismissed)}
  <div class="bg-cream text-ink-900 border-b border-ink-300/60" role="region" aria-label="Site announcement">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2 flex items-center justify-center gap-3 relative">
      <p class="text-[11px] sm:text-xs font-display font-semibold uppercase tracking-[0.15em] text-center">
        訂單滿 HK${threshold()} 免運費 · 即日訂購次日送達
      </p>
      <button type="button" onclick={dismiss}
              aria-label="Dismiss announcement"
              class="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 p-1.5 text-ink-500 hover:text-ink-900 transition-colors">
        <svg class="w-3.5 h-3.5" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>
{/if}
