<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData, ActionData } from './$types';
  import type { AdminUser } from '$lib/api/admin';
  import { spotlight } from '$lib/actions/spotlight';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import AdminModal from '$lib/components/admin/AdminModal.svelte';
  import SaveIcon from '$lib/components/admin/SaveIcon.svelte';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let showCreate = $state(false);
  let editingUser = $state<AdminUser | null>(null);

  function onSearch(q: string) {
    const url = new URL(page.url);
    if (q) url.searchParams.set('q', q);
    else url.searchParams.delete('q');
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  const roleLabel: Record<string, string> = {
    super_admin: 'Super Admin',
    admin: 'Admin',
    editor: 'Editor'
  };

  const roleBadge: Record<string, string> = {
    super_admin: 'bg-violet-50 text-violet-700',
    admin: 'bg-blue-50 text-blue-700',
    editor: 'bg-gray-100 text-gray-600'
  };
</script>

<svelte:head><title>Users — Gyeon Admin</title></svelte:head>

<div class="max-w-4xl">
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-gray-900">Admin Users</h1>
    <NewButton label="New User" action={() => showCreate = true} />
  </div>

  <div class="mb-4">
    <SearchInput value={data.q} placeholder="Search by name or email…" onChange={onSearch} />
  </div>

  {#if form?.error}
    <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
      {form.error}
    </div>
  {/if}

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    <table class="w-full text-sm">
      <thead class="bg-gray-50 border-b border-gray-100">
        <tr>
          <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Name</th>
          <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">Email</th>
          <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Role</th>
          <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden md:table-cell">Status</th>
          <th class="px-5 py-3"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.users as user}
          <tr class="js-row transition-colors">
            <td class="px-5 py-3">
              <p class="font-medium text-gray-900">{user.name}</p>
              <p class="text-xs text-gray-400 sm:hidden">{user.email}</p>
            </td>
            <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">{user.email}</td>
            <td class="px-5 py-3">
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                           {roleBadge[user.role] ?? 'bg-gray-100 text-gray-600'}">
                {roleLabel[user.role] ?? user.role}
              </span>
            </td>
            <td class="px-5 py-3 hidden md:table-cell">
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                           {user.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
                {user.is_active ? 'Active' : 'Inactive'}
              </span>
            </td>
            <td class="px-5 py-3 text-right">
              <div class="flex items-center justify-end gap-1">
                <!-- Edit -->
                <button onclick={() => editingUser = user}
                        title="Edit"
                        aria-label="Edit user"
                        class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                  </svg>
                </button>
                <!-- Delete -->
                <form method="POST" action="?/delete" use:enhance class="inline-flex">
                  <input type="hidden" name="id" value={user.id} />
                  <button type="submit"
                          title="Delete"
                          aria-label="Delete user"
                          class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                          onclick={(e) => { if (!confirm(`Delete ${user.name}?`)) e.preventDefault(); }}>
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                    </svg>
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="px-5 py-8 text-center text-gray-400 text-sm">
              {data.q ? `No admin users matching "${data.q}".` : 'No admin users found.'}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<!-- ── Create User Modal ── -->
<AdminModal open={showCreate} onClose={() => showCreate = false}>
  <h3 class="font-semibold text-gray-900 mb-4">New Admin User</h3>
  <form method="POST" action="?/create"
        use:enhance={() => async ({ update }) => { await update(); showCreate = false; }}>
    <div class="flex flex-col gap-4">
      <div class="flex flex-col gap-1.5">
        <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Name *</label>
        <input name="name" required
               class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div class="flex flex-col gap-1.5">
        <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Email *</label>
        <input name="email" type="email" required
               class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div class="flex flex-col gap-1.5">
        <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Password *</label>
        <input name="password" type="password" required minlength="8"
               class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div class="flex flex-col gap-1.5">
        <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Role</label>
        <select name="role"
                class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900">
          <option value="editor">Editor</option>
          <option value="admin">Admin</option>
          <option value="super_admin">Super Admin</option>
        </select>
      </div>
    </div>
    <div class="flex gap-3 mt-5">
      <button type="submit"
              class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900 text-white
                     text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors">
        <SaveIcon />
        Create User
      </button>
      <button type="button" onclick={() => showCreate = false}
              class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                     hover:border-gray-400 transition-colors">
        Cancel
      </button>
    </div>
  </form>
</AdminModal>

<!-- ── Edit User Modal ── -->
<AdminModal open={!!editingUser} onClose={() => editingUser = null}>
  {#if editingUser}
    <h3 class="font-semibold text-gray-900 mb-4">Edit User</h3>
    <form method="POST" action="?/update"
          use:enhance={() => async ({ update }) => { await update(); editingUser = null; }}>
      <input type="hidden" name="id" value={editingUser.id} />
      <div class="flex flex-col gap-4">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Name *</label>
          <input name="name" required value={editingUser.name}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Role</label>
          <select name="role"
                  class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="editor" selected={editingUser.role === 'editor'}>Editor</option>
            <option value="admin" selected={editingUser.role === 'admin'}>Admin</option>
            <option value="super_admin" selected={editingUser.role === 'super_admin'}>Super Admin</option>
          </select>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</label>
          <select name="is_active"
                  class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            <option value="true" selected={editingUser.is_active}>Active</option>
            <option value="false" selected={!editingUser.is_active}>Inactive</option>
          </select>
        </div>
      </div>
      <div class="flex gap-3 mt-5">
        <button type="submit"
                class="flex-1 inline-flex items-center justify-center gap-1.5 py-2.5 bg-gray-900 text-white
                       text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors">
          <SaveIcon />
          Save Changes
        </button>
        <button type="button" onclick={() => editingUser = null}
                class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                       hover:border-gray-400 transition-colors">
          Cancel
        </button>
      </div>
    </form>
  {/if}
</AdminModal>
