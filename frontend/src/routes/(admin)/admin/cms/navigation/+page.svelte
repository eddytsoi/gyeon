<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData } from './$types';
  import type { NavItem } from '$lib/api/admin';
  import { adminReorderNavItems } from '$lib/api/admin';
  import { showResult, notify } from '$lib/stores/notifications.svelte';
  import { sortable } from '$lib/actions/sortable';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let saving = $state(false);
  let removing = $state(false);

  let showItemForm = $state(false);
  let editingItem = $state<NavItem | null>(null);
  let deleteTarget = $state<NavItem | null>(null);

  // Item form fields
  let fLabel = $state('');
  let fUrl = $state('');
  let fTarget = $state('_self');

  function openAddItem() {
    editingItem = null;
    fLabel = '';
    fUrl = '';
    fTarget = '_self';
    showItemForm = true;
  }

  function openEditItem(item: NavItem) {
    editingItem = item;
    fLabel = item.label;
    fUrl = item.url;
    fTarget = item.target;
    showItemForm = true;
  }

  // Flatten nested items for the list display
  const flatItems = $derived.by(() => {
    const result: (NavItem & { _depth: number })[] = [];
    function walk(items: NavItem[], depth = 0) {
      for (const item of items) {
        result.push({ ...item, _depth: depth });
        if (item.children?.length) walk(item.children, depth + 1);
      }
    }
    walk(data.selected?.items ?? []);
    return result;
  });

  function switchMenu(id: string) {
    goto(`?menu=${id}`, { invalidateAll: true });
  }

  async function persistReorder(orderedIds: string[]) {
    if (!data.selected) return;
    const token = page.data.token ?? '';
    try {
      await adminReorderNavItems(token, data.selected.id, orderedIds);
      notify.success(m.admin_cms_navigation_reorder_success());
      await invalidateAll();
    } catch {
      notify.error(m.admin_cms_navigation_reorder_failure());
      await invalidateAll();
    }
  }
</script>

