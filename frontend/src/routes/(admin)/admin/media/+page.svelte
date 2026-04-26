<script lang="ts">
  import type { PageData } from './$types';
  import type { MediaFile } from '$lib/api/admin';
  import { adminUploadMedia, adminDeleteMedia, adminAddMediaLink } from '$lib/api/admin';

  let { data }: { data: PageData } = $props();

  let media = $state<MediaFile[]>(data.media);
  let filter = $state<'all' | 'image' | 'video' | 'link'>('all');
  let dragging = $state(false);
  let dragCounter = $state(0);
  let uploading = $state<Map<string, number>>(new Map());
  let deleteTarget = $state<MediaFile | null>(null);
  let uploadErrors = $state<string[]>([]);
  let failedImages = $state<Set<string>>(new Set());

  // ── Add Link modal ───────────────────────────────────────────────────────────
  let linkModalOpen = $state(false);
  let linkUrl = $state('');
  let linkName = $state('');
  let linkSaving = $state(false);
  let linkError = $state('');

  function openLinkModal() {
    linkUrl = '';
    linkName = '';
    linkError = '';
    linkModalOpen = true;
  }

  function closeLinkModal() {
    if (linkSaving) return;
    linkModalOpen = false;
  }

  async function saveLink() {
    const url = linkUrl.trim();
    if (!url) { linkError = 'URL is required'; return; }
    linkSaving = true;
    linkError = '';
    try {
      const added = await adminAddMediaLink(data.token, url, linkName.trim());
      media = [added, ...media];
      linkModalOpen = false;
    } catch {
      linkError = 'Failed to save link. Please check the URL and try again.';
    } finally {
      linkSaving = false;
    }
  }

  // ── Helpers ──────────────────────────────────────────────────────────────────
  const VIDEO_EXTS = /\.(mp4|webm|mov|avi|mkv)(\?|#|$)/i;
  const IMAGE_EXTS = /\.(jpe?g|png|gif|webp|svg|avif|heic|bmp)(\?|#|$)/i;

  function isVideo(f: MediaFile) {
    return f.mime_type.startsWith('video/') || (f.mime_type === 'link' && VIDEO_EXTS.test(f.url));
  }
  function isImage(f: MediaFile) {
    return f.mime_type.startsWith('image/') || (f.mime_type === 'link' && IMAGE_EXTS.test(f.url));
  }
  function isLink(f: MediaFile) {
    return f.mime_type === 'link';
  }

  const filtered = $derived(
    filter === 'all'
      ? media
      : filter === 'image'
        ? media.filter((f) => isImage(f))
        : filter === 'video'
          ? media.filter((f) => isVideo(f))
          : media.filter((f) => isLink(f))
  );

  function formatBytes(n: number) {
    if (n === 0) return '—';
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  }

  // ── Drag & drop ──────────────────────────────────────────────────────────────
  function onDragEnter(e: DragEvent) {
    e.preventDefault();
    dragCounter++;
    dragging = true;
  }

  function onDragLeave(e: DragEvent) {
    e.preventDefault();
    dragCounter--;
    if (dragCounter === 0) dragging = false;
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault();
  }

  async function onDrop(e: DragEvent) {
    e.preventDefault();
    dragCounter = 0;
    dragging = false;
    const files = Array.from(e.dataTransfer?.files ?? []);
    await uploadFiles(files);
  }

  // ── Upload ───────────────────────────────────────────────────────────────────
  function openPicker() {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.accept = 'image/*,video/mp4,video/webm,video/quicktime';
    input.onchange = () => uploadFiles(Array.from(input.files ?? []));
    input.click();
  }

  async function uploadFiles(files: File[]) {
    uploadErrors = [];
    for (const file of files) {
      const placeholderId = crypto.randomUUID();
      uploading = new Map(uploading.set(placeholderId, 0));

      const tick = setInterval(() => {
        const cur = uploading.get(placeholderId) ?? 0;
        if (cur < 85) uploading = new Map(uploading.set(placeholderId, cur + 12));
      }, 180);

      try {
        const uploaded = await adminUploadMedia(data.token, file);
        clearInterval(tick);
        uploading = new Map(uploading.set(placeholderId, 100));
        await new Promise((r) => setTimeout(r, 250));
        media = [uploaded, ...media];
      } catch (err) {
        clearInterval(tick);
        const msg = err instanceof Error ? err.message : 'Upload failed';
        uploadErrors = [...uploadErrors, `${file.name}: ${msg}`];
      } finally {
        uploading.delete(placeholderId);
        uploading = new Map(uploading);
      }
    }
  }

  // ── Delete ───────────────────────────────────────────────────────────────────
  async function doDelete(file: MediaFile) {
    deleteTarget = null;
    try {
      await adminDeleteMedia(data.token, file.id);
      media = media.filter((f) => f.id !== file.id);
    } catch {
      // silently ignore
    }
  }
</script>

<svelte:head><title>Media — Gyeon Admin</title></svelte:head>

<svelte:window
  ondragenter={onDragEnter}
  ondragleave={onDragLeave}
  ondragover={onDragOver}
  ondrop={onDrop}
/>

<!-- Full-page drop overlay -->
{#if dragging}
  <div class="fixed inset-0 z-50 pointer-events-none flex items-center justify-center bg-gray-900/60 backdrop-blur-sm">
    <div class="border-2 border-dashed border-white/60 rounded-3xl px-16 py-10 text-center">
      <svg class="w-10 h-10 text-white mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
      </svg>
      <p class="text-white font-semibold text-lg">Drop to upload</p>
      <p class="text-white/60 text-sm mt-1">Images ≤ 1 MB · Videos ≤ 10 MB</p>
    </div>
  </div>
{/if}

<!-- Add Link modal -->
{#if linkModalOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/50 backdrop-blur-sm"
    onclick={(e) => { if (e.target === e.currentTarget) closeLinkModal(); }}
  >
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 p-6">
      <h3 class="text-base font-semibold text-gray-900 mb-4">Add Link</h3>

      <div class="space-y-3">
        <div>
          <label class="block text-xs font-medium text-gray-700 mb-1" for="link-url">URL</label>
          <input
            id="link-url"
            type="url"
            bind:value={linkUrl}
            placeholder="https://example.com/image.jpg"
            onkeydown={(e) => e.key === 'Enter' && saveLink()}
            class="w-full rounded-xl border border-gray-200 px-3 py-2 text-sm text-gray-900
                   placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-700 mb-1" for="link-name">Label <span class="text-gray-400 font-normal">(optional)</span></label>
          <input
            id="link-name"
            type="text"
            bind:value={linkName}
            placeholder="My image"
            class="w-full rounded-xl border border-gray-200 px-3 py-2 text-sm text-gray-900
                   placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        {#if linkError}
          <p class="text-xs text-red-600">{linkError}</p>
        {/if}
      </div>

      <div class="flex justify-end gap-2 mt-5">
        <button
          onclick={closeLinkModal}
          disabled={linkSaving}
          class="px-4 py-2 rounded-xl text-sm font-medium text-gray-600 hover:bg-gray-100 transition-colors disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          onclick={saveLink}
          disabled={linkSaving}
          class="px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50"
        >
          {linkSaving ? 'Saving…' : 'Save'}
        </button>
      </div>
    </div>
  </div>
{/if}

<div class="space-y-5">

  <!-- Toolbar -->
  <div class="flex flex-wrap items-center gap-3">
    <div class="flex items-baseline gap-2">
      <h2 class="text-xl font-bold text-gray-900">Media Library</h2>
      <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-gray-100 text-xs font-medium text-gray-500">
        {media.length}
      </span>
    </div>

    <!-- Filter tabs -->
    <div class="flex items-center gap-1 bg-gray-100 rounded-xl p-1">
      {#each (['all', 'image', 'video', 'link'] as const) as tab}
        <button
          onclick={() => (filter = tab)}
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-all {filter === tab
            ? 'bg-white text-gray-900 shadow-sm'
            : 'text-gray-500 hover:text-gray-700'}"
        >
          {tab === 'all' ? 'All' : tab === 'image' ? 'Images' : tab === 'video' ? 'Videos' : 'Links'}
        </button>
      {/each}
    </div>

    <div class="ml-auto flex items-center gap-2">
      <!-- Add Link -->
      <button
        onclick={openLinkModal}
        class="inline-flex items-center gap-2 px-4 py-2 rounded-xl border border-gray-200 bg-white text-gray-700 text-sm font-medium hover:bg-gray-50 transition-colors"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
        </svg>
        Add Link
      </button>
      <!-- Upload -->
      <button
        onclick={openPicker}
        class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
        </svg>
        Upload
      </button>
    </div>
  </div>

  <!-- Upload errors -->
  {#if uploadErrors.length > 0}
    <div class="rounded-xl bg-red-50 border border-red-100 p-3 space-y-1">
      {#each uploadErrors as err}
        <p class="text-xs text-red-600">{err}</p>
      {/each}
    </div>
  {/if}

  <!-- Grid -->
  {#if filtered.length === 0 && uploading.size === 0}
    <div class="flex flex-col items-center justify-center py-24 text-center">
      <div class="w-16 h-16 rounded-2xl bg-gray-50 flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 15.75l5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
        </svg>
      </div>
      <p class="text-sm font-medium text-gray-400">No media files yet.</p>
      <p class="text-xs text-gray-300 mt-1">Drop files anywhere, click Upload, or Add Link to get started.</p>
    </div>
  {:else}
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3">

      <!-- Uploading placeholder tiles -->
      {#each [...uploading.entries()] as [placeholderId, pct]}
        <div class="flex flex-col gap-1.5">
          <div class="aspect-square rounded-xl overflow-hidden bg-gray-100 relative animate-pulse">
            <div class="absolute inset-0 flex items-center justify-center">
              <svg class="w-10 h-10 -rotate-90" viewBox="0 0 36 36">
                <circle cx="18" cy="18" r="14" fill="none" stroke="#e5e7eb" stroke-width="3" />
                <circle
                  cx="18" cy="18" r="14" fill="none"
                  stroke="#111827" stroke-width="3"
                  stroke-dasharray="87.96"
                  stroke-dashoffset="{87.96 - 87.96 * pct / 100}"
                  stroke-linecap="round"
                  class="transition-all duration-200"
                />
              </svg>
            </div>
          </div>
        </div>
      {/each}

      <!-- Media tiles -->
      {#each filtered as file (file.id)}
        <div class="flex flex-col gap-1.5">
          <div class="aspect-square rounded-xl overflow-hidden relative group bg-gray-100">

            {#if isVideo(file)}
              <!-- Video: use native element to render first-frame thumbnail -->
              <video
                src={file.url}
                preload="metadata"
                muted
                playsinline
                class="w-full h-full object-cover"
              ></video>
              <!-- Play icon overlay (always visible, fades on hover) -->
              <div class="absolute inset-0 flex items-center justify-center pointer-events-none group-hover:opacity-0 transition-opacity">
                <div class="w-10 h-10 rounded-full bg-black/40 flex items-center justify-center backdrop-blur-sm">
                  <svg class="w-5 h-5 text-white ml-0.5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                </div>
              </div>

            {:else if isLink(file)}
              <!-- Link: always try to render as image; fallback placeholder on load error -->
              {#if failedImages.has(file.id)}
                <div class="w-full h-full flex flex-col items-center justify-center gap-2 bg-gray-50">
                  <svg class="w-8 h-8 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
                  </svg>
                  <span class="text-xs text-gray-400 font-medium px-2 text-center line-clamp-2 break-all">{file.original_name}</span>
                </div>
              {:else}
                <img
                  src={file.url}
                  alt={file.original_name}
                  class="w-full h-full object-cover"
                  loading="lazy"
                  onerror={() => { failedImages = new Set([...failedImages, file.id]); }}
                />
              {/if}
              <!-- "Link" badge -->
              <div class="absolute top-1.5 left-1.5 pointer-events-none">
                <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-gray-900/70 text-white text-xs font-medium backdrop-blur-sm">
                  <svg class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
                  </svg>
                  Link
                </span>
              </div>

            {:else}
              <!-- Image -->
              <img src={file.url} alt={file.original_name} class="w-full h-full object-cover" loading="lazy" />
            {/if}

            <!-- Hover overlay -->
            <div class="absolute inset-0 bg-gray-900/70 opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex flex-col justify-between p-2.5">
              <div>
                <p class="text-white text-xs font-medium leading-snug line-clamp-2 break-all">{file.original_name}</p>
                <p class="text-white/60 text-xs mt-0.5">{formatBytes(file.size_bytes)}</p>
              </div>
              <div class="flex items-center justify-end gap-1.5">
                <!-- Copy URL -->
                <button
                  title="Copy URL"
                  onclick={() => navigator.clipboard.writeText(file.url)}
                  class="p-1.5 rounded-lg bg-white/10 hover:bg-white/20 transition-colors text-white"
                >
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
                  </svg>
                </button>

                <!-- Delete -->
                <button
                  title="Delete"
                  onclick={() => (deleteTarget = file)}
                  class="p-1.5 rounded-lg bg-white/10 hover:bg-red-500/80 transition-colors text-white"
                >
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                  </svg>
                </button>
              </div>
            </div>

            <!-- Edit badge (always visible) -->
            <a
              href="/admin/media/{file.id}"
              title="Edit"
              class="absolute bottom-1.5 right-1.5 z-10 p-1.5 rounded-lg bg-gray-900/60 text-white hover:bg-gray-900/90 transition-colors backdrop-blur-sm"
            >
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z" />
              </svg>
            </a>
          </div>

          <!-- Entity ref badges -->
          {#if file.refs && file.refs.length > 0}
            <div class="flex flex-wrap gap-1">
              {#each file.refs as ref}
                <span
                  class="inline-flex items-center px-1.5 py-0.5 rounded-md text-xs font-medium truncate max-w-full {ref.type === 'product'
                    ? 'bg-blue-50 text-blue-700'
                    : 'bg-purple-50 text-purple-700'}"
                  title={ref.name}
                >
                  {ref.type === 'product' ? 'Product' : 'Post'}
                </span>
              {/each}
            </div>
          {/if}
        </div>
      {/each}

    </div>
  {/if}

</div>

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" onclick={() => (deleteTarget = null)}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">Delete media?</h3>
      <p class="text-sm text-gray-500 mb-5">
        "<span class="font-medium text-gray-700">{deleteTarget.original_name}</span>" will be permanently deleted.
      </p>
      <div class="flex gap-3">
        <button
          onclick={() => (deleteTarget = null)}
          class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
        >
          Cancel
        </button>
        <button
          onclick={() => { if (deleteTarget) doDelete(deleteTarget); }}
          class="flex-1 px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium hover:bg-red-600 transition-colors"
        >
          Delete
        </button>
      </div>
    </div>
  </div>
{/if}
