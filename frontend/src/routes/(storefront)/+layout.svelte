<script lang="ts">
  import '../../app.css';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { wishlistStore } from '$lib/stores/wishlist.svelte';
  import { registerStorefrontTools } from '$lib/webmcp';
  import { onMount } from 'svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  onMount(async () => {
    await cartStore.init();
    await wishlistStore.init(!!data.customer);
    await registerStorefrontTools(data.mcpEnabled);
  });
</script>

<div class="min-h-screen flex flex-col bg-gray-50">
  <Header navItems={data.headerNav?.items ?? []} customer={data.customer} />
  <main class="flex-1">
    {@render children()}
  </main>
  <Footer navItems={data.footerNav?.items ?? []} />
</div>
