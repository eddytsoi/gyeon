<script lang="ts">
  import Button from '$lib/components/shop/Button.svelte';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs }: { attrs: ShortcodeAttrs } = $props();

  const style = $derived(attrs.style === 'secondary' ? 'secondary' : 'primary');
</script>

{#if attrs.href && attrs.label}
  <div class="my-6">
    <Button href={attrs.href} label={attrs.label} {style} />
  </div>
{/if}
