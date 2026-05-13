<script lang="ts">
  import { buildResponsiveAttrs, DEFAULT_WIDTHS } from '$lib/image';

  // Thin wrapper around <img> that swaps a /uploads/foo.jpg src for a srcset
  // pointing at the backend's on-demand resize endpoint. Non-/uploads/ srcs
  // pass through unchanged, so this component is safe to drop in anywhere.

  let {
    src,
    alt = '',
    sizes = '100vw',
    widths = DEFAULT_WIDTHS,
    loading = 'lazy',
    fetchpriority = 'auto',
    decoding = 'async',
    class: className = '',
    onload,
    onerror
  }: {
    src: string;
    alt?: string;
    sizes?: string;
    widths?: number[];
    loading?: 'lazy' | 'eager';
    fetchpriority?: 'high' | 'low' | 'auto';
    decoding?: 'async' | 'sync' | 'auto';
    class?: string;
    onload?: ((e: Event) => void) | null;
    onerror?: ((e: Event) => void) | null;
  } = $props();

  const attrs = $derived(buildResponsiveAttrs(src, widths));
</script>

<img
  src={attrs.src}
  srcset={attrs.srcset || undefined}
  {sizes}
  {alt}
  {loading}
  {fetchpriority}
  {decoding}
  class={className}
  {onload}
  {onerror}
/>
