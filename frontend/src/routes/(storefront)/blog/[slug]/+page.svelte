<script lang="ts">
  import type { PageData } from './$types';
  import { page } from '$app/state';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';

  // Blog cover is full-bleed within the article container (max ~960px on lg)
  // at 16:7 aspect — treat it as the LCP for the post. Widths must come from
  // backend allowedWidths (resize.go) — 1024 isn't bucketed there so we use
  // 960 instead.
  const COVER_WIDTHS = [768, 960, 1280, 1600];
  const COVER_SIZES = '(min-width: 1024px) 960px, 100vw';

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

</script>

<Seo
  title={m.blog_post_title({ title: post.title })}
  description={blogDescription}
  canonical={blogCanonical}
  image={post.cover_image_url}
  type="article"
  jsonLd={blogJsonLd}
/>

<article class="max-w-[1280px] mx-auto px-4 lg:px-8 py-12 sm:py-16">
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
      <ResponsiveImage src={post.cover_image_url} alt={post.title}
                       widths={COVER_WIDTHS} sizes={COVER_SIZES}
                       loading="eager" fetchpriority="high"
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
    <MarkdownContent content={post.content} refs={data.shortcodeRefs} />
  </div>
</article>
