<script lang="ts">
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import Section from '$lib/components/shop/Section.svelte';
  import {
    resolveBg,
    resolveLayout,
    resolvePadding,
    resolveWidth,
    resolveAlign,
    splitBodyOnHr
  } from '$lib/shortcodes/section';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';
  import { EMPTY_REFS } from '$lib/shortcodes/types';

  let {
    attrs,
    body,
    refs = EMPTY_REFS
  }: {
    attrs: ShortcodeAttrs;
    body: string;
    refs?: ShortcodeRefs;
  } = $props();

  // Warn (dev only) when an attr falls outside its whitelist so authors can
  // see typos without breaking the page. import.meta.env.DEV is replaced at
  // build time and stripped from prod.
  function warnIfBad(key: string, raw: string | undefined, resolved: string) {
    if (import.meta.env.DEV && raw && raw !== resolved) {
      // eslint-disable-next-line no-console
      console.warn(`[section] unknown ${key}="${raw}", falling back to "${resolved}"`);
    }
  }

  const bg = $derived(resolveBg(attrs.bg));
  const layout = $derived(resolveLayout(attrs.layout));
  const padding = $derived(resolvePadding(attrs.padding));
  const width = $derived(resolveWidth(attrs.width));
  const align = $derived(resolveAlign(attrs.align));
  const id = $derived(attrs.id || undefined);

  $effect(() => {
    warnIfBad('bg', attrs.bg, bg);
    warnIfBad('layout', attrs.layout, layout);
    warnIfBad('padding', attrs.padding, padding);
    warnIfBad('width', attrs.width, width);
    warnIfBad('align', attrs.align, align);
  });

  const split = $derived(
    layout === 'default' ? null : splitBodyOnHr(body)
  );
</script>

<Section {bg} {layout} {padding} {width} {align} {id}>
  {#if layout === 'default' || !split}
    <MarkdownContent content={body} {refs} />
  {:else if layout === 'split'}
    <div><MarkdownContent content={split[0]} {refs} /></div>
    <div><MarkdownContent content={split[1]} {refs} /></div>
  {:else if layout === 'split-reverse'}
    <div class="order-2 md:order-1"><MarkdownContent content={split[1]} {refs} /></div>
    <div class="order-1 md:order-2"><MarkdownContent content={split[0]} {refs} /></div>
  {:else if layout === 'hero'}
    <div class="md:col-span-7 order-2 md:order-1">
      <MarkdownContent content={split[0]} {refs} />
    </div>
    <div class="md:col-span-5 order-1 md:order-2">
      <MarkdownContent content={split[1]} {refs} />
    </div>
  {/if}
</Section>
