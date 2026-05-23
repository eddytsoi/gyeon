<script lang="ts">
  import { buildResponsiveAttrs } from '$lib/image';
  import {
    resolveBleed,
    resolveBleedLg,
    resolveCol,
    resolveColBreakpoint,
    resolveGap,
    resolveGapBreakpoint,
    parseSourceList,
    resolveMediaSrc
  } from '$lib/shortcodes/photogrid';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs }: { attrs: ShortcodeAttrs } = $props();

  function warnIfBad(key: string, raw: string | undefined, resolved: unknown) {
    if (import.meta.env.DEV && raw !== undefined && raw !== '' && String(resolved) !== raw) {
      // eslint-disable-next-line no-console
      console.warn(`[photo-grid] invalid ${key}="${raw}", falling back to "${resolved}"`);
    }
  }

  const names = $derived(parseSourceList(attrs.source));
  const items = $derived(
    names.map((name) => ({ name, ...buildResponsiveAttrs(resolveMediaSrc(name)) }))
  );

  const bleed = $derived(resolveBleed(attrs.bleed));
  const bleedLg = $derived(resolveBleedLg(attrs['bleed-lg']));
  const col = $derived(resolveCol(attrs.col));
  const colXs = $derived(resolveColBreakpoint(attrs['col-xs']));
  const colLg = $derived(resolveColBreakpoint(attrs['col-lg']));
  const gap = $derived(resolveGap(attrs.gap));
  const gapXs = $derived(resolveGapBreakpoint(attrs['gap-xs']));
  const gapLg = $derived(resolveGapBreakpoint(attrs['gap-lg']));

  // Only emit a CSS variable when the corresponding attribute has a concrete
  // value — an unset --photogrid-*-xs / -lg lets the CSS fallback chain
  // inherit from the base.
  const cssVars = $derived(
    [
      `--photogrid-col:${col}`,
      colXs !== undefined ? `--photogrid-col-xs:${colXs}` : '',
      colLg !== undefined ? `--photogrid-col-lg:${colLg}` : '',
      `--photogrid-gap:${gap}`,
      gapXs !== undefined ? `--photogrid-gap-xs:${gapXs}` : '',
      gapLg !== undefined ? `--photogrid-gap-lg:${gapLg}` : ''
    ]
      .filter(Boolean)
      .join(';')
  );

  // Approximate per-image width for the `sizes` attribute. Ignores the gap —
  // browsers tolerate a small over-estimate, and srcset picks the next size
  // up. CSS variables can't be referenced in `sizes`, so this resolves the
  // effective col-per-breakpoint at render time.
  const sizes = $derived.by(() => {
    const xs = colXs ?? col;
    const lg = colLg ?? col;
    return `(max-width: 639.98px) ${Math.round(100 / xs)}vw, (min-width: 1024px) ${Math.round(100 / lg)}vw, ${Math.round(100 / col)}vw`;
  });

  // Same viewport-edge escape as Banner / Video. bleed-lg overrides at lg
  // (≥ 1024px); the lg:w-auto/lg:ml-0/lg:mr-0 reset neutralizes the negative-
  // margin escape when bleed="full" is paired with bleed-lg="container".
  const bleedClass = $derived(
    (() => {
      const base = bleed === 'full' ? 'w-screen ml-[calc(50%-50vw)] mr-[calc(50%-50vw)]' : '';
      if (bleedLg === undefined || bleedLg === bleed) return base;
      const lg =
        bleedLg === 'full'
          ? 'lg:w-screen lg:ml-[calc(50%-50vw)] lg:mr-[calc(50%-50vw)]'
          : 'lg:w-auto lg:ml-0 lg:mr-0';
      return base ? `${base} ${lg}` : lg;
    })()
  );

  $effect(() => {
    warnIfBad('bleed', attrs.bleed, bleed);
    warnIfBad('bleed-lg', attrs['bleed-lg'], bleedLg ?? '');
    warnIfBad('col', attrs.col, col);
    warnIfBad('col-xs', attrs['col-xs'], colXs ?? '');
    warnIfBad('col-lg', attrs['col-lg'], colLg ?? '');
    warnIfBad('gap', attrs.gap, gap);
    warnIfBad('gap-xs', attrs['gap-xs'], gapXs ?? '');
    warnIfBad('gap-lg', attrs['gap-lg'], gapLg ?? '');
  });
</script>

{#if items.length > 0}
  <div class="photo-grid my-6 {bleedClass} {attrs.class ?? ''}" style={cssVars}>
    {#each items as item, i (i)}
      <div class="photo-grid__item {attrs['class-item'] ?? ''}">
        <img
          src={item.src}
          srcset={item.srcset}
          {sizes}
          alt=""
          loading="lazy"
          decoding="async"
        />
      </div>
    {/each}
  </div>
{/if}

<style>
  .photo-grid {
    display: grid;
    grid-template-columns: repeat(var(--photogrid-col, 2), minmax(0, 1fr));
    gap: var(--photogrid-gap, 8px);
  }
  .photo-grid__item {
    display: block;
    width: 100%;
  }
  .photo-grid__item :global(img) {
    display: block;
    width: 100%;
    height: auto;
  }
  @media (max-width: 639.98px) {
    .photo-grid {
      grid-template-columns: repeat(
        var(--photogrid-col-xs, var(--photogrid-col, 2)),
        minmax(0, 1fr)
      );
      gap: var(--photogrid-gap-xs, var(--photogrid-gap, 8px));
    }
  }
  @media (min-width: 1024px) {
    .photo-grid {
      grid-template-columns: repeat(
        var(--photogrid-col-lg, var(--photogrid-col, 2)),
        minmax(0, 1fr)
      );
      gap: var(--photogrid-gap-lg, var(--photogrid-gap, 8px));
    }
  }
</style>
