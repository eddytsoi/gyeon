<script lang="ts">
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();
</script>

<svelte:head>
  <link rel="icon" href={data.faviconUrl || '/icon.svg'} />
  {#if data.faviconUrl}
    <link rel="apple-touch-icon" href={data.faviconUrl} />
  {/if}
</svelte:head>

{@render children()}
