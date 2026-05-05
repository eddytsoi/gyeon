<script lang="ts">
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

  let { title, description, canonical, image, type = 'website', siteName = 'Gyeon', jsonLd }: SeoProps = $props();
</script>

<svelte:head>
  <title>{title}</title>
  {#if description}
    <meta name="description" content={description} />
  {/if}
  {#if canonical}
    <link rel="canonical" href={canonical} />
  {/if}

  <!-- Open Graph -->
  <meta property="og:type" content={type} />
  <meta property="og:title" content={title} />
  {#if description}
    <meta property="og:description" content={description} />
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
  {#if description}
    <meta name="twitter:description" content={description} />
  {/if}
  {#if image}
    <meta name="twitter:image" content={image} />
  {/if}

  {#if jsonLd}
    {@html `<script type="application/ld+json">${JSON.stringify(jsonLd).replace(/</g, '\\u003c')}<\/script>`}
  {/if}
</svelte:head>
