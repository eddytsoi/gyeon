<script lang="ts">
  import type { PageData } from './$types';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import { siteName, siteDescription } from '$lib/seo';

  let { data }: { data: PageData } = $props();
  const { page } = data;
  // Page's own meta description, else the site-wide 網站描述 fallback.
  const metaDescription = $derived(page.meta_desc?.trim() || siteDescription(data.publicSettings));
</script>

<svelte:head>
  <title>{page.meta_title ?? page.title} — {siteName(data.publicSettings)}</title>
  {#if metaDescription}<meta name="description" content={metaDescription} />{/if}
</svelte:head>

<div class="max-w-7xl mx-auto px-4 lg:px-8 {page.content_padded === false ? '' : 'py-12 sm:py-16'}">
  {#if page.show_title}
    <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight mb-8">
      {page.title}
    </h1>
  {/if}

  <div class="text-gray-700 text-base leading-relaxed">
    <MarkdownContent content={page.content} refs={data.shortcodeRefs} />
  </div>
</div>
