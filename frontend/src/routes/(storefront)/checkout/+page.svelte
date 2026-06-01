<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import CheckoutClassic from './CheckoutClassic.svelte';
  import CheckoutModern from './CheckoutModern.svelte';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.checkout_title()}</title>
</svelte:head>

{#if data.checkoutLayout === 'modern'}
  <CheckoutModern {data} />
{:else}
  <CheckoutClassic {data} />
{/if}
