<script lang="ts">
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';

  let { attrs, refs }: { attrs: ShortcodeAttrs; refs: ShortcodeRefs } = $props();

  const ref = $derived(attrs.id ? refs.products[attrs.id] : undefined);
</script>

{#if ref}
  <div class="my-6 max-w-xs">
    <ProductCard product={ref.product} image={ref.image ?? undefined} variant={ref.variant ?? undefined} />
  </div>
{/if}
