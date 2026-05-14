<script lang="ts">
  import Banner from '$lib/components/shop/Banner.svelte';
  import {
    resolveBleed,
    resolveBleedLg,
    resolveAspectRatio,
    resolveHeight
  } from '$lib/shortcodes/banner';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs }: { attrs: ShortcodeAttrs } = $props();

  function warnIfBad(key: string, raw: string | undefined, resolved: unknown) {
    if (import.meta.env.DEV && raw !== undefined && raw !== '' && String(resolved) !== raw) {
      // eslint-disable-next-line no-console
      console.warn(`[banner] invalid ${key}="${raw}", falling back to "${resolved}"`);
    }
  }

  const bleed = $derived(resolveBleed(attrs.bleed));
  const bleedLg = $derived(resolveBleedLg(attrs['bleed-lg']));
  const aspectRatio = $derived(resolveAspectRatio(attrs['aspect-ratio']));
  const aspectRatioMobile = $derived(resolveAspectRatio(attrs['aspect-ratio-mobile']));
  const height = $derived(resolveHeight(attrs.height));

  $effect(() => {
    warnIfBad('bleed', attrs.bleed, bleed);
    warnIfBad('bleed-lg', attrs['bleed-lg'], bleedLg ?? '');
    warnIfBad('aspect-ratio', attrs['aspect-ratio'], aspectRatio);
    warnIfBad('aspect-ratio-mobile', attrs['aspect-ratio-mobile'], aspectRatioMobile);
    warnIfBad('height', attrs.height, height);
    if (import.meta.env.DEV && attrs.image && (attrs.alt === undefined || attrs.alt === '')) {
      // eslint-disable-next-line no-console
      console.warn('[banner] missing alt="" — add alt text for accessibility or set alt="" explicitly if decorative');
    }
  });
</script>

{#if attrs.image}
  <Banner
    image={attrs.image}
    imageMobile={attrs['image-mobile']}
    alt={attrs.alt ?? ''}
    href={attrs.href}
    {bleed}
    {bleedLg}
    {aspectRatio}
    {aspectRatioMobile}
    {height}
  />
{/if}
