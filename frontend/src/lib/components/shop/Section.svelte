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
    type SectionAlign
  } from '$lib/shortcodes/section';

  let {
    bg = 'paper',
    layout = 'default',
    padding = 'md',
    width = 'default',
    align = 'left',
    id,
    children
  }: {
    bg?: SectionBg;
    layout?: SectionLayout;
    padding?: SectionPadding;
    width?: SectionWidth;
    align?: SectionAlign;
    id?: string;
    children?: Snippet;
  } = $props();

  const bgClass = $derived(SECTION_BG[bg]);
  const padClass = $derived(SECTION_PADDING[padding]);
  const widthClass = $derived(SECTION_WIDTH[width]);
  const alignClass = $derived(align === 'center' ? 'text-center' : '');

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

<section {id} class={bgClass}>
  {#if width === 'full'}
    {#if layout === 'default'}
      <div class={alignClass}>{@render children?.()}</div>
    {:else}
      <div class="{gridClass} {alignClass}">{@render children?.()}</div>
    {/if}
  {:else if layout === 'default'}
    <div class="{widthClass} {padClass} {alignClass}">{@render children?.()}</div>
  {:else}
    <div class="{widthClass} {padClass} {gridClass} {alignClass}">{@render children?.()}</div>
  {/if}
</section>
