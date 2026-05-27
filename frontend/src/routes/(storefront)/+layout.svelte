<script lang="ts">
  import '../../app.css';
  import Header from '$lib/components/Header.svelte';
  import Footer from '$lib/components/Footer.svelte';
  import AnnouncementStrip from '$lib/components/shop/AnnouncementStrip.svelte';
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

  const companyLogoUrl = $derived(
    data.publicSettings?.find((s) => s.key === 'company_logo_url')?.value ?? ''
  );
  const companyLogoHeight = $derived(
    Number(data.publicSettings?.find((s) => s.key === 'company_logo_height_px')?.value) || 40
  );

  const companyLogoFooterUrl = $derived(
    data.publicSettings?.find((s) => s.key === 'company_logo_footer_url')?.value ?? ''
  );
  const companyLogoFooterHeight = $derived(
    Number(data.publicSettings?.find((s) => s.key === 'company_logo_footer_height_px')?.value) || 40
  );

  const websiteSlogan = $derived(
    data.publicSettings?.find((s) => s.key === 'website_slogan')?.value ?? ''
  );

  onMount(async () => {
    initTracker(data.publicSettings ?? []);
    if ('serviceWorker' in navigator) {
      if (data.pwaEnabled) {
        try {
          await navigator.serviceWorker.register('/service-worker.js', { type: 'module' });
        } catch (e) {
          console.warn('Service worker registration failed', e);
        }
      } else {
        const regs = await navigator.serviceWorker.getRegistrations();
        await Promise.all(regs.map((r) => r.unregister()));
      }
    }
    await cartStore.init();
    await wishlistStore.init(!!data.customer);
    await registerStorefrontTools(data.mcpEnabled);
  });

  // Surface cart-add failures (e.g. role-purchase 403 from the backend) as a
  // bottom-center toast. Auto-clears after 4s; clicking dismisses immediately.
  // Lives in the layout so every storefront cart-add path benefits without
  // each component duplicating error UI.
  let toastTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!cartStore.error) return;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => cartStore.clearError(), 4000);
    return () => {
      if (toastTimer) clearTimeout(toastTimer);
      toastTimer = null;
    };
  });
</script>

<svelte:head>
  {#if faviconUrl}
    <link rel="icon" href={faviconUrl} />
    <link rel="apple-touch-icon" href={faviconUrl} />
  {/if}
  {#if data.pwaEnabled}
    <link rel="manifest" href="/manifest.webmanifest" />
    <meta name="theme-color" content="#111827" />
    <meta name="mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-status-bar-style" content="default" />
    <meta name="apple-mobile-web-app-title" content="Gyeon" />
    {#if !faviconUrl}
      <link rel="apple-touch-icon" href="/icon.svg" />
    {/if}
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

<div class="min-h-screen flex flex-col bg-white">
  <AnnouncementStrip settings={data.publicSettings ?? []} />
  <Header
    navItems={data.headerNav?.items ?? []}
    customer={data.customer}
    blogEnabled={data.blogEnabled}
    {companyLogoUrl}
    {companyLogoHeight}
  />
  <main id="main-content" tabindex="-1" class="flex-1">
    {@render children()}
  </main>
  <Footer
    navItems={data.footerNav?.items ?? []}
    socials={data.socials}
    {companyLogoFooterUrl}
    {companyLogoFooterHeight}
    slogan={websiteSlogan}
  />
</div>

{#if cartStore.error}
  <div
    role="status"
    aria-live="polite"
    class="fixed bottom-6 left-1/2 -translate-x-1/2 z-[300] max-w-[90vw]
           px-4 py-3 rounded-md bg-ink-900 text-white text-sm font-body
           shadow-lg cursor-pointer"
    onclick={() => cartStore.clearError()}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') cartStore.clearError(); }}
    tabindex="0"
  >
    {cartStore.error}
  </div>
{/if}
