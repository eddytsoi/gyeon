<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();

  const p = data.page;
  const isNew = !p;

  let title = $state(p?.title ?? '');
  let slug = $state(p?.slug ?? '');
  let content = $state(p?.content ?? '');
  let metaTitle = $state(p?.meta_title ?? '');
  let metaDesc = $state(p?.meta_desc ?? '');
  let isPublished = $state(p?.is_published ?? false);

  // Auto-generate slug from title when creating
  function onTitleInput() {
    if (isNew) {
      slug = title
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '');
    }
  }
</script>

<div class="max-w-4xl mx-auto space-y-6">
  <!-- Back + header -->
  <div class="flex items-center gap-4">
    <a href="/admin/cms/pages"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900">{isNew ? 'New Page' : 'Edit Page'}</h2>
  </div>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          const pageTitle = title;
          return async ({ result, update }) => {
            showResult(result,
              isNew ? `Page '${pageTitle}' created` : `Page '${pageTitle}' saved`,
              isNew ? `Failed to create page '${pageTitle}'` : `Failed to save page '${pageTitle}'`);
            await update();
          };
        }}>
    <!-- Main card -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-5 space-y-5">
        <!-- Title -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            Title
          </label>
          <input type="text" name="title" bind:value={title} oninput={onTitleInput}
                 required placeholder="Page title"
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <!-- Slug -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            Slug
          </label>
          <div class="flex items-center">
            <span class="px-3.5 py-2.5 bg-gray-50 border border-r-0 border-gray-200 rounded-l-xl
                         text-sm text-gray-400 select-none">/</span>
            <input type="text" name="slug" bind:value={slug}
                   required placeholder="page-url-slug"
                   class="w-full flex-1 px-3.5 py-2.5 border border-gray-200 rounded-r-xl text-sm
                          text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                          focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
          </div>
        </div>

        <!-- Content -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            Content <span class="normal-case font-normal text-gray-400">(Markdown)</span>
          </label>
          <textarea name="content" bind:value={content} rows="16"
                    placeholder="Write your page content in Markdown..."
                    class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                           text-gray-900 placeholder-gray-400 font-mono leading-relaxed
                           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                           transition resize-y"></textarea>
        </div>
      </div>
    </div>

    <!-- SEO card -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-4 border-b border-gray-50">
        <h3 class="text-sm font-semibold text-gray-700">SEO</h3>
      </div>
      <div class="px-6 py-5 space-y-4">
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            Meta Title
          </label>
          <input type="text" name="meta_title" bind:value={metaTitle}
                 placeholder="Overrides page title in search results"
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            Meta Description
          </label>
          <textarea name="meta_desc" bind:value={metaDesc} rows="2"
                    placeholder="Short description shown in search results (max ~160 chars)"
                    class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                           text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                           focus:ring-gray-900 focus:border-transparent transition resize-none"></textarea>
        </div>
      </div>
    </div>

    <!-- Publish + submit -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5
                flex flex-col sm:flex-row sm:items-center gap-4">
      <label class="flex items-center gap-3 cursor-pointer select-none">
        <div class="relative">
          <input type="checkbox" class="sr-only peer"
                 bind:checked={isPublished} />
          <input type="hidden" name="is_published" value={isPublished ? 'true' : 'false'} />
          <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
          <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                      transition-transform peer-checked:translate-x-4"></div>
        </div>
        <span class="text-sm font-medium text-gray-700">
          {isPublished ? 'Published' : 'Draft'}
        </span>
      </label>
      <div class="sm:ml-auto flex gap-3">
        <a href="/admin/cms/pages"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </a>
        <button type="submit"
                class="px-5 py-2.5 rounded-xl bg-gray-900 text-white text-sm font-medium
                       hover:bg-gray-700 transition-colors">
          {isNew ? 'Create Page' : 'Save Changes'}
        </button>
      </div>
    </div>
  </form>
</div>
