<script lang="ts">
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import { resolveProductRef } from '$lib/shortcodes/types';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';

  let { attrs, refs }: { attrs: ShortcodeAttrs; refs: ShortcodeRefs } = $props();

  const DEFAULT_LIMIT = 12;

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

  const limit = $derived(
    attrs.limit && /^\d+$/.test(attrs.limit) ? Math.max(1, Number(attrs.limit)) : DEFAULT_LIMIT
  );

  const items = $derived(
    uuids
      .slice(0, limit)
      .map((id) => refs.products[id])
      .filter((r) => r != null)
  );
</script>

{#if items.length > 0}
  <div class="my-6 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-6 {attrs.class ?? ''}">
    {#each items as ref (ref.product.id)}
      <ProductCard product={ref.product} image={ref.image ?? undefined} variant={ref.variant ?? undefined} />
    {/each}
  </div>
{/if}
