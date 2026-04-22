<script lang="ts">
  import '../../app.css';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { onMount } from 'svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  onMount(() => cartStore.init());
</script>

<div class="min-h-screen flex flex-col bg-gray-50">
  <Header navItems={data.headerNav?.items ?? []} />
  <main class="flex-1">
    {@render children()}
  </main>
  <Footer />
</div>
