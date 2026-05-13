<script lang="ts">
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';

  let { attrs, refs }: { attrs: ShortcodeAttrs; refs: ShortcodeRefs } = $props();

  const ids = $derived(
    (attrs.ids ?? '')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
  );

  const items = $derived(ids.map((id) => refs.products[id]).filter((r) => r != null));
</script>

{#if items.length > 0}
  <div class="my-6 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-6">
    {#each items as ref (ref.product.id)}
      <ProductCard product={ref.product} image={ref.image ?? undefined} variant={ref.variant ?? undefined} />
    {/each}
  </div>
{/if}
