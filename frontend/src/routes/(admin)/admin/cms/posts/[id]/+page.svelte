<script lang="ts">
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  const p = data.post;
  const isNew = !p;

  let title = $state(p?.title ?? '');
  let slug = $state(p?.slug ?? '');
  let excerpt = $state(p?.excerpt ?? '');
  let content = $state(p?.content ?? '');
  let coverImageUrl = $state(p?.cover_image_url ?? '');
  let categoryID = $state(p?.category_id ?? '');
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

  // Split-pane preview state
  let preview = $state(false);

  // Very basic Markdown → HTML preview (headings, bold, italic, code, links, line breaks)
  function markdownToHtml(md: string): string {
    return md
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/^### (.+)$/gm, '<h3 class="text-lg font-bold mt-5 mb-2">$1</h3>')
      .replace(/^## (.+)$/gm, '<h2 class="text-xl font-bold mt-6 mb-2">$1</h2>')
      .replace(/^# (.+)$/gm, '<h1 class="text-2xl font-bold mt-6 mb-3">$1</h1>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/`(.+?)`/g, '<code class="bg-gray-100 px-1 rounded text-xs font-mono">$1</code>')
      .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" class="text-blue-600 underline">$1</a>')
      .replace(/^- (.+)$/gm, '<li class="ml-4 list-disc">$1</li>')
      .replace(/\n\n/g, '</p><p class="mb-3">')
      .replace(/\n/g, '<br>');
  }
</script>

<div class="max-w-5xl mx-auto space-y-6">
  <!-- Back + header -->
  <div class="flex items-center gap-4">
    <a href="/admin/cms/posts"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900">{isNew ? 'New Post' : 'Edit Post'}</h2>

    <!-- Preview toggle -->
    <button type="button" onclick={() => preview = !preview}
            class="ml-auto inline-flex items-center gap-2 px-3.5 py-2 rounded-xl border border-gray-200
                   text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.641 0-8.573-3.007-9.964-7.178Z"/>
        <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
      </svg>
      {preview ? 'Edit' : 'Preview'}
    </button>
  </div>

  <form method="POST" action="?/save" class="space-y-6">
    <div class="{preview ? 'grid grid-cols-1 lg:grid-cols-2 gap-6' : 'space-y-6'}">

      <!-- Editor column -->
      <div class="space-y-6">
        <!-- Main card -->
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-5 space-y-5">
            <!-- Title -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Title</label>
              <input type="text" name="title" bind:value={title} oninput={onTitleInput}
                     required placeholder="Post title"
                     class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                            text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                            focus:ring-gray-900 focus:border-transparent transition" />
            </div>

            <!-- Slug -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Slug</label>
              <div class="flex items-center">
                <span class="px-3.5 py-2.5 bg-gray-50 border border-r-0 border-gray-200 rounded-l-xl
                             text-sm text-gray-400 select-none">/blog/</span>
                <input type="text" name="slug" bind:value={slug}
                       required placeholder="post-url-slug"
                       class="flex-1 px-3.5 py-2.5 border border-gray-200 rounded-r-xl text-sm
                              text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                              focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
              </div>
            </div>

            <!-- Excerpt -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
                Excerpt <span class="normal-case font-normal text-gray-400">(shown in post list)</span>
              </label>
              <textarea name="excerpt" bind:value={excerpt} rows="2"
                        placeholder="Short summary of this post..."
                        class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                               text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                               focus:ring-gray-900 focus:border-transparent transition resize-none"></textarea>
            </div>

            <!-- Content -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
                Content <span class="normal-case font-normal text-gray-400">(Markdown)</span>
              </label>
              <textarea name="content" bind:value={content} rows="20"
                        placeholder="Write your post in Markdown..."
                        class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                               text-gray-900 placeholder-gray-400 font-mono leading-relaxed
                               focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                               transition resize-y"></textarea>
            </div>
          </div>
        </div>

        <!-- Category -->
        {#if data.categories.length > 0}
          <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
            <div class="px-6 py-4 border-b border-gray-50">
              <h3 class="text-sm font-semibold text-gray-700">Category</h3>
            </div>
            <div class="px-6 py-5">
              <select name="category_id" bind:value={categoryID}
                      class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                             text-gray-900 focus:outline-none focus:ring-2 focus:ring-gray-900
                             focus:border-transparent transition bg-white">
                <option value="">— No category —</option>
                {#each data.categories.toSorted((a, b) => a.sort_order - b.sort_order) as cat}
                  <option value={cat.id}>{cat.name}</option>
                {/each}
              </select>
            </div>
          </div>
        {/if}

        <!-- Cover image -->
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-50">
            <h3 class="text-sm font-semibold text-gray-700">Cover Image</h3>
          </div>
          <div class="px-6 py-5">
            <input type="url" name="cover_image_url" bind:value={coverImageUrl}
                   placeholder="https://example.com/image.jpg"
                   class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                          text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                          focus:ring-gray-900 focus:border-transparent transition" />
            {#if coverImageUrl}
              <div class="mt-3 rounded-xl overflow-hidden bg-gray-50 aspect-video">
                <img src={coverImageUrl} alt="Cover preview" class="w-full h-full object-cover" />
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Preview column -->
      {#if preview}
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-50">
            <h3 class="text-sm font-semibold text-gray-700">Preview</h3>
          </div>
          <div class="px-6 py-5 prose prose-sm max-w-none overflow-y-auto" style="max-height: 80vh">
            {#if coverImageUrl}
              <div class="rounded-xl overflow-hidden mb-6 aspect-video bg-gray-50">
                <img src={coverImageUrl} alt="Cover" class="w-full h-full object-cover" />
              </div>
            {/if}
            <h1 class="text-2xl font-bold text-gray-900 mb-2">{title || 'Post Title'}</h1>
            {#if excerpt}
              <p class="text-gray-500 text-sm mb-4 italic">{excerpt}</p>
            {/if}
            <hr class="my-4 border-gray-100" />
            <!-- svelte-ignore html-non-void-elements-not-closed -->
            <div class="text-sm text-gray-700 leading-relaxed">
              {@html `<p class="mb-3">${markdownToHtml(content || '_No content yet_')}</p>`}
            </div>
          </div>
        </div>
      {/if}
    </div>

    <!-- Publish + submit -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5
                flex flex-col sm:flex-row sm:items-center gap-4">
      <label class="flex items-center gap-3 cursor-pointer select-none">
        <div class="relative">
          <input type="checkbox" class="sr-only peer" bind:checked={isPublished} />
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
        <a href="/admin/cms/posts"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </a>
        <button type="submit"
                class="px-5 py-2.5 rounded-xl bg-gray-900 text-white text-sm font-medium
                       hover:bg-gray-700 transition-colors">
          {isNew ? 'Create Post' : 'Save Changes'}
        </button>
      </div>
    </div>
  </form>
</div>
