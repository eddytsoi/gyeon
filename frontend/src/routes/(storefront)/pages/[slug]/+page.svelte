<script lang="ts">
  import type { PageData } from './$types';
  import { page as appPage } from '$app/state';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';
  import { renderMarkdown } from '$lib/markdown';

  let { data }: { data: PageData } = $props();
  const { page } = data;
  const cmsOrigin = $derived(siteOrigin(appPage.data.publicSettings));
  const cmsCanonical = $derived(`${cmsOrigin}/${page.slug}`);
  const cmsDescription = $derived(page.meta_desc ?? snippet(page.content));
</script>

<Seo
  title={`${page.meta_title ?? page.title} — Gyeon`}
  description={cmsDescription}
  canonical={cmsCanonical}
/>

<div class="max-w-3xl mx-auto px-4 py-12 sm:py-16">
  <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight mb-8">
    {page.title}
  </h1>

  <div class="text-gray-700 text-base leading-relaxed">
    {@html renderMarkdown(page.content)}
  </div>
</div>
