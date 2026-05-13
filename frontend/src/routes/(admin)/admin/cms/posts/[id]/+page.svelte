<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import ShortcodeToolbar from '$lib/components/admin/ShortcodeToolbar.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import * as m from '$lib/paraglide/messages';

  // Cover preview thumbnail (aspect-video card on the right rail) and the
  // in-preview cover. coverImageUrl may be external — ResponsiveImage falls
  // through to a plain <img> for non-/uploads/ paths.
  const COVER_WIDTHS = [480, 768, 960];
  const COVER_SIZES = '(min-width: 1024px) 320px, 50vw';

  let { data }: { data: PageData } = $props();

  const p = data.post;
  const isNew = !p;
  let saving = $state(false);

  let title = $state(p?.title ?? '');
  let slug = $state(p?.slug ?? '');
  let excerpt = $state(p?.excerpt ?? '');
  let content = $state(p?.content ?? '');
  let coverImageUrl = $state(p?.cover_image_url ?? '');
  let categoryID = $state(p?.category_id ?? '');
  let categoryIDs = $state<string[]>(p?.category_ids ?? []);
  const sortedCategories = $derived(
    data.categories.toSorted((a, b) => a.sort_order - b.sort_order)
  );
  const categoryOptions = $derived(
    sortedCategories.map((c) => ({ value: c.id, label: c.name }))
  );
  const primaryCategoryChoices = $derived(
    sortedCategories.filter((c) => categoryIDs.includes(c.id))
  );
  $effect(() => {
    if (categoryID && !categoryIDs.includes(categoryID)) {
      categoryID = '';
    }
  });
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
  let contentTextarea = $state<HTMLTextAreaElement | null>(null);
</script>

<svelte:head>
  <title>{isNew ? m.admin_cms_post_edit_new_title() : m.admin_cms_post_edit_edit_title({ title: p?.title ?? '' })}</title>
</svelte:head>

