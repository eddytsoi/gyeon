<script lang="ts">
  import { buildResponsiveAttrs } from '$lib/image';
  import type { BannerBleed, BannerAspect, BannerHeight } from '$lib/shortcodes/banner';

  let {
    image,
    imageMobile,
    alt = '',
    href,
    bleed = 'full',
    bleedLg = undefined,
    aspectRatio = 'auto',
    aspectRatioMobile = 'auto',
    height = 'auto'
  }: {
    image: string;
    imageMobile?: string;
    alt?: string;
    href?: string;
    bleed?: BannerBleed;
    bleedLg?: BannerBleed;
    aspectRatio?: BannerAspect;
    aspectRatioMobile?: BannerAspect;
    height?: BannerHeight;
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
      height === 'auto' && aspectRatioMobile !== 'auto' ? `--banner-ar-mobile:${aspectRatioMobile}` : ''
    ]
      .filter(Boolean)
      .join(';')
  );

  // No shape set anywhere → image renders at its intrinsic ratio.
  const natural = $derived(
    height === 'auto' && aspectRatio === 'auto' && aspectRatioMobile === 'auto'
  );

  // Same viewport-edge escape Section.svelte uses for bleed="full".
  // bleed-lg overrides at the Tailwind `lg` breakpoint (≥ 1024px); the
  // lg:w-auto/lg:ml-0/lg:mr-0 reset neutralizes the negative-margin escape
  // when bleed="full" is paired with bleed-lg="container".
  const bleedClass = $derived(
    (() => {
      const base =
        bleed === 'full' ? 'w-screen ml-[calc(50%-50vw)] mr-[calc(50%-50vw)]' : '';
      if (bleedLg === undefined || bleedLg === bleed) return base;
      const lg =
        bleedLg === 'full'
          ? 'lg:w-screen lg:ml-[calc(50%-50vw)] lg:mr-[calc(50%-50vw)]'
          : 'lg:w-auto lg:ml-0 lg:mr-0';
      return base ? `${base} ${lg}` : lg;
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
    class="banner my-6 {bleedClass}"
    style={cssVars}
    data-natural={natural ? 'true' : undefined}
  >
    {@render inner()}
  </a>
{:else}
  <div
    class="banner my-6 {bleedClass}"
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
    object-fit: cover;
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
