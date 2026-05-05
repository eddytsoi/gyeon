<script lang="ts">
  import type { MediaFile } from '$lib/api/admin';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    files: MediaFile[];
    value: string;
    onChange: (url: string) => void;
    accept?: 'image' | 'all';
    label?: string;
    description?: string;
    previewClass?: string;
  }

  let {
    files,
    value,
    onChange,
    accept = 'image',
    label,
    description,
    previewClass = 'w-16 h-16'
  }: Props = $props();

  const filtered = $derived(
    accept === 'image'
      ? files.filter((f) => f.mime_type.startsWith('image/'))
      : files
  );

  let open = $state(false);

  function selectUrl(url: string) {
    onChange(url);
    open = false;
  }

  function clear() {
    onChange('');
  }

  function onKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') open = false;
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="flex items-center gap-3">
  <div class="rounded-xl border border-gray-200 bg-gray-50 flex items-center justify-center overflow-hidden {previewClass}">
    {#if value}
      <img src={value} alt="" class="w-full h-full object-contain" />
    {:else}
      <svg class="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z" />
      </svg>
    {/if}
  </div>
  <div class="flex flex-col gap-1.5 min-w-0 flex-1">
    {#if label}
      <p class="text-sm font-semibold text-gray-900">{label}</p>
    {/if}
    {#if description}
      <p class="text-xs text-gray-400">{description}</p>
    {/if}
    <div class="flex items-center gap-2">
      <button type="button"
              onclick={() => (open = true)}
              class="px-3 py-1.5 rounded-lg border border-gray-200 text-xs font-medium text-gray-700 hover:bg-gray-50">
        {m.admin_media_picker_choose()}
      </button>
      {#if value}
        <button type="button"
                onclick={clear}
                class="px-3 py-1.5 rounded-lg text-xs font-medium text-gray-500 hover:text-red-600">
          {m.admin_media_picker_clear()}
        </button>
      {/if}
    </div>
  </div>
</div>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => (open = false)} role="button" tabindex="-1"
         aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-3xl max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between p-5 border-b border-gray-100">
        <h3 class="text-base font-semibold text-gray-900">{m.admin_media_picker_modal_title()}</h3>
        <button type="button"
                onclick={() => (open = false)}
                class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100"
                aria-label={m.admin_modal_close()}>
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="overflow-y-auto p-5">
        {#if filtered.length === 0}
          <p class="text-sm text-gray-500 text-center py-8">{m.admin_media_picker_empty()}</p>
        {:else}
          <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-3">
            {#each filtered as f (f.id)}
              <button type="button"
                      onclick={() => selectUrl(f.url)}
                      class="group relative aspect-square rounded-xl overflow-hidden border-2 transition-colors
                             {value === f.url ? 'border-gray-900' : 'border-transparent hover:border-gray-300'}"
                      title={f.original_name}>
                <img src={f.thumbnail_url ?? f.url} alt={f.original_name}
                     class="w-full h-full object-cover bg-gray-50" />
                <div class="absolute inset-x-0 bottom-0 px-1.5 py-1 bg-gradient-to-t from-black/60 to-transparent">
                  <p class="text-[10px] text-white truncate text-left">{f.original_name}</p>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
