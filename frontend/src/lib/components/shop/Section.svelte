<script lang="ts">
  import type { Snippet } from 'svelte';
  import {
    SECTION_BG,
    SECTION_PADDING,
    SECTION_WIDTH,
    type SectionBg,
    type SectionLayout,
    type SectionPadding,
    type SectionWidth,
    type SectionAlign,
    type SectionBleed
  } from '$lib/shortcodes/section';

  let {
    bg = 'paper',
    layout = 'default',
    padding = 'md',
    width = 'default',
    align = 'left',
    bleed = 'full',
    bleedLg = undefined,
    id,
    class: klass = '',
    children
  }: {
    bg?: SectionBg;
    layout?: SectionLayout;
    padding?: SectionPadding;
    width?: SectionWidth;
    align?: SectionAlign;
    bleed?: SectionBleed;
    bleedLg?: SectionBleed;
    id?: string;
    class?: string;
    children?: Snippet;
  } = $props();

  const bgClass = $derived(SECTION_BG[bg]);
  const padClass = $derived(SECTION_PADDING[padding]);
  const widthClass = $derived(SECTION_WIDTH[width]);
  const alignClass = $derived(align === 'center' ? 'text-center' : '');
  const containerBleed = $derived(bleed === 'container' && width !== 'full');
  // Same predicate at the lg breakpoint; falls back to the base when bleed-lg
  // is unset so default behavior is unchanged.
  const containerBleedLg = $derived(
    bleedLg === undefined ? containerBleed : bleedLg === 'container' && width !== 'full'
  );
  // Escape any constraining ancestor so bleed="full" really reaches the viewport
  // edge (CMS pages wrap shortcodes in max-w-7xl). No-op on top-level usage.
  // bleed-lg appends a responsive override at ≥1024px when it differs from
  // base bleed: container→full adds the escape, full→container neutralizes it.
  // Note: bg placement still follows base bleed (containerBleed → bg on inner),
  // so a container→full lg flip only widens the outer's width/margin; if you
  // need bg to also extend to viewport edge on lg, set bleed="full" as base.
  const fullBleedClass = $derived.by(() => {
    const base = containerBleed ? '' : 'w-screen ml-[calc(50%-50vw)] mr-[calc(50%-50vw)]';
    if (containerBleedLg === containerBleed) return base;
    const lg = containerBleedLg
      ? 'lg:w-auto lg:ml-0 lg:mr-0'
      : 'lg:w-screen lg:ml-[calc(50%-50vw)] lg:mr-[calc(50%-50vw)]';
    return base ? `${base} ${lg}` : lg;
  });

  // Grid classes per layout. Children supply their own col-span / order
  // classes; the shortcode wrapper handles that automatically, the Svelte
  // call-site can do it inline.
  const gridClass = $derived(
    layout === 'split' || layout === 'split-reverse'
      ? 'grid md:grid-cols-2 gap-8 md:gap-12 lg:gap-16 items-center'
      : layout === 'hero'
        ? 'grid md:grid-cols-12 gap-8 md:gap-10 lg:gap-16 items-center'
        : ''
  );
</script>

<section {id} class="{containerBleed ? '' : bgClass} {fullBleedClass} {klass}">
  {#if width === 'full'}
    {#if layout === 'default'}
      <div class={alignClass}>{@render children?.()}</div>
    {:else}
      <div class="{gridClass} {alignClass}">{@render children?.()}</div>
    {/if}
  {:else if layout === 'default'}
    <div class="{containerBleed ? bgClass : ''} {widthClass} {padClass} {alignClass}">{@render children?.()}</div>
  {:else}
    <div class="{containerBleed ? bgClass : ''} {widthClass} {padClass} {gridClass} {alignClass}">{@render children?.()}</div>
  {/if}
</section>
