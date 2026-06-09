<script lang="ts">
  import { page } from '$app/state';
  import { siteDescription } from '$lib/seo';

  interface SeoProps {
    title: string;
    description?: string;
    canonical?: string;
    image?: string;
    type?: 'website' | 'article' | 'product';
    siteName?: string;
    /** Optional JSON-LD structured data (will be JSON-stringified). */
    jsonLd?: unknown;
  }

  let { title, description, canonical, image, type = 'website', siteName = 'GYEON', jsonLd }: SeoProps = $props();

  // Fall back to the site-wide 網站描述 (site_description setting) when the page
  // sets no description of its own. Safe when publicSettings is absent (yields '').
  const finalDescription = $derived(
    description?.trim() || siteDescription(page.data?.publicSettings) || undefined
  );
</script>

<svelte:head>
  <title>{title}</title>
  {#if finalDescription}
    <meta name="description" content={finalDescription} />
  {/if}
  {#if canonical}
    <link rel="canonical" href={canonical} />
  {/if}

  <!-- Open Graph -->
  <meta property="og:type" content={type} />
  <meta property="og:title" content={title} />
  {#if finalDescription}
    <meta property="og:description" content={finalDescription} />
  {/if}
  {#if canonical}
    <meta property="og:url" content={canonical} />
  {/if}
  {#if image}
    <meta property="og:image" content={image} />
  {/if}
  <meta property="og:site_name" content={siteName} />

  <!-- Twitter -->
  <meta name="twitter:card" content={image ? 'summary_large_image' : 'summary'} />
  <meta name="twitter:title" content={title} />
  {#if finalDescription}
    <meta name="twitter:description" content={finalDescription} />
  {/if}
  {#if image}
    <meta name="twitter:image" content={image} />
  {/if}

  {#if jsonLd}
    {@html `<script type="application/ld+json">${JSON.stringify(jsonLd).replace(/</g, '\\u003c')}<\/script>`}
  {/if}
</svelte:head>
