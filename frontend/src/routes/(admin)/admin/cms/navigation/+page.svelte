<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto, invalidateAll } from '$app/navigation';
  import type { PageData } from './$types';
  import type { NavItem } from '$lib/api/admin';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveIcon from '$lib/components/admin/SaveIcon.svelte';

  let { data }: { data: PageData } = $props();

  let showItemForm = $state(false);
  let editingItem = $state<NavItem | null>(null);
  let deleteTarget = $state<NavItem | null>(null);

  // Item form fields
  let fLabel = $state('');
  let fUrl = $state('');
  let fTarget = $state('_self');
  let fOrder = $state(0);

  function openAddItem() {
    editingItem = null;
    fLabel = '';
    fUrl = '';
    fTarget = '_self';
    fOrder = flatItems.length;
    showItemForm = true;
  }

  function openEditItem(item: NavItem) {
    editingItem = item;
    fLabel = item.label;
    fUrl = item.url;
    fTarget = item.target;
    fOrder = item.sort_order;
    showItemForm = true;
  }

  // Flatten nested items for the list display
  const flatItems = $derived.by(() => {
    const result: NavItem[] = [];
    function walk(items: NavItem[], depth = 0) {
      for (const item of items) {
        result.push({ ...item, _depth: depth } as NavItem & { _depth: number });
        if (item.children?.length) walk(item.children, depth + 1);
      }
    }
    walk(data.selected?.items ?? []);
    return result;
  });

  function switchMenu(id: string) {
    goto(`?menu=${id}`, { invalidateAll: true });
  }
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">Navigation</h2>
      <p class="text-sm text-gray-500 mt-0.5">Manage header and footer menus</p>
    </div>
    {#if data.selected}
      <button onclick={openAddItem}
              class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
                     text-sm font-medium hover:bg-gray-700 transition-colors">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/>
        </svg>
        Add Link
      </button>
    {/if}
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
    <!-- Menu selector -->
    <div class="lg:col-span-1">
      <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-50">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide">Menus</p>
        </div>
        <ul class="py-2">
          {#each data.menus as menu}
            <li>
              <button onclick={() => switchMenu(menu.id)}
                      class="w-full flex items-center gap-3 px-4 py-2.5 text-sm font-medium
                             transition-colors text-left
                             {data.selectedID === menu.id
                               ? 'bg-gray-900 text-white'
                               : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}">
                <svg class="w-4 h-4 flex-shrink-0 {data.selectedID === menu.id ? 'text-white' : 'text-gray-400'}"
                     fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12"/>
                </svg>
                <div class="min-w-0">
                  <p class="truncate">{menu.name}</p>
                  <p class="text-[10px] {data.selectedID === menu.id ? 'text-gray-400' : 'text-gray-400'} font-mono">
                    {menu.handle}
                  </p>
                </div>
              </button>
            </li>
          {/each}
        </ul>
      </div>
    </div>

    <!-- Items list -->
    <div class="lg:col-span-3">
      <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        {#if !data.selected}
          <div class="flex items-center justify-center py-20 text-gray-400 text-sm">
            Select a menu
          </div>
        {:else if flatItems.length === 0}
          <div class="flex flex-col items-center justify-center py-20 text-center">
            <div class="w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mb-3">
              <svg class="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12"/>
              </svg>
            </div>
            <p class="text-sm font-medium text-gray-400">No links yet</p>
            <button onclick={openAddItem} class="mt-3 text-sm text-gray-900 underline underline-offset-2">
              Add your first link
            </button>
          </div>
        {:else}
          <div class="divide-y divide-gray-50">
            {#each flatItems as item}
              {@const depth = (item as NavItem & { _depth: number })._depth ?? 0}
              <div class="flex items-center gap-3 px-5 py-3.5 hover:bg-gray-50/50 transition-colors"
                   style="padding-left: {1.25 + depth * 1.5}rem">
                <!-- Indent indicator -->
                {#if depth > 0}
                  <div class="w-3 h-px bg-gray-200 flex-shrink-0"></div>
                {/if}

                <!-- Icon -->
                <div class="w-8 h-8 rounded-lg bg-gray-50 flex items-center justify-center flex-shrink-0">
                  <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244"/>
                  </svg>
                </div>

                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-900">{item.label}</p>
                  <div class="flex items-center gap-2 mt-0.5">
                    <p class="text-xs text-gray-400 font-mono truncate">{item.url}</p>
                    {#if item.target === '_blank'}
                      <span class="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 text-gray-400 flex-shrink-0">
                        new tab
                      </span>
                    {/if}
                  </div>
                </div>

                <span class="text-xs text-gray-300 font-mono w-5 text-center flex-shrink-0">
                  {item.sort_order}
                </span>

                <div class="flex items-center gap-1 flex-shrink-0">
                  <button onclick={() => openEditItem(item)}
                          class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                    </svg>
                  </button>
                  <button onclick={() => deleteTarget = item}
                          class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<!-- Add / Edit item modal -->
{#if showItemForm && data.selected}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showItemForm = false} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-md p-6">
      <h3 class="text-base font-bold text-gray-900 mb-5">
        {editingItem ? 'Edit Link' : 'Add Link'}
      </h3>

      <form method="POST" action={editingItem ? '?/updateItem' : '?/addItem'}
            use:enhance={() => {
              const wasEditing = !!editingItem;
              const linkLabel = fLabel;
              return async ({ result, update }) => {
                showResult(result,
                  wasEditing ? `Link '${linkLabel}' saved` : `Link '${linkLabel}' added`,
                  wasEditing ? `Failed to save link '${linkLabel}'` : `Failed to add link '${linkLabel}'`);
                await update({ invalidateAll: true });
                if (result.type === 'success') showItemForm = false;
              };
            }}
            class="space-y-4">
        <input type="hidden" name="menu_id" value={data.selected.id} />
        {#if editingItem}
          <input type="hidden" name="item_id" value={editingItem.id} />
        {/if}

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Label</label>
          <input type="text" name="label" bind:value={fLabel} required placeholder="e.g. About Us"
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">URL</label>
          <input type="text" name="url" bind:value={fUrl} required placeholder="/about or https://..."
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                        focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Open in</label>
            <select name="target" bind:value={fTarget}
                    class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                           text-gray-900 focus:outline-none focus:ring-2 focus:ring-gray-900
                           focus:border-transparent transition bg-white">
              <option value="_self">Same tab</option>
              <option value="_blank">New tab</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">Order</label>
            <input type="number" name="sort_order" bind:value={fOrder} min="0"
                   class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                          text-gray-900 focus:outline-none focus:ring-2 focus:ring-gray-900
                          focus:border-transparent transition" />
          </div>
        </div>

        <div class="flex gap-3 pt-2">
          <button type="button" onclick={() => showItemForm = false}
                  class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                         text-gray-700 hover:bg-gray-50 transition-colors">
            Cancel
          </button>
          <button type="submit"
                  class="flex-1 inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl
                         bg-gray-900 text-white text-sm font-medium
                         hover:bg-gray-700 transition-colors">
            <SaveIcon />
            {editingItem ? 'Save Changes' : 'Add Link'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Delete confirm -->
{#if deleteTarget && data.selected}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteTarget = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">Remove link?</h3>
      <p class="text-sm text-gray-500 mb-5">
        "<span class="font-medium text-gray-700">{deleteTarget.label}</span>" will be removed from this menu.
        {#if deleteTarget.children?.length}
          <span class="text-red-500">Its sub-links will also be removed.</span>
        {/if}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </button>
        <form method="POST" action="?/deleteItem" class="flex-1"
              use:enhance={() => {
                const linkLabel = deleteTarget?.label ?? '';
                return async ({ result }) => {
                  showResult(result, `Link '${linkLabel}' removed`, `Failed to remove link '${linkLabel}'`);
                  if (result.type === 'success') await invalidateAll();
                  deleteTarget = null;
                };
              }}>
          <input type="hidden" name="menu_id" value={data.selected.id} />
          <input type="hidden" name="item_id" value={deleteTarget.id} />
          <button type="submit"
                  class="w-full px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors">
            Remove
          </button>
        </form>
      </div>
    </div>
  </div>
{/if}
