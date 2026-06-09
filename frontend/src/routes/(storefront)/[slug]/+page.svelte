<script lang="ts">
  import type { PageData } from './$types';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import Seo from '$lib/components/Seo.svelte';
  import { siteName, siteOrigin, snippet } from '$lib/seo';

  let { data }: { data: PageData } = $props();
  const { page } = data;
</script>

<Seo
  title={`${page.meta_title ?? page.title} — ${siteName(data.publicSettings)}`}
  description={page.meta_desc ?? snippet(page.content)}
  canonical={`${siteOrigin(data.publicSettings)}/${page.slug}`}
/>

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
