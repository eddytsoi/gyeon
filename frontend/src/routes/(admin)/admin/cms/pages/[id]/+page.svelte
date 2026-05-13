<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import ShortcodeToolbar from '$lib/components/admin/ShortcodeToolbar.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const p = data.page;
  const isNew = !p;
  let saving = $state(false);

  let title = $state(p?.title ?? '');
  let slug = $state(p?.slug ?? '');
  let content = $state(p?.content ?? '');
  let metaTitle = $state(p?.meta_title ?? '');
  let metaDesc = $state(p?.meta_desc ?? '');
  let isPublished = $state(p?.is_published ?? false);

  let preview = $state(false);
  let contentTextarea = $state<HTMLTextAreaElement | null>(null);

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
    <h2 class="text-xl font-bold text-gray-900">{isNew ? m.admin_cms_page_edit_new_heading() : m.admin_cms_page_edit_edit_heading()}</h2>

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
          const pageTitle = title;
          return async ({ result, update }) => {
            showResult(result,
              isNew ? m.admin_cms_page_edit_create_success({ title: pageTitle }) : m.admin_cms_page_edit_save_success({ title: pageTitle }),
              isNew ? m.admin_cms_page_edit_create_failure({ title: pageTitle }) : m.admin_cms_page_edit_save_failure({ title: pageTitle }));
            await update();
            saving = false;
          };
        }}>
    <!-- Main card -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-5 space-y-5">
        <!-- Title -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_cms_page_edit_label_title()}
          </label>
          <input type="text" name="title" bind:value={title} oninput={onTitleInput}
                 required placeholder={m.admin_cms_page_edit_title_placeholder()}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <!-- Slug -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_cms_page_edit_label_slug()}
          </label>
          <div class="flex items-center">
            <span class="px-3.5 py-2.5 bg-gray-50 border border-r-0 border-gray-200 rounded-l-xl
                         text-sm text-gray-400 select-none">/</span>
            <input type="text" name="slug" bind:value={slug}
                   required placeholder={m.admin_cms_page_edit_slug_placeholder()}
                   class="w-full flex-1 px-3.5 py-2.5 border border-gray-200 rounded-r-xl text-sm
                          text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                          focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
          </div>
        </div>

        <!-- Content -->
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_cms_page_edit_label_content()} <span class="normal-case font-normal text-gray-400">{m.admin_cms_page_edit_content_markdown_hint()}</span>
          </label>
          <ShortcodeToolbar bind:value={content} textarea={contentTextarea} />
          <div class="{preview ? 'grid grid-cols-1 lg:grid-cols-2 gap-4' : ''}">
            <textarea name="content" bind:value={content} bind:this={contentTextarea} rows="16"
                      placeholder={m.admin_cms_page_edit_content_placeholder()}
                      class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                             text-gray-900 placeholder-gray-400 font-mono leading-relaxed
                             focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                             transition resize-y"></textarea>
            {#if preview}
              <div class="rounded-xl border border-gray-100 bg-gray-50 px-4 py-3 prose prose-sm max-w-none overflow-y-auto"
                   style="max-height: 480px">
                <MarkdownContent content={content || m.admin_cms_post_edit_preview_no_content()} placeholderMode />
              </div>
            {/if}
          </div>
        </div>
      </div>
    </div>

    <!-- SEO card -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-4 border-b border-gray-50">
        <h3 class="text-sm font-semibold text-gray-700">{m.admin_cms_page_edit_section_seo()}</h3>
      </div>
      <div class="px-6 py-5 space-y-4">
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_cms_page_edit_label_meta_title()}
          </label>
          <input type="text" name="meta_title" bind:value={metaTitle}
                 placeholder={m.admin_cms_page_edit_meta_title_placeholder()}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_cms_page_edit_label_meta_desc()}
          </label>
          <textarea name="meta_desc" bind:value={metaDesc} rows="2"
                    placeholder={m.admin_cms_page_edit_meta_desc_placeholder()}
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
          {isPublished ? m.admin_cms_pages_status_published() : m.admin_cms_pages_status_draft()}
        </span>
      </label>
      <div class="sm:ml-auto flex gap-3">
        <a href="/admin/cms/pages"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          {m.admin_cms_page_edit_cancel()}
        </a>
        <SaveButton loading={saving}
                class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                       text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
          {isNew ? m.admin_cms_page_edit_create() : m.admin_cms_page_edit_save()}
        </SaveButton>
      </div>
    </div>
  </form>
</div>
