<script lang="ts">
  import type { PageData } from './$types';
  import { renderMarkdown } from '$lib/markdown';

  let { data }: { data: PageData } = $props();
  const { page } = data;
</script>

<svelte:head>
  <title>{page.meta_title ?? page.title} — Gyeon</title>
  {#if page.meta_desc}<meta name="description" content={page.meta_desc} />{/if}
</svelte:head>

<div class="max-w-3xl mx-auto px-4 py-12 sm:py-16">
  <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight mb-8">
    {page.title}
  </h1>

  <div class="text-gray-700 text-base leading-relaxed">
    {@html renderMarkdown(page.content)}
  </div>
</div>
