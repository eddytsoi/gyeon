<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import { showResult } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const file = $derived(data.file);
  const isLink = $derived(file.mime_type === 'link');
  const isVideo = $derived(
    file.mime_type.startsWith('video/') ||
    (isLink && /\.(mp4|webm|mov|avi|mkv)(\?|#|$)/i.test(file.url))
  );
  const isImage = $derived(file.mime_type.startsWith('image/'));

  let linkImageFailed = $state(false);
  let showDeleteModal = $state(false);
  let saving = $state(false);
  let deleting = $state(false);

  function formatBytes(n: number) {
    if (n === 0) return '—';
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  }

  function formatDate(s: string) {
    return new Date(s).toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  function typeLabel(mimeType: string) {
    if (mimeType === 'link') return m.admin_media_edit_type_link();
    if (mimeType.startsWith('image/')) return m.admin_media_edit_type_image();
    if (mimeType.startsWith('video/')) return m.admin_media_edit_type_video();
    return mimeType;
  }
</script>

<svelte:head><title>{file.original_name} — Media — Gyeon Admin</title></svelte:head>

<!-- Delete confirmation modal -->
{#if showDeleteModal}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/50 backdrop-blur-sm"
    onclick={(e) => { if (e.target === e.currentTarget && !deleting) showDeleteModal = false; }}
  >
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm mx-4 p-6">
      <h3 class="text-base font-semibold text-gray-900 mb-2">{m.admin_media_edit_delete_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        <span class="font-medium text-gray-800">{file.original_name}</span>{m.admin_media_edit_delete_body_post()}
        {#if file.refs && file.refs.length > 0}
          <span class="text-red-600">{file.refs.length === 1 ? m.admin_media_edit_delete_used_one({ count: file.refs.length }) : m.admin_media_edit_delete_used_many({ count: file.refs.length })}</span>
        {/if}
      </p>
      <div class="flex justify-end gap-2">
        <button
          onclick={() => (showDeleteModal = false)}
          disabled={deleting}
          class="px-4 py-2 rounded-xl text-sm font-medium text-gray-600 hover:bg-gray-100 transition-colors disabled:opacity-50"
        >
          {m.admin_media_edit_cancel()}
        </button>
        <form
          method="POST"
          action="?/delete"
          use:enhance={() => {
            if (deleting) return;
            deleting = true;
            const targetName = file.original_name;
            return async ({ result, update }) => {
              showResult(
                result,
                m.admin_media_deleted_success({ name: targetName }),
                m.admin_media_deleted_failure()
              );
              await update();
              deleting = false;
            };
          }}
        >
          <SaveButton
            loading={deleting}
            class="inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-xl bg-red-600 text-white text-sm font-medium hover:bg-red-700 transition-colors disabled:opacity-50"
          >
            {m.admin_media_edit_delete()}
          </SaveButton>
        </form>
      </div>
    </div>
  </div>
{/if}

<div class="space-y-6">

  <!-- Header -->
  <div class="flex items-center gap-3">
    <a
      href="/admin/media"
      class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
      title={m.admin_media_edit_back()}
    >
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900 truncate">{file.original_name}</h2>
  </div>

  <!-- Two-column layout -->
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">

    <!-- Left: preview -->
    <div class="rounded-2xl overflow-hidden bg-gray-100 aspect-square flex items-center justify-center">
      {#if isVideo}
        <video
          src={file.url}
          controls
          class="w-full h-full object-contain bg-black"
        ></video>
      {:else if isImage}
        <img
          src={file.url}
          alt={file.original_name}
          class="w-full h-full object-contain"
        />
      {:else if isLink}
        {#if !linkImageFailed}
          <img
            src={file.url}
            alt={file.original_name}
            class="w-full h-full object-contain"
            onerror={() => { linkImageFailed = true; }}
          />
        {:else}
          <div class="flex flex-col items-center gap-3 text-center px-6">
            <div class="w-16 h-16 rounded-2xl bg-gray-200 flex items-center justify-center">
              <svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
              </svg>
            </div>
            <a
              href={file.url}
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm text-blue-600 hover:underline break-all"
            >
              {file.url}
            </a>
          </div>
        {/if}
      {:else}
        <div class="flex flex-col items-center gap-3 text-center px-6">
          <div class="w-16 h-16 rounded-2xl bg-gray-200 flex items-center justify-center">
            <svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
            </svg>
          </div>
          <p class="text-sm font-medium text-gray-500 break-all">{file.filename}</p>
        </div>
      {/if}
    </div>

    <!-- Right: form -->
    <div class="space-y-5">

      <form
        method="POST"
        action="?/save"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          return async ({ result, update }) => {
            showResult(
              result,
              m.admin_media_edit_save_success(),
              m.admin_media_edit_save_failure()
            );
            await update({ reset: false });
            saving = false;
          };
        }}
        class="space-y-4"
      >
        <!-- Name -->
        <div>
          <label class="block text-xs font-medium text-gray-700 mb-1" for="original_name">{m.admin_media_edit_label_name()}</label>
          <input
            id="original_name"
            name="original_name"
            type="text"
            value={file.original_name}
            required
            class="w-full rounded-xl border border-gray-200 px-3 py-2 text-sm text-gray-900
                   placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>

        <!-- URL -->
        <div>
          <label class="block text-xs font-medium text-gray-700 mb-1" for="url">
            {isLink ? m.admin_media_edit_label_url() : m.admin_media_edit_label_file_path()}
          </label>
          {#if isLink}
            <input
              id="url"
              name="url"
              type="url"
              value={file.url}
              required
              class="w-full rounded-xl border border-gray-200 px-3 py-2 text-sm text-gray-900
                     placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900"
            />
          {:else}
            <input
              id="url"
              type="text"
              value={file.url}
              readonly
              class="w-full rounded-xl border border-gray-100 bg-gray-50 px-3 py-2 text-sm text-gray-500 cursor-default select-all"
            />
          {/if}
        </div>

        <!-- Read-only meta row -->
        <div class="grid grid-cols-2 gap-4">
          <div>
            <p class="text-xs font-medium text-gray-500 mb-1">{m.admin_media_edit_label_type()}</p>
            <span class="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium
              {isLink ? 'bg-purple-50 text-purple-700' : isVideo ? 'bg-blue-50 text-blue-700' : 'bg-green-50 text-green-700'}">
              {typeLabel(file.mime_type)}
            </span>
          </div>
          <div>
            <p class="text-xs font-medium text-gray-500 mb-1">{m.admin_media_edit_label_size()}</p>
            <p class="text-sm text-gray-700">{formatBytes(file.size_bytes)}</p>
          </div>
        </div>

        <div>
          <p class="text-xs font-medium text-gray-500 mb-1">{m.admin_media_edit_label_uploaded_at()}</p>
          <p class="text-sm text-gray-700">{formatDate(file.created_at)}</p>
        </div>

        {#if file.refs && file.refs.length > 0}
          <div>
            <p class="text-xs font-medium text-gray-500 mb-1.5">{m.admin_media_edit_label_used_by()}</p>
            <div class="flex flex-wrap gap-1.5">
              {#each file.refs as ref}
                <span
                  class="inline-flex items-center px-2 py-1 rounded-lg text-xs font-medium
                    {ref.type === 'product' ? 'bg-blue-50 text-blue-700' : 'bg-purple-50 text-purple-700'}"
                  title={ref.name}
                >
                  {m.admin_media_edit_used_by_format({ type: ref.type === 'product' ? m.admin_media_ref_product() : m.admin_media_ref_post(), name: ref.name })}
                </span>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Action bar -->
        <div class="flex items-center gap-2 pt-2 border-t border-gray-100">
          <SaveButton
            loading={saving}
            class="inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50"
          >
            {m.admin_media_edit_save()}
          </SaveButton>
          <a
            href="/admin/media"
            class="px-4 py-2 rounded-xl border border-gray-200 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors"
          >
            {m.admin_media_edit_cancel()}
          </a>
          <button
            type="button"
            onclick={() => (showDeleteModal = true)}
            class="ml-auto px-4 py-2 rounded-xl border border-red-200 text-sm font-medium text-red-600 hover:bg-red-50 transition-colors"
          >
            {m.admin_media_edit_delete()}
          </button>
        </div>
      </form>

    </div>
  </div>
</div>
