<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import type { AdminUser } from '$lib/api/admin';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let showCreate = $state(false);
  let editingUser = $state<AdminUser | null>(null);

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
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Admin Users</h1>
    <button onclick={() => showCreate = true}
            class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl
                   hover:bg-gray-700 transition-colors">
      + New User
    </button>
  </div>

  {#if form?.error}
    <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
      {form.error}
    </div>
  {/if}

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
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
          <tr class="hover:bg-gray-50 transition-colors">
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
              <div class="flex items-center justify-end gap-3">
                <button onclick={() => editingUser = user}
                        class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                  Edit
                </button>
                <form method="POST" action="?/delete" use:enhance>
                  <input type="hidden" name="id" value={user.id} />
                  <button type="submit"
                          class="text-xs text-red-400 hover:text-red-600 transition-colors"
                          onclick={(e) => { if (!confirm(`Delete ${user.name}?`)) e.preventDefault(); }}>
                    Delete
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="px-5 py-8 text-center text-gray-400 text-sm">No admin users found.</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<!-- ── Create User Modal ── -->
{#if showCreate}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showCreate = false} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
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
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            Create User
          </button>
          <button type="button" onclick={() => showCreate = false}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- ── Edit User Modal ── -->
{#if editingUser}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => editingUser = null} role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
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
                  class="flex-1 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            Save Changes
          </button>
          <button type="button" onclick={() => editingUser = null}
                  class="flex-1 py-2.5 border border-gray-200 text-gray-600 text-sm rounded-xl
                         hover:border-gray-400 transition-colors">
            Cancel
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
