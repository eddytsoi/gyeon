<script lang="ts">
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import { resolveProductRef } from '$lib/shortcodes/types';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';

  let { attrs, refs }: { attrs: ShortcodeAttrs; refs: ShortcodeRefs } = $props();

  const DEFAULT_LIMIT = 12;

  function parseLimit(val: string | undefined, fallback: number): number {
    return val && /^\d+$/.test(val) ? Math.max(1, Number(val)) : fallback;
  }

  // Order: explicit ids first (in author-declared order), then category
  // expansions (slug by slug). Dedup by UUID so a product listed in both
  // sources only renders once.
  const uuids = $derived.by(() => {
    const seen = new Set<string>();
    const out: string[] = [];

    for (const token of (attrs.ids ?? '').split(',')) {
      const trimmed = token.trim();
      if (!trimmed) continue;
      const uuid = resolveProductRef(trimmed, refs);
      if (uuid && !seen.has(uuid)) {
        seen.add(uuid);
        out.push(uuid);
      }
    }

    for (const slug of (attrs.categories ?? '').split(',')) {
      const trimmed = slug.trim();
      if (!trimmed) continue;
      const ids = refs.productsByCategory[trimmed] ?? [];
      for (const id of ids) {
        if (!seen.has(id)) {
          seen.add(id);
          out.push(id);
        }
      }
    }

    return out;
  });

  // Mobile-first effective limits; each breakpoint falls back to the one below.
  const limitMobile = $derived(parseLimit(attrs.limit, DEFAULT_LIMIT));
  const limitTablet = $derived(parseLimit(attrs['limit-md'], limitMobile));
  const limitDesktop = $derived(parseLimit(attrs['limit-lg'], limitTablet));
  const maxItems = $derived(Math.max(limitMobile, limitTablet, limitDesktop));

  const items = $derived(
    uuids
      .slice(0, maxItems)
      .map((id) => refs.products[id])
      .filter((r) => r != null)
  );

  function visibilityClass(i: number): string {
    const showMobile = i < limitMobile;
    const showTablet = i < limitTablet;
    const showDesktop = i < limitDesktop;
    const classes: string[] = [];
    if (!showMobile) classes.push('hidden');
    if (showTablet !== showMobile) classes.push(showTablet ? 'md:block' : 'md:hidden');
    if (showDesktop !== showTablet) classes.push(showDesktop ? 'lg:block' : 'lg:hidden');
    return classes.join(' ');
  }
</script>

{#if items.length > 0}
  <div class="my-6 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-6 {attrs.class ?? ''}">
    {#each items as ref, i (ref.product.id)}
      <div class={visibilityClass(i)}>
        <ProductCard product={ref.product} image={ref.image ?? undefined} variant={ref.variant ?? undefined} />
      </div>
    {/each}
  </div>
{/if}
