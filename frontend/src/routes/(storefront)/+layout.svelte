<script lang="ts">
  import '../../app.css';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { wishlistStore } from '$lib/stores/wishlist.svelte';
  import { registerStorefrontTools } from '$lib/webmcp';
  import { initTracker } from '$lib/tracker';
  import { onMount } from 'svelte';
  import type { LayoutData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const faviconUrl = $derived(
    data.publicSettings?.find((s) => s.key === 'favicon_url')?.value ?? ''
  );

  onMount(async () => {
    initTracker(data.publicSettings ?? []);
    await cartStore.init();
    await wishlistStore.init(!!data.customer);
    await registerStorefrontTools(data.mcpEnabled);
  });
</script>

<svelte:head>
  {#if faviconUrl}
    <link rel="icon" href={faviconUrl} />
    <link rel="apple-touch-icon" href={faviconUrl} />
  {/if}
</svelte:head>

<!--
  Skip-to-content (P3 #32). Hidden until keyboard focus, then anchors at the
  top-left so screen-reader / keyboard users can jump past Header nav.
-->
<a href="#main-content"
   class="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[100]
          focus:px-4 focus:py-2 focus:rounded-lg focus:bg-gray-900 focus:text-white
          focus:text-sm focus:font-medium focus:shadow-lg focus:outline-none
          focus:ring-2 focus:ring-offset-2 focus:ring-gray-900">
  {m.a11y_skip_to_content()}
</a>

<div class="min-h-screen flex flex-col bg-gray-50">
  <Header navItems={data.headerNav?.items ?? []} customer={data.customer} />
  <main id="main-content" tabindex="-1" class="flex-1">
    {@render children()}
  </main>
  <Footer navItems={data.footerNav?.items ?? []} />
</div>
