<script lang="ts">
  import type { PageData } from './$types';
  import type { CmsPost } from '$lib/api/admin';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<CmsPost | null>(null);

  const publishedCount = $derived(data.posts.filter(p => p.is_published).length);
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">Posts</h2>
      <p class="text-sm text-gray-500 mt-0.5">
        {data.posts.length} total · {publishedCount} published
      </p>
    </div>
    <a href="/admin/cms/posts/new"
       class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
              text-sm font-medium hover:bg-gray-700 transition-colors">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/>
      </svg>
      New Post
    </a>
  </div>

  <!-- List -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    {#if data.posts.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <div class="w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mb-3">
          <svg class="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 0 1-2.25 2.25M16.5 7.5V18a2.25 2.25 0 0 0 2.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 0 0 2.25 2.25h13.5M6 7.5h3v3H6v-3Z"/>
          </svg>
        </div>
        <p class="text-sm font-medium text-gray-400">No posts yet</p>
        <a href="/admin/cms/posts/new" class="mt-3 text-sm text-gray-900 underline underline-offset-2">
          Write your first post
        </a>
      </div>
    {:else}
      <!-- Mobile cards -->
      <div class="divide-y divide-gray-50 sm:hidden">
        {#each data.posts as post}
          <div class="px-4 py-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="text-sm font-semibold text-gray-900 truncate">{post.title}</p>
                {#if post.excerpt}
                  <p class="text-xs text-gray-400 mt-0.5 line-clamp-1">{post.excerpt}</p>
                {/if}
              </div>
              <div class="flex items-center gap-1.5 flex-shrink-0">
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                             {post.is_published ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'}">
                  {post.is_published ? 'Published' : 'Draft'}
                </span>
                <a href="/admin/cms/posts/{post.id}"
                   class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                  </svg>
                </a>
              </div>
            </div>
          </div>
        {/each}
      </div>

      <!-- Desktop table -->
      <table class="hidden sm:table w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">Title</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">Excerpt</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">Status</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">Published</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.posts as post}
            <tr class="hover:bg-gray-50/50 transition-colors">
              <td class="px-6 py-4">
                <p class="font-medium text-gray-900">{post.title}</p>
                <p class="text-xs text-gray-400 font-mono mt-0.5">/{post.slug}</p>
              </td>
              <td class="px-6 py-4 text-gray-500 max-w-xs">
                <p class="truncate">{post.excerpt ?? '—'}</p>
              </td>
              <td class="px-6 py-4">
                <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                             {post.is_published ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'}">
                  {post.is_published ? 'Published' : 'Draft'}
                </span>
              </td>
              <td class="px-6 py-4 text-gray-400 text-xs">
                {post.published_at ? new Date(post.published_at).toLocaleDateString() : '—'}
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center justify-end gap-2">
                  <a href="/admin/cms/posts/{post.id}"
                     class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                    </svg>
                  </a>
                  <button onclick={() => deleteTarget = post}
                          class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteTarget = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">Delete post?</h3>
      <p class="text-sm text-gray-500 mb-5">
        "<span class="font-medium text-gray-700">{deleteTarget.title}</span>" will be permanently deleted.
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </button>
        <form method="POST" action="?/delete" class="flex-1">
          <input type="hidden" name="id" value={deleteTarget.id} />
          <button type="submit"
                  class="w-full px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors">
            Delete
          </button>
        </form>
      </div>
    </div>
  </div>
{/if}
