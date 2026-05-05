<script lang="ts">
  import type { PageData } from './$types';
  import { page } from '$app/state';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';

  let { data }: { data: PageData } = $props();
  const { post } = data;
  const blogOrigin = $derived(siteOrigin(page.data.publicSettings));
  const blogCanonical = $derived(`${blogOrigin}/blog/${post.slug}`);
  const blogDescription = $derived(snippet(post.excerpt || post.content));
  const blogJsonLd = $derived({
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: post.title,
    description: blogDescription,
    datePublished: post.published_at ?? post.created_at,
    dateModified: post.updated_at,
    url: blogCanonical,
    ...(post.cover_image_url ? { image: post.cover_image_url } : {})
  });

  // Simple Markdown renderer (no external deps)
  function renderMarkdown(md: string): string {
    return md
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      // Headings
      .replace(/^#### (.+)$/gm, '<h4 class="text-base font-bold mt-6 mb-1 text-gray-900">$1</h4>')
      .replace(/^### (.+)$/gm, '<h3 class="text-lg font-bold mt-7 mb-2 text-gray-900">$1</h3>')
      .replace(/^## (.+)$/gm, '<h2 class="text-xl font-bold mt-8 mb-2 text-gray-900">$1</h2>')
      .replace(/^# (.+)$/gm, '<h1 class="text-2xl font-bold mt-8 mb-3 text-gray-900">$1</h1>')
      // Bold / italic / inline code
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/`(.+?)`/g, '<code class="bg-gray-100 text-gray-800 px-1.5 py-0.5 rounded text-sm font-mono">$1</code>')
      // Links
      .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" class="text-gray-900 underline underline-offset-2 hover:text-gray-600">$1</a>')
      // Blockquote
      .replace(/^> (.+)$/gm, '<blockquote class="border-l-4 border-gray-200 pl-4 italic text-gray-500 my-4">$1</blockquote>')
      // Unordered list items
      .replace(/^- (.+)$/gm, '<li class="ml-5 list-disc mb-1">$1</li>')
      // Ordered list items
      .replace(/^\d+\. (.+)$/gm, '<li class="ml-5 list-decimal mb-1">$1</li>')
      // Horizontal rule
      .replace(/^---$/gm, '<hr class="my-8 border-gray-100" />')
      // Paragraphs (double newline)
      .replace(/\n\n/g, '</p><p class="mb-5 leading-relaxed text-gray-700">')
      .replace(/\n/g, '<br />');
  }
</script>

<Seo
  title={m.blog_post_title({ title: post.title })}
  description={blogDescription}
  canonical={blogCanonical}
  image={post.cover_image_url}
  type="article"
  jsonLd={blogJsonLd}
/>

<article class="max-w-3xl mx-auto px-4 py-12 sm:py-16">
  <!-- Breadcrumbs -->
  <nav class="flex flex-wrap gap-2 items-center text-[11px] uppercase tracking-[0.15em] text-gray-400 mb-10">
    <a href="/" class="hover:text-gray-700 transition-colors">{m.common_home()}</a>
    <span>/</span>
    <a href="/blog" class="hover:text-gray-700 transition-colors">{m.common_blog()}</a>
    {#if post.category_slug && post.category_name}
      <span>/</span>
      <a href="/blog/category/{post.category_slug}" class="hover:text-gray-700 transition-colors">
        {post.category_name}
      </a>
    {/if}
    <span>/</span>
    <span class="font-semibold text-gray-700 truncate max-w-[40ch]">{post.title}</span>
  </nav>

  <!-- Cover image -->
  {#if post.cover_image_url}
    <div class="rounded-2xl overflow-hidden aspect-[16/7] bg-gray-100 mb-10">
      <img src={post.cover_image_url} alt={post.title}
           class="w-full h-full object-cover" />
    </div>
  {/if}

  <!-- Meta -->
  {#if post.published_at}
    <p class="text-xs text-gray-400 mb-3">
      <time datetime={post.published_at}>
        {new Date(post.published_at).toLocaleDateString('en-US', {
          year: 'numeric', month: 'long', day: 'numeric'
        })}
      </time>
    </p>
  {/if}

  <!-- Title -->
  <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight leading-tight mb-4">
    {post.title}
  </h1>

  <!-- Excerpt -->
  {#if post.excerpt}
    <p class="text-lg text-gray-500 leading-relaxed mb-8 pb-8 border-b border-gray-100">
      {post.excerpt}
    </p>
  {/if}

  <!-- Content -->
  <div class="text-gray-700 text-base leading-relaxed">
    {@html `<p class="mb-5 leading-relaxed text-gray-700">${renderMarkdown(post.content)}</p>`}
  </div>
</article>