<div class="max-w-5xl mx-auto space-y-6">
  <!-- Back + header -->
  <div class="flex items-center gap-4">
    <a href="/admin/cms/posts"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900">{isNew ? m.admin_cms_post_edit_new_heading() : m.admin_cms_post_edit_edit_heading()}</h2>

    <!-- Preview toggle -->
    <button type="button" onclick={() => preview = !preview}
            class="ml-auto inline-flex items-center gap-2 px-3.5 py-2 rounded-xl border border-gray-200
                   text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.641 0-8.573-3.007-9.964-7.178Z"/>
        <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
      </svg>
      {preview ? m.admin_cms_post_edit_edit_button() : m.admin_cms_post_edit_preview_button()}
    </button>
  </div>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          const postTitle = title;
          return async ({ result, update }) => {
            showResult(result,
              isNew ? m.admin_cms_post_edit_create_success({ title: postTitle }) : m.admin_cms_post_edit_save_success({ title: postTitle }),
              isNew ? m.admin_cms_post_edit_create_failure({ title: postTitle }) : m.admin_cms_post_edit_save_failure({ title: postTitle }));
            await update();
            saving = false;
          };
        }}>
    <div class="{preview ? 'grid grid-cols-1 lg:grid-cols-2 gap-6' : 'space-y-6'}">

      <!-- Editor column -->
      <div class="space-y-6">
        <!-- Main card -->
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-5 space-y-5">
            <!-- Title -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_cms_post_edit_label_title()}</label>
              <input type="text" name="title" bind:value={title} oninput={onTitleInput}
                     required placeholder={m.admin_cms_post_edit_title_placeholder()}
                     class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                            text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                            focus:ring-gray-900 focus:border-transparent transition" />
            </div>

            <!-- Slug -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_cms_post_edit_label_slug()}</label>
              <div class="flex items-center">
                <span class="px-3.5 py-2.5 bg-gray-50 border border-r-0 border-gray-200 rounded-l-xl
                             text-sm text-gray-400 select-none">/blog/</span>
                <input type="text" name="slug" bind:value={slug}
                       required placeholder={m.admin_cms_post_edit_slug_placeholder()}
                       class="w-full flex-1 px-3.5 py-2.5 border border-gray-200 rounded-r-xl text-sm
                              text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                              focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
              </div>
            </div>

            <!-- Excerpt -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
                {m.admin_cms_post_edit_label_excerpt()} <span class="normal-case font-normal text-gray-400">{m.admin_cms_post_edit_excerpt_hint()}</span>
              </label>
              <textarea name="excerpt" bind:value={excerpt} rows="2"
                        placeholder={m.admin_cms_post_edit_excerpt_placeholder()}
                        class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                               text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                               focus:ring-gray-900 focus:border-transparent transition resize-none"></textarea>
            </div>

            <!-- Content -->
            <div>
              <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
                {m.admin_cms_post_edit_label_content()} <span class="normal-case font-normal text-gray-400">{m.admin_cms_post_edit_content_markdown_hint()}</span>
              </label>
              <ShortcodeToolbar bind:value={content} textarea={contentTextarea} />
              <textarea name="content" bind:value={content} bind:this={contentTextarea} rows="20"
                        placeholder={m.admin_cms_post_edit_content_placeholder()}
                        class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                               text-gray-900 placeholder-gray-400 font-mono leading-relaxed
                               focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                               transition resize-y"></textarea>
            </div>
          </div>
        </div>

        <!-- Categories -->
        {#if data.categories.length > 0}
          <div class="bg-white rounded-2xl border border-gray-100">
            <div class="px-6 py-4 border-b border-gray-50">
              <h3 class="text-sm font-semibold text-gray-700">{m.admin_cms_post_edit_section_category()}</h3>
            </div>
            <div class="px-6 py-5 grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div class="flex flex-col gap-1.5">
                <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                  {m.admin_cms_post_edit_additional_categories()}
                </span>
                <MultiSelect
                  options={categoryOptions}
                  selected={categoryIDs}
                  placeholder={m.admin_cms_post_edit_additional_categories_placeholder()}
                  onChange={(values) => (categoryIDs = values)}
                />
                {#each categoryIDs as id (id)}
                  <input type="hidden" name="category_ids" value={id} />
                {/each}
              </div>
              <div class="flex flex-col gap-1.5">
                <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                  {m.admin_cms_post_edit_label_primary_category()}
                </label>
                <select name="category_id" bind:value={categoryID}
                        class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                               text-gray-900 focus:outline-none focus:ring-2 focus:ring-gray-900
                               focus:border-transparent transition bg-white">
                  <option value="">{m.admin_cms_post_edit_no_category()}</option>
                  {#each primaryCategoryChoices as cat}
                    <option value={cat.id}>{cat.name}</option>
                  {/each}
                </select>
              </div>
            </div>
          </div>
        {/if}

        <!-- Cover image -->
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-50">
            <h3 class="text-sm font-semibold text-gray-700">{m.admin_cms_post_edit_section_cover()}</h3>
          </div>
          <div class="px-6 py-5">
            <input type="url" name="cover_image_url" bind:value={coverImageUrl}
                   placeholder={m.admin_cms_post_edit_cover_placeholder()}
                   class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                          text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                          focus:ring-gray-900 focus:border-transparent transition" />
            {#if coverImageUrl}
              <div class="mt-3 rounded-xl overflow-hidden bg-gray-50 aspect-video">
                <ResponsiveImage src={coverImageUrl} alt={m.admin_cms_post_edit_cover_alt_preview()}
                                 widths={COVER_WIDTHS} sizes={COVER_SIZES}
                                 class="w-full h-full object-cover" />
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Preview column -->
      {#if preview}
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-50">
            <h3 class="text-sm font-semibold text-gray-700">{m.admin_cms_post_edit_section_preview()}</h3>
          </div>
          <div class="px-6 py-5 prose prose-sm max-w-none overflow-y-auto" style="max-height: 80vh">
            {#if coverImageUrl}
              <div class="rounded-xl overflow-hidden mb-6 aspect-video bg-gray-50">
                <ResponsiveImage src={coverImageUrl} alt={m.admin_cms_post_edit_cover_alt()}
                                 widths={COVER_WIDTHS} sizes={COVER_SIZES}
                                 class="w-full h-full object-cover" />
              </div>
            {/if}
            <h1 class="text-2xl font-bold text-gray-900 mb-2">{title || m.admin_cms_post_edit_preview_default_title()}</h1>
            {#if excerpt}
              <p class="text-gray-500 text-sm mb-4 italic">{excerpt}</p>
            {/if}
            <hr class="my-4 border-gray-100" />
            <div class="text-sm text-gray-700 leading-relaxed">
              <MarkdownContent content={content || m.admin_cms_post_edit_preview_no_content()} placeholderMode />
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
          {isPublished ? m.admin_cms_posts_status_published() : m.admin_cms_posts_status_draft()}
        </span>
      </label>
      <div class="sm:ml-auto flex gap-3">
        <a href="/admin/cms/posts"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          {m.admin_cms_post_edit_cancel()}
        </a>
        <SaveButton loading={saving}
                class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                       text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
          {isNew ? m.admin_cms_post_edit_create() : m.admin_cms_post_edit_save()}
        </SaveButton>
      </div>
    </div>
  </form>
</div>
