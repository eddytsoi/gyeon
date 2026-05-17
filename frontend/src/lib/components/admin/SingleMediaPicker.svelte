<script lang="ts">
  import { adminUploadMedia, type MediaFile } from '$lib/api/admin';
  import * as m from '$lib/paraglide/messages';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';

  interface Props {
    files: MediaFile[];
    value: string | null;
    onChange: (mediaFileId: string | null, file: MediaFile | null) => void;
    onUpload?: (file: MediaFile) => void;
    token: string | null;
    label?: string;
    description?: string;
    name?: string;
    form?: string;
    previewClass?: string;
  }

  let {
    files,
    value,
    onChange,
    onUpload,
    token,
    label,
    description,
    name,
    form,
    previewClass = 'aspect-[4/3] w-full'
  }: Props = $props();

  const current = $derived(value ? files.find((f) => f.id === value) ?? null : null);
  const previewUrl = $derived(current ? (current.webp_url ?? current.url) : null);

  let open = $state(false);
  let tab = $state<'library' | 'upload'>('library');
  let uploading = $state(false);
  let uploadError = $state<string | null>(null);
  let progress = $state(0);

  const imageFiles = $derived(files.filter((f) => f.mime_type.startsWith('image/')));

  function select(file: MediaFile) {
    onChange(file.id, file);
    open = false;
  }

  function clear() {
    onChange(null, null);
  }

  async function onFile(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file || !token) return;
    uploadError = null;
    uploading = true;
    progress = 0;
    try {
      const mf = await adminUploadMedia(token, file, (pct) => (progress = pct));
      onUpload?.(mf);
      select(mf);
    } catch (err) {
      uploadError = err instanceof Error ? err.message : String(err);
    } finally {
      uploading = false;
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') open = false;
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="flex flex-col gap-2">
  {#if label}
    <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{label}</label>
  {/if}

  <button
    type="button"
    onclick={() => (open = true)}
    class="group relative {previewClass} overflow-hidden rounded-xl border-2 border-dashed
           border-gray-200 bg-gray-50 hover:border-gray-400 hover:bg-gray-100 transition-colors
           flex items-center justify-center"
  >
    {#if previewUrl}
      <ResponsiveImage
        src={previewUrl}
        alt={current?.original_name ?? ''}
        widths={[320, 640]}
        sizes="320px"
        class="w-full h-full object-cover"
      />
      <span
        class="absolute inset-x-0 bottom-0 p-2 text-[10px] text-white truncate
               bg-gradient-to-t from-black/60 to-transparent text-left"
      >{current?.original_name}</span>
    {:else}
      <div class="flex flex-col items-center gap-1 text-gray-400 py-6">
        <svg class="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z"/>
        </svg>
        <span class="text-xs font-medium">{m.admin_single_media_picker_choose()}</span>
      </div>
    {/if}
  </button>

  {#if description}
    <p class="text-xs text-gray-400">{description}</p>
  {/if}

  {#if value}
    <button
      type="button"
      onclick={clear}
      class="self-start text-xs font-medium text-gray-500 hover:text-red-600"
    >{m.admin_single_media_picker_clear()}</button>
  {/if}

  {#if name}
    <input type="hidden" {name} {form} value={value ?? ''} />
  {/if}
</div>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div
      class="absolute inset-0 bg-black/40 backdrop-blur-sm"
      onclick={() => (open = false)}
      role="button"
      tabindex="-1"
      aria-label={m.admin_modal_close()}
    ></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-3xl max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between p-5 border-b border-gray-100">
        <h3 class="text-base font-semibold text-gray-900">
          {label ?? m.admin_single_media_picker_modal_title()}
        </h3>
        <button
          type="button"
          onclick={() => (open = false)}
          class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100"
          aria-label={m.admin_modal_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      <div class="flex gap-1 px-5 pt-3 border-b border-gray-100">
        <button
          type="button"
          onclick={() => (tab = 'library')}
          class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                 {tab === 'library'
                   ? 'border-gray-900 text-gray-900'
                   : 'border-transparent text-gray-400 hover:text-gray-700'}"
        >{m.admin_product_edit_add_media_tab_library()}</button>
        <button
          type="button"
          onclick={() => (tab = 'upload')}
          class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                 {tab === 'upload'
                   ? 'border-gray-900 text-gray-900'
                   : 'border-transparent text-gray-400 hover:text-gray-700'}"
        >{m.admin_product_edit_add_media_tab_upload()}</button>
      </div>

      <div class="overflow-y-auto p-5">
        {#if tab === 'library'}
          {#if imageFiles.length === 0}
            <p class="text-sm text-gray-500 text-center py-8">{m.admin_media_picker_empty()}</p>
          {:else}
            <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-3">
              {#each imageFiles as f (f.id)}
                <button
                  type="button"
                  onclick={() => select(f)}
                  class="group relative aspect-square rounded-xl overflow-hidden border-2 transition-colors
                         {value === f.id ? 'border-gray-900' : 'border-transparent hover:border-gray-300'}"
                  title={f.original_name}
                >
                  <ResponsiveImage
                    src={f.webp_url ?? f.url}
                    alt={f.original_name}
                    widths={[160, 320]}
                    sizes="120px"
                    class="w-full h-full object-cover bg-gray-50"
                  />
                  <div class="absolute inset-x-0 bottom-0 px-1.5 py-1 bg-gradient-to-t from-black/60 to-transparent">
                    <p class="text-[10px] text-white truncate text-left">{f.original_name}</p>
                  </div>
                </button>
              {/each}
            </div>
          {/if}
        {:else}
          <label
            class="flex flex-col items-center justify-center gap-2 px-6 py-10 rounded-2xl
                   border-2 border-dashed border-gray-200 hover:border-gray-400 hover:bg-gray-50
                   cursor-pointer text-center transition-colors"
          >
            <svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 7.5m0 0L7.5 12M12 7.5v9"/>
            </svg>
            <p class="text-sm font-medium text-gray-700">{m.admin_product_edit_add_media_dropzone()}</p>
            <p class="text-xs text-gray-400">{m.admin_product_edit_add_media_accepted()}</p>
            <input type="file" class="sr-only" accept="image/*" onchange={onFile} disabled={uploading} />
          </label>

          {#if uploading}
            <div class="mt-4 flex flex-col gap-2">
              <div class="h-1 w-full rounded-full bg-gray-200 overflow-hidden">
                <div
                  class="h-full bg-gray-900 transition-[width] duration-150"
                  style="width: {progress}%"
                ></div>
              </div>
              <p class="text-xs text-gray-500 text-center">{progress}%</p>
            </div>
          {/if}

          {#if uploadError}
            <p class="mt-3 text-xs text-red-600">{uploadError}</p>
          {/if}
        {/if}
      </div>
    </div>
  </div>
{/if}
