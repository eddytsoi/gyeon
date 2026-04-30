<script lang="ts">
  import { enhance } from '$app/forms';
  import { invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData } from './$types';
  import type { Category } from '$lib/types';
  import { adminReorderCategories } from '$lib/api/admin';
  import { showResult, notify } from '$lib/stores/notifications.svelte';
  import { spotlight } from '$lib/actions/spotlight';

  let { data }: { data: PageData } = $props();

  // Local list mirrors data.categories; we reorder this instantly on drop and
  // resync whenever the server data changes.
  let items = $state<Category[]>([]);
  $effect(() => {
    items = [...data.categories].sort((a, b) => a.sort_order - b.sort_order);
  });

  let showForm = $state(false);
  let editing = $state<Category | null>(null);
  let deleteTarget = $state<Category | null>(null);

  let fName = $state('');
  let fSlug = $state('');

  function openNew() {
    editing = null;
    fName = '';
    fSlug = '';
    showForm = true;
  }

  function openEdit(cat: Category) {
    editing = cat;
    fName = cat.name;
    fSlug = cat.slug;
    showForm = true;
  }

  function onNameInput() {
    if (!editing) {
      fSlug = fName
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '');
    }
  }

  // ── Drag & Drop ─────────────────────────────────────────────────
  let draggingId = $state<string | null>(null);
  let dropTargetId = $state<string | null>(null);

  function onDragStart(e: DragEvent, id: string) {
    draggingId = id;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', id);
    }
  }
  function onDragOver(e: DragEvent, id: string) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    if (draggingId && id !== draggingId) dropTargetId = id;
  }
  function onDragLeave(id: string) {
    if (dropTargetId === id) dropTargetId = null;
  }
  async function onDrop(e: DragEvent, targetId: string) {
    e.preventDefault();
    const sourceId = draggingId;
    draggingId = null;
    dropTargetId = null;
    if (!sourceId || sourceId === targetId) return;

    const src = items.findIndex((c) => c.id === sourceId);
    const dst = items.findIndex((c) => c.id === targetId);
    if (src === -1 || dst === -1) return;

    // Move src into dst's slot, preserving direction
    const next = [...items];
    const [moved] = next.splice(src, 1);
    next.splice(dst, 0, moved);
    // Optimistic local update + sort_order rewrite for stable display
    items = next.map((c, i) => ({ ...c, sort_order: i + 1 }));

    const token = page.data.token ?? '';
    try {
      await adminReorderCategories(token, items.map((c) => c.id));
      await invalidateAll();
    } catch {
      notify.error('Reorder failed', 'Categories were not saved. Refresh to retry.');
      await invalidateAll();
    }
  }
  function onDragEnd() {
    draggingId = null;
    dropTargetId = null;
  }
</script>

<div class="max-w-2xl mx-auto space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">Product Categories</h2>
      <p class="text-sm text-gray-500 mt-0.5">{items.length} categor{items.length !== 1 ? 'ies' : 'y'} · drag to reorder</p>
    </div>
    <button onclick={openNew}
            class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
                   text-sm font-medium hover:bg-gray-700 transition-colors">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/>
      </svg>
      New Category
    </button>
  </div>

  <!-- List -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    {#if items.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <div class="w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mb-3">
          <svg class="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M9.568 3H5.25A2.25 2.25 0 0 0 3 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 0 0 5.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 0 0 9.568 3Z M6 6h.008v.008H6V6Z"/>
          </svg>
        </div>
        <p class="text-sm font-medium text-gray-400">No categories yet</p>
        <button onclick={openNew} class="mt-3 text-sm text-gray-900 underline underline-offset-2">
          Create your first category
        </button>
      </div>
    {:else}
      <ul class="divide-y divide-gray-50">
        {#each items as cat (cat.id)}
          <li class="js-row flex items-center gap-4 px-5 py-4 transition-colors
                     {draggingId === cat.id ? 'opacity-40' : ''}
                     {dropTargetId === cat.id ? 'bg-gray-50' : ''}"
              ondragover={(e) => onDragOver(e, cat.id)}
              ondragleave={() => onDragLeave(cat.id)}
              ondrop={(e) => onDrop(e, cat.id)}>
            <!-- Drag handle -->
            <button type="button"
                    draggable="true"
                    ondragstart={(e) => onDragStart(e, cat.id)}
                    ondragend={onDragEnd}
                    aria-label="Drag to reorder"
                    class="cursor-grab active:cursor-grabbing p-1 -m-1 text-gray-500
                           hover:text-gray-800 transition-colors flex-shrink-0">
              <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                <path d="M7 4a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm6-10a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Z" />
              </svg>
            </button>

            <div class="flex-1 min-w-0">
              <p class="text-sm font-semibold text-gray-900">{cat.name}</p>
              <p class="text-xs text-gray-400 font-mono mt-0.5">{cat.slug}</p>
            </div>

            <div class="flex items-center gap-1.5">
              <button onclick={() => openEdit(cat)}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                </svg>
              </button>
              <button onclick={() => deleteTarget = cat}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                </svg>
              </button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

<!-- Create / Edit modal -->
{#if showForm}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showForm = false} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-md p-6">
      <h3 class="text-base font-bold text-gray-900 mb-5">
        {editing ? 'Edit Category' : 'New Category'}
      </h3>

      <form method="POST" action={editing ? '?/update' : '?/create'}
            use:enhance={() => {
              const wasEditing = !!editing;
              const catName = fName;
              return async ({ result, update }) => {
                showResult(result,
                  wasEditing ? `Category '${catName}' saved` : `Category '${catName}' created`,
                  wasEditing ? `Failed to save category '${catName}'` : `Failed to create category '${catName}'`);
                await update();
                if (result.type === 'success') showForm = false;
              };
            }}
            class="space-y-4">
        {#if editing}
          <input type="hidden" name="id" value={editing.id} />
        {/if}

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Name</label>
          <input type="text" name="name" bind:value={fName} oninput={onNameInput}
                 required placeholder="e.g. Accessories"
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Slug</label>
          <input type="text" name="slug" bind:value={fSlug}
                 required placeholder="accessories"
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                        focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div class="flex gap-3 pt-2">
          <button type="button" onclick={() => showForm = false}
                  class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                         text-gray-700 hover:bg-gray-50 transition-colors">
            Cancel
          </button>
          <button type="submit"
                  class="flex-1 px-4 py-2.5 rounded-xl bg-gray-900 text-white text-sm font-medium
                         hover:bg-gray-700 transition-colors">
            {editing ? 'Save Changes' : 'Create'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Delete confirm -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteTarget = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">Delete category?</h3>
      <p class="text-sm text-gray-500 mb-5">
        "<span class="font-medium text-gray-700">{deleteTarget.name}</span>" will be removed.
        Products using this category will be unassigned.
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </button>
        <form method="POST" action="?/delete" class="flex-1"
              use:enhance={() => {
                const catName = deleteTarget?.name ?? '';
                return async ({ result, update }) => {
                  showResult(result, `Category '${catName}' deleted`, `Failed to delete category '${catName}'`);
                  await update();
                  deleteTarget = null;
                };
              }}>
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
