<script lang="ts">
  import {
    resolveBleed,
    resolveBleedLg,
    resolveAspectRatio,
    resolveAspectRatioBreakpoint,
    resolveHeight,
    resolveAutoplay,
    resolveFitSize
  } from '$lib/shortcodes/video';
  import { detectStreamingVideoFromURL, buildEmbedURL, VIDEO_EXTS } from '$lib/media';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs }: { attrs: ShortcodeAttrs } = $props();

  function warnIfBad(key: string, raw: string | undefined, resolved: unknown) {
    if (import.meta.env.DEV && raw !== undefined && raw !== '' && String(resolved) !== raw) {
      // eslint-disable-next-line no-console
      console.warn(`[video] invalid ${key}="${raw}", falling back to "${resolved}"`);
    }
  }

  const source = $derived((attrs.source ?? '').trim());
  const streaming = $derived(source ? detectStreamingVideoFromURL(source) : null);
  const isFile = $derived(!streaming && VIDEO_EXTS.test(source));
  const isSupported = $derived(!!streaming || isFile);

  const bleed = $derived(resolveBleed(attrs.bleed));
  const bleedLg = $derived(resolveBleedLg(attrs['bleed-lg']));
  const aspectRatio = $derived(resolveAspectRatio(attrs['aspect-ratio']));
  const aspectRatioXs = $derived(resolveAspectRatioBreakpoint(attrs['aspect-ratio-xs']));
  const aspectRatioLg = $derived(resolveAspectRatioBreakpoint(attrs['aspect-ratio-lg']));
  const height = $derived(resolveHeight(attrs.height));
  const autoplay = $derived(resolveAutoplay(attrs.autoplay));
  const fitSize = $derived(resolveFitSize(attrs['fit-size']));

  const embedSrc = $derived(streaming ? buildEmbedURL(streaming.provider, streaming.videoID, autoplay) : null);

  // Only emit a CSS variable when the corresponding attribute has a concrete
  // value — an unset --video-ar-* lets the CSS fallback chain inherit. When
  // height is numeric, suppress all AR variables so the explicit height wins.
  const cssVars = $derived(
    [
      height === 'auto' ? '' : `--video-h:${height}px`,
      height === 'auto' && aspectRatio !== 'auto' ? `--video-ar:${aspectRatio}` : '',
      height === 'auto' && aspectRatioXs !== undefined && aspectRatioXs !== 'auto' ? `--video-ar-xs:${aspectRatioXs}` : '',
      height === 'auto' && aspectRatioLg !== undefined && aspectRatioLg !== 'auto' ? `--video-ar-lg:${aspectRatioLg}` : '',
      `--video-fit:${fitSize}`
    ]
      .filter(Boolean)
      .join(';')
  );

  // No shape set anywhere → element renders at intrinsic ratio.
  const natural = $derived(
    height === 'auto' &&
      aspectRatio === 'auto' &&
      (aspectRatioXs === undefined || aspectRatioXs === 'auto') &&
      (aspectRatioLg === undefined || aspectRatioLg === 'auto')
  );

  // Same viewport-edge escape as Banner.svelte. bleed-lg overrides at lg
  // (≥ 1024px); the lg:w-auto/lg:ml-0/lg:mr-0 reset neutralizes the
  // negative-margin escape when bleed="full" is paired with bleed-lg="container".
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

  $effect(() => {
    warnIfBad('bleed', attrs.bleed, bleed);
    warnIfBad('bleed-lg', attrs['bleed-lg'], bleedLg ?? '');
    warnIfBad('aspect-ratio', attrs['aspect-ratio'], aspectRatio);
    warnIfBad('aspect-ratio-xs', attrs['aspect-ratio-xs'], aspectRatioXs ?? '');
    warnIfBad('aspect-ratio-lg', attrs['aspect-ratio-lg'], aspectRatioLg ?? '');
    warnIfBad('height', attrs.height, height);
    warnIfBad('fit-size', attrs['fit-size'], fitSize);
    if (import.meta.env.DEV && source && !isSupported) {
      // eslint-disable-next-line no-console
      console.warn(`[video] unsupported source "${source}" — expected .mp4/.webm file or YouTube/Vimeo/Wistia URL`);
    }
  });
</script>

{#if source && isSupported}
  <div
    class="video my-6 {bleedClass} {attrs.class ?? ''}"
    style={cssVars}
    data-natural={natural ? 'true' : undefined}
    data-fit={fitSize}
  >
    {#if embedSrc}
      <iframe
        src={embedSrc}
        title="Embedded video"
        allow="autoplay; encrypted-media; picture-in-picture"
        allowfullscreen
      ></iframe>
    {:else if autoplay}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video src={source} preload="metadata" autoplay muted loop playsinline></video>
    {:else}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video src={source} preload="metadata" controls playsinline></video>
    {/if}
  </div>
{/if}

<style>
  .video {
    position: relative;
    overflow: hidden;
    display: block;
    height: var(--video-h, auto);
    aspect-ratio: var(--video-ar, auto);
  }
  .video :global(iframe),
  .video :global(video) {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: var(--video-fit, cover);
    border: 0;
  }
  .video[data-natural='true'] :global(video) {
    height: auto;
    object-fit: initial;
  }
  /* Streaming iframes adapt their inner rendering to the iframe's box —
   * object-fit on an iframe doesn't crop the embedded player. To get
   * `cover` semantics, oversize the iframe so its 16:9 viewport overflows
   * the container in the off-axis; the wrapper's overflow:hidden crops the
   * excess. Assumes embedded video is 16:9 (the universal default for
   * YouTube / Vimeo / Wistia). */
  .video[data-fit='cover'] :global(iframe) {
    --video-inner-ar: 1.7778;
    --video-effective-ar: var(--video-ar, var(--video-inner-ar));
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: max(100%, calc(var(--video-inner-ar) / var(--video-effective-ar) * 100%));
    height: max(100%, calc(var(--video-effective-ar) / var(--video-inner-ar) * 100%));
  }
  @media (max-width: 639.98px) {
    .video {
      aspect-ratio: var(--video-ar-xs, var(--video-ar, auto));
    }
    .video[data-fit='cover'] :global(iframe) {
      --video-effective-ar: var(--video-ar-xs, var(--video-ar, var(--video-inner-ar)));
    }
  }
  @media (min-width: 1024px) {
    .video {
      aspect-ratio: var(--video-ar-lg, var(--video-ar, auto));
    }
    .video[data-fit='cover'] :global(iframe) {
      --video-effective-ar: var(--video-ar-lg, var(--video-ar, var(--video-inner-ar)));
    }
  }
</style>
