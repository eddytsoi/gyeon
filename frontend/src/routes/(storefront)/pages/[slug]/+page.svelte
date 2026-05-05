<script lang="ts">
  import type { PageData } from './$types';
  import { page as appPage } from '$app/state';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';

  let { data }: { data: PageData } = $props();
  const { page } = data;
  const cmsOrigin = $derived(siteOrigin(appPage.data.publicSettings));
  const cmsCanonical = $derived(`${cmsOrigin}/${page.slug}`);
  const cmsDescription = $derived(page.meta_desc ?? snippet(page.content));

  function renderMarkdown(md: string): string {
    return md
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/^#### (.+)$/gm, '<h4 class="text-base font-bold mt-6 mb-1 text-gray-900">$1</h4>')
      .replace(/^### (.+)$/gm, '<h3 class="text-lg font-bold mt-7 mb-2 text-gray-900">$1</h3>')
      .replace(/^## (.+)$/gm, '<h2 class="text-xl font-bold mt-8 mb-2 text-gray-900">$1</h2>')
      .replace(/^# (.+)$/gm, '<h1 class="text-2xl font-bold mt-8 mb-3 text-gray-900">$1</h1>')
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/`(.+?)`/g, '<code class="bg-gray-100 text-gray-800 px-1.5 py-0.5 rounded text-sm font-mono">$1</code>')
      .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" class="text-gray-900 underline underline-offset-2 hover:text-gray-600">$1</a>')
      .replace(/^> (.+)$/gm, '<blockquote class="border-l-4 border-gray-200 pl-4 italic text-gray-500 my-4">$1</blockquote>')
      .replace(/^- (.+)$/gm, '<li class="ml-5 list-disc mb-1">$1</li>')
      .replace(/^\d+\. (.+)$/gm, '<li class="ml-5 list-decimal mb-1">$1</li>')
      .replace(/^---$/gm, '<hr class="my-8 border-gray-100" />')
      .replace(/\n\n/g, '</p><p class="mb-5 leading-relaxed text-gray-700">')
      .replace(/\n/g, '<br />');
  }
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
    {@html `<p class="mb-5 leading-relaxed text-gray-700">${renderMarkdown(page.content)}</p>`}
  </div>
</div>
