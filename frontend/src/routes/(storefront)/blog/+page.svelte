<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.blog_title()}</title>
  <meta name="description" content={m.blog_meta_description()} />
</svelte:head>

<div class="max-w-[1280px] mx-auto px-4 lg:px-8 py-12 sm:py-16">
  <!-- Header -->
  <div class="mb-10 sm:mb-14">
    <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight">{m.blog_heading()}</h1>
    <p class="mt-2 text-gray-500">{m.blog_subheading()}</p>
  </div>

  {#if data.posts.length === 0}
    <div class="flex flex-col items-center justify-center py-24 text-center">
      <p class="text-gray-400 text-lg">{m.blog_empty()}</p>
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
              {m.blog_read_more()}
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
