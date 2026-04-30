<script lang="ts">
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{data.category.name} — Gyeon Blog</title>
  <meta name="description" content={`${data.category.name} posts from the Gyeon blog`} />
</svelte:head>

<div class="max-w-4xl mx-auto px-4 py-12 sm:py-16">
  <!-- Breadcrumbs -->
  <nav class="flex gap-2 items-center text-[11px] uppercase tracking-[0.15em] text-gray-400 mb-6">
    <a href="/" class="hover:text-gray-700 transition-colors">Home</a>
    <span>/</span>
    <a href="/blog" class="hover:text-gray-700 transition-colors">Blog</a>
    <span>/</span>
    <span class="font-semibold text-gray-700">{data.category.name}</span>
  </nav>

  <!-- Header -->
  <div class="mb-10 sm:mb-14">
    <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight">{data.category.name}</h1>
    <p class="mt-2 text-gray-500">Stories filed under {data.category.name}.</p>
  </div>

  {#if data.posts.length === 0}
    <div class="flex flex-col items-center justify-center py-24 text-center">
      <p class="text-gray-400 text-lg">No posts in this category yet — check back soon.</p>
    </div>
  {:else}
    <div class="space-y-10">
      {#each data.posts as post}
        <article class="group">
          <a href="/blog/{post.slug}" class="block">
            <!-- Cover image -->
            {#if post.cover_image_url}
              <div class="rounded-2xl overflow-hidden aspect-[16/7] bg-gray-100 mb-5">
                <img src={post.cover_image_url} alt={post.title}
                     class="w-full h-full object-cover transition-transform duration-500
                            group-hover:scale-105" />
              </div>
            {/if}

            <!-- Meta -->
            <div class="flex items-center gap-2 text-xs text-gray-400 mb-2">
              {#if post.published_at}
                <time datetime={post.published_at}>
                  {new Date(post.published_at).toLocaleDateString('en-US', {
                    year: 'numeric', month: 'long', day: 'numeric'
                  })}
                </time>
              {/if}
            </div>

            <!-- Title -->
            <h2 class="text-xl sm:text-2xl font-bold text-gray-900 leading-snug
                       group-hover:text-gray-600 transition-colors">
              {post.title}
            </h2>

            <!-- Excerpt -->
            {#if post.excerpt}
              <p class="mt-2 text-gray-500 leading-relaxed line-clamp-3">{post.excerpt}</p>
            {/if}

            <!-- Read more -->
            <div class="mt-4 inline-flex items-center gap-1.5 text-sm font-medium text-gray-900
                        group-hover:gap-3 transition-all duration-200">
              Read more
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3"/>
              </svg>
            </div>
          </a>

          <!-- Divider -->
          <div class="mt-10 border-t border-gray-100"></div>
        </article>
      {/each}
    </div>
  {/if}
</div>
