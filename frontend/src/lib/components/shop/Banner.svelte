<script lang="ts">
  import { buildResponsiveAttrs } from '$lib/image';
  import type { BannerBleed, BannerAspect, BannerHeight, BannerFit } from '$lib/shortcodes/banner';

  let {
    image,
    imageMobile,
    alt = '',
    href,
    bleed = 'full',
    bleedSm = undefined,
    bleedLg = undefined,
    aspectRatio = 'auto',
    aspectRatioMobile = 'auto',
    height = 'auto',
    fitSize = 'cover',
    class: klass = ''
  }: {
    image: string;
    imageMobile?: string;
    alt?: string;
    href?: string;
    bleed?: BannerBleed;
    bleedSm?: BannerBleed;
    bleedLg?: BannerBleed;
    aspectRatio?: BannerAspect;
    aspectRatioMobile?: BannerAspect;
    height?: BannerHeight;
    fitSize?: BannerFit;
    class?: string;
  } = $props();

  const desktop = $derived(buildResponsiveAttrs(image));
  const mobile = $derived(imageMobile ? buildResponsiveAttrs(imageMobile) : null);

  // Only emit a CSS variable when the corresponding attribute has a concrete
  // value — an unset --banner-ar-* lets the CSS fallback chain inherit from
  // the other breakpoint. When height is numeric, suppress both AR variables
  // so the explicit height wins (otherwise CSS would widen the element to
  // satisfy the aspect-ratio + height pair).
  const cssVars = $derived(
    [
      height === 'auto' ? '' : `--banner-h:${height}px`,
      height === 'auto' && aspectRatio !== 'auto' ? `--banner-ar-desktop:${aspectRatio}` : '',
      height === 'auto' && aspectRatioMobile !== 'auto' ? `--banner-ar-mobile:${aspectRatioMobile}` : '',
      `--banner-fit:${fitSize}`
    ]
      .filter(Boolean)
      .join(';')
  );

  // No shape set anywhere → image renders at its intrinsic ratio.
  const natural = $derived(
    height === 'auto' && aspectRatio === 'auto' && aspectRatioMobile === 'auto'
  );

  // Same viewport-edge escape Section.svelte uses for bleed="full".
  // Three responsive tiers: base (mobile) → `bleed-sm` at the Tailwind `sm`
  // breakpoint (≥ 640px) → `bleed-lg` at `lg` (≥ 1024px). Each override is
  // optional; a tier only emits its prefixed utilities when its effective
  // value differs from the tier below it, so we never emit redundant classes.
  // The w-auto/ml-0/mr-0 reset neutralizes the negative-margin escape when a
  // wider tier switches "full" back to "container". `sm:` source-orders before
  // `lg:` in Tailwind, so the wider breakpoint correctly wins.
  const bleedClass = $derived(
    (() => {
      // Full literal class strings (not built by prefix concat) so Tailwind's
      // JIT scanner picks every variant up. base "container" emits nothing —
      // the element stays in normal flow.
      const lit = {
        base: {
          full: 'w-screen ml-[calc(50%-50vw)] mr-[calc(50%-50vw)]',
          container: ''
        },
        sm: {
          full: 'sm:w-screen sm:ml-[calc(50%-50vw)] sm:mr-[calc(50%-50vw)]',
          container: 'sm:w-auto sm:ml-0 sm:mr-0'
        },
        lg: {
          full: 'lg:w-screen lg:ml-[calc(50%-50vw)] lg:mr-[calc(50%-50vw)]',
          container: 'lg:w-auto lg:ml-0 lg:mr-0'
        }
      } as const;
      let out: string = lit.base[bleed];
      let eff: BannerBleed = bleed;
      if (bleedSm !== undefined && bleedSm !== eff) {
        out = out ? `${out} ${lit.sm[bleedSm]}` : lit.sm[bleedSm];
        eff = bleedSm;
      }
      if (bleedLg !== undefined && bleedLg !== eff) {
        out = out ? `${out} ${lit.lg[bleedLg]}` : lit.lg[bleedLg];
        eff = bleedLg;
      }
      return out;
    })()
  );

  const isExternal = $derived(href ? /^https?:\/\//i.test(href) : false);
</script>

{#snippet inner()}
  <picture>
    {#if mobile}
      <source media="(max-width: 767.98px)" srcset={mobile.srcset} sizes="100vw" />
    {/if}
    <img
      src={desktop.src}
      srcset={desktop.srcset}
      sizes="100vw"
      {alt}
      loading="lazy"
      decoding="async"
    />
  </picture>
{/snippet}

{#if href}
  <a
    {href}
    target={isExternal ? '_blank' : undefined}
    rel={isExternal ? 'noopener noreferrer' : undefined}
    class="banner my-6 {bleedClass} {klass}"
    style={cssVars}
    data-natural={natural ? 'true' : undefined}
  >
    {@render inner()}
  </a>
{:else}
  <div
    class="banner my-6 {bleedClass} {klass}"
    style={cssVars}
    data-natural={natural ? 'true' : undefined}
  >
    {@render inner()}
  </div>
{/if}

<style>
  .banner {
    position: relative;
    overflow: hidden;
    display: block;
    height: var(--banner-h, auto);
    aspect-ratio: var(--banner-ar-mobile, var(--banner-ar-desktop, auto));
  }
  .banner :global(picture) {
    display: block;
    width: 100%;
    height: 100%;
  }
  .banner :global(picture > img) {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: var(--banner-fit, cover);
  }
  .banner[data-natural='true'] :global(picture > img) {
    height: auto;
    object-fit: initial;
  }
  @media (min-width: 768px) {
    .banner {
      aspect-ratio: var(--banner-ar-desktop, var(--banner-ar-mobile, auto));
    }
  }
</style>
