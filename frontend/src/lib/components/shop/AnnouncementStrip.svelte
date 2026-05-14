<script lang="ts">
  /*
   * Slim site-wide announcement bar — pinned above the header.
   * Reads `site_notice` from publicSettings; renders nothing when empty
   * or the user has dismissed it (localStorage flag).
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

  function settingValue(key: string): string {
    return (settings.find((s) => s.key === key)?.value ?? '').trim();
  }

  const notice = $derived(settingValue('site_notice'));
  const enabled = $derived(settingValue('site_notice_enabled') !== 'false');
  const bgColor = $derived(settingValue('site_notice_bg_color') || '#EDE9E1');
  const textColor = $derived(settingValue('site_notice_text_color') || '#1A1A1A');
  const textSizePx = $derived(Number(settingValue('site_notice_text_size')) || 16);

  function dismiss() {
    dismissed = true;
    try { localStorage.setItem(STORAGE_KEY, '1'); } catch { /* ignore */ }
  }
</script>

{#if enabled && notice && (!mounted || !dismissed)}
  <div class="border-b border-ink-300/60"
       style="background-color: {bgColor}; color: {textColor};"
       role="region" aria-label="Site announcement">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2 flex items-center justify-center gap-3 relative">
      <p class="font-display font-semibold uppercase tracking-[0.15em] text-center"
         style="font-size: {textSizePx}px;">
        {notice}
      </p>
      <button type="button" onclick={dismiss}
              aria-label="Dismiss announcement"
              class="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 p-1.5 opacity-60 hover:opacity-100 transition-opacity"
              style="color: {textColor};">
        <svg class="w-3.5 h-3.5" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>
{/if}
