<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import { siteName } from '$lib/seo';
  import CheckoutClassic from './CheckoutClassic.svelte';
  import CheckoutModern from './CheckoutModern.svelte';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.checkout_title({ brand: siteName(data.publicSettings) })}</title>
</svelte:head>

{#if data.checkoutLayout === 'modern'}
  <CheckoutModern {data} />
{:else}
  <CheckoutClassic {data} />
{/if}
