<script lang="ts">
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import { resolveProductRef } from '$lib/shortcodes/types';
  import type { ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';

  let { attrs, refs }: { attrs: ShortcodeAttrs; refs: ShortcodeRefs } = $props();

  const uuid = $derived(attrs.id ? resolveProductRef(attrs.id, refs) : null);
  const ref = $derived(uuid ? refs.products[uuid] : undefined);
</script>

{#if ref}
  <div class="my-6 max-w-xs">
    <ProductCard product={ref.product} image={ref.image ?? undefined} variant={ref.variant ?? undefined} />
  </div>
{/if}