<svelte:head><title>{m.admin_cms_navigation_title()}</title></svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">{m.admin_cms_navigation_heading()}</h2>
      <p class="text-sm text-gray-500 mt-0.5">{m.admin_cms_navigation_subtitle()}</p>
    </div>
    {#if data.selected}
      <button onclick={openAddItem}
              class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
                     text-sm font-medium hover:bg-gray-700 transition-colors">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/>
        </svg>
        {m.admin_cms_navigation_add_link()}
      </button>
    {/if}
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
    <!-- Menu selector -->
    <div class="lg:col-span-1">
      <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-50">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_cms_navigation_section_menus()}</p>
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
            {m.admin_cms_navigation_select_menu()}
          </div>
        {:else if flatItems.length === 0}
          <div class="flex flex-col items-center justify-center py-20 text-center">
            <div class="w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mb-3">
              <svg class="w-6 h-6 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12"/>
              </svg>
            </div>
            <p class="text-sm font-medium text-gray-400">{m.admin_cms_navigation_no_links()}</p>
            <button onclick={openAddItem} class="mt-3 text-sm text-gray-900 underline underline-offset-2">
              {m.admin_cms_navigation_add_first_link()}
            </button>
          </div>
        {:else}
          <ul class="divide-y divide-gray-50"
              use:sortable={{ onReorder: persistReorder }}>
            {#each flatItems as item (item.id)}
              {@const depth = item._depth ?? 0}
              <li class="flex items-center gap-3 px-5 py-3.5 transition-colors bg-white hover:bg-gray-50/50"
                  data-id={item.id}
                  style="padding-left: {1.25 + depth * 1.5}rem">
                <!-- Drag handle -->
                <button type="button"
                        data-drag-handle
                        aria-label={m.admin_cms_navigation_aria_drag()}
                        class="cursor-grab active:cursor-grabbing p-1 -m-1 text-gray-400
                               hover:text-gray-700 transition-colors flex-shrink-0">
                  <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                    <path d="M7 4a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm6-10a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm0 5a1 1 0 1 1 0 2 1 1 0 0 1 0-2Z" />
                  </svg>
                </button>

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
                        {m.admin_cms_navigation_target_new_tab()}
                      </span>
                    {/if}
                  </div>
                </div>

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
              </li>
            {/each}
          </ul>
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
        {editingItem ? m.admin_cms_navigation_modal_edit_title() : m.admin_cms_navigation_modal_add_title()}
      </h3>

      <form method="POST" action={editingItem ? '?/updateItem' : '?/addItem'}
            use:enhance={() => {
              if (saving) return;
              saving = true;
              const wasEditing = !!editingItem;
              const linkLabel = fLabel;
              return async ({ result, update }) => {
                showResult(result,
                  wasEditing ? m.admin_cms_navigation_save_success({ label: linkLabel }) : m.admin_cms_navigation_add_success({ label: linkLabel }),
                  wasEditing ? m.admin_cms_navigation_save_failure({ label: linkLabel }) : m.admin_cms_navigation_add_failure({ label: linkLabel }));
                await update({ invalidateAll: true });
                saving = false;
                if (result.type === 'success') showItemForm = false;
              };
            }}
            class="space-y-4">
        <input type="hidden" name="menu_id" value={data.selected.id} />
        <input type="hidden" name="sort_order"
               value={editingItem ? editingItem.sort_order : flatItems.length} />
        {#if editingItem}
          <input type="hidden" name="item_id" value={editingItem.id} />
        {/if}

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_cms_navigation_label_label()}</label>
          <input type="text" name="label" bind:value={fLabel} required placeholder={m.admin_cms_navigation_label_placeholder()}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2
                        focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_cms_navigation_label_url()}</label>
          <input type="text" name="url" bind:value={fUrl} required placeholder={m.admin_cms_navigation_url_placeholder()}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        text-gray-900 placeholder-gray-400 font-mono focus:outline-none
                        focus:ring-2 focus:ring-gray-900 focus:border-transparent transition" />
        </div>

        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_cms_navigation_label_open()}</label>
          <select name="target" bind:value={fTarget}
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                         text-gray-900 focus:outline-none focus:ring-2 focus:ring-gray-900
                         focus:border-transparent transition bg-white">
            <option value="_self">{m.admin_cms_navigation_open_same()}</option>
            <option value="_blank">{m.admin_cms_navigation_open_new()}</option>
          </select>
        </div>

        <div class="flex gap-3 pt-2">
          <button type="button" onclick={() => showItemForm = false}
                  class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                         text-gray-700 hover:bg-gray-50 transition-colors">
            {m.admin_cms_navigation_cancel()}
          </button>
          <SaveButton loading={saving}
                  class="flex-1 inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl
                         bg-gray-900 text-white text-sm font-medium
                         hover:bg-gray-700 transition-colors disabled:opacity-50">
            {editingItem ? m.admin_cms_navigation_save() : m.admin_cms_navigation_add_link_submit()}
          </SaveButton>
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
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_cms_navigation_remove_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_cms_navigation_remove_body_pre()}<span class="font-medium text-gray-700">{deleteTarget.label}</span>{m.admin_cms_navigation_remove_body_post()}
        {#if deleteTarget.children?.length}
          <span class="text-red-500">{m.admin_cms_navigation_remove_warns_children()}</span>
        {/if}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          {m.admin_cms_navigation_cancel()}
        </button>
        <form method="POST" action="?/deleteItem" class="flex-1"
              use:enhance={() => {
                if (removing) return;
                removing = true;
                const linkLabel = deleteTarget?.label ?? '';
                return async ({ result }) => {
                  showResult(result, m.admin_cms_navigation_remove_success({ label: linkLabel }), m.admin_cms_navigation_remove_failure({ label: linkLabel }));
                  if (result.type === 'success') await invalidateAll();
                  removing = false;
                  deleteTarget = null;
                };
              }}>
          <input type="hidden" name="menu_id" value={data.selected.id} />
          <input type="hidden" name="item_id" value={deleteTarget.id} />
          <SaveButton loading={removing}
                  class="w-full inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors disabled:opacity-50">
            {m.admin_cms_navigation_remove_button()}
          </SaveButton>
        </form>
      </div>
    </div>
  </div>
{/if}

<style>
  :global(.gy-ghost) {
    background: #f3f4f6;
    border: 1px dashed #d1d5db;
    border-radius: 0.75rem;
    margin: 0.25rem 0;
  }
  :global(.gy-ghost) > * {
    opacity: 0;
  }
  :global(.gy-chosen) {
    background: #f9fafb;
  }
  :global(.gy-drag) {
    background: #ffffff;
    border-radius: 0.75rem;
    box-shadow: 0 12px 32px -8px rgba(17, 24, 39, 0.25),
                0 4px 12px -2px rgba(17, 24, 39, 0.1);
    border: 1px solid #e5e7eb;
    cursor: grabbing !important;
    opacity: 1;
  }
</style>
