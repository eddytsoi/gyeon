<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import { siteName } from '$lib/seo';
  import CartClassic from './CartClassic.svelte';
  import CartModern from './CartModern.svelte';
  import type { PageData } from './$types';

  // publicSettings flows in from the (storefront) layout load.
  let { data }: { data: PageData } = $props();

  const cartLayout = $derived(
    data.publicSettings?.find((s) => s.key === 'cart_page_layout')?.value || 'classic'
  );
</script>

<svelte:head>
  <title>{m.cart_title({ brand: siteName(data.publicSettings) })}</title>
</svelte:head>

{#if cartLayout === 'modern'}
  <CartModern {data} />
{:else}
  <CartClassic {data} />
{/if}
