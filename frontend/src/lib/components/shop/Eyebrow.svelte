<script lang="ts">
  /*
   * Editorial eyebrow — the 11px uppercase tracking-0.18em primitive used
   * across breadcrumbs, section captions, stat-panel labels, and above page H1s.
   * Defaults to navy; pass `tone="muted"` for ink-500 or `tone="invert"` on dark surfaces.
   */
  type Tone = 'navy' | 'muted' | 'invert';
  let {
    tone = 'navy',
    as = 'p',
    class: extra = '',
    children
  }: {
    tone?: Tone;
    as?: 'p' | 'span' | 'div';
    class?: string;
    children: any;
  } = $props();

  const toneClass = $derived(
    tone === 'invert' ? 'text-white/60'
    : tone === 'muted' ? 'text-ink-500'
    : 'text-navy-500'
  );
</script>

<svelte:element this={as}
  class="text-[11px] uppercase tracking-[0.18em] font-display font-semibold {toneClass} {extra}">
  {@render children()}
</svelte:element>
