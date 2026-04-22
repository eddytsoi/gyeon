<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  let showCreate = $state(false);
  let creating = $state(false);

  function slugify(s: string) {
    return s.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
  }

  let newName = $state('');
  let newSlug = $derived(slugify(newName));
</script>

<svelte:head><title>Products — Gyeon Admin</title></svelte:head>

<div class="max-w-5xl">
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Products</h1>
    <button onclick={() => showCreate = !showCreate}
            class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl
                   hover:bg-gray-700 transition-colors">
      + New Product
    </button>
  </div>

  <!-- Create form -->
  {#if showCreate}
    <form method="POST" action="?/create" class="bg-white rounded-2xl border border-gray-100 p-6 mb-6"
          use:enhance={() => {
            creating = true;
            return async ({ update }) => { await update(); creating = false; showCreate = false; };
          }}>
      <h2 class="font-semibold text-gray-900 mb-4">New Product</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="new-name" class="text-xs font-medium text-gray-600">Name *</label>
          <input id="new-name" name="name" required bind:value={newName}
                 class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                        focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="new-slug" class="text-xs font-medium text-gray-600">Slug *</label>
          <input id="new-slug" name="slug" required value={newSlug}
                 class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                        focus:ring-2 focus:ring-gray-900 bg-gray-50" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="new-category" class="text-xs font-medium text-gray-600">Category</label>
          <select id="new-category" name="category_id"
                  class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                         focus:ring-2 focus:ring-gray-900">
            <option value="">— None —</option>
            {#each data.categories as cat}
              <option value={cat.id}>{cat.name}</option>
            {/each}
          </select>
        </div>
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <label for="new-desc" class="text-xs font-medium text-gray-600">Description</label>
          <textarea id="new-desc" name="description" rows="3"
                    class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none
                           focus:ring-2 focus:ring-gray-900 resize-none"></textarea>
        </div>
      </div>
      <div class="flex gap-3 mt-4">
        <button type="submit" disabled={creating}
                class="px-5 py-2 bg-gray-900 text-white text-sm font-medium rounded-lg
                       hover:bg-gray-700 transition-colors disabled:opacity-50">
          {creating ? 'Creating…' : 'Create'}
        </button>
        <button type="button" onclick={() => showCreate = false}
                class="px-5 py-2 border border-gray-200 text-gray-600 text-sm rounded-lg
                       hover:border-gray-400 transition-colors">
          Cancel
        </button>
      </div>
    </form>
  {/if}

  <!-- Products table -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    <table class="w-full text-sm">
      <thead class="bg-gray-50 border-b border-gray-100">
        <tr>
          <th class="text-left px-5 py-3 font-medium text-gray-500">Product</th>
          <th class="text-left px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">Category</th>
          <th class="text-left px-5 py-3 font-medium text-gray-500 hidden md:table-cell">Variants</th>
          <th class="text-left px-5 py-3 font-medium text-gray-500">Status</th>
          <th class="px-5 py-3"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.products as { product, variants }}
          <tr class="hover:bg-gray-50 transition-colors">
            <td class="px-5 py-3">
              <p class="font-medium text-gray-900">{product.name}</p>
              <p class="text-xs text-gray-400">{product.slug}</p>
            </td>
            <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">
              {data.categories.find(c => c.id === product.category_id)?.name ?? '—'}
            </td>
            <td class="px-5 py-3 text-gray-500 hidden md:table-cell">
              {variants.length} variant{variants.length !== 1 ? 's' : ''}
            </td>
            <td class="px-5 py-3">
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                           {product.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
                {product.is_active ? 'Active' : 'Inactive'}
              </span>
            </td>
            <td class="px-5 py-3 text-right">
              <div class="flex items-center justify-end gap-2">
                <a href="/products/{product.slug}" target="_blank"
                   class="text-xs text-gray-400 hover:text-gray-700 transition-colors">
                  Preview ↗
                </a>
                <form method="POST" action="?/toggle" use:enhance>
                  <input type="hidden" name="id" value={product.id} />
                  <input type="hidden" name="slug" value={product.slug} />
                  <input type="hidden" name="name" value={product.name} />
                  <input type="hidden" name="is_active" value={product.is_active} />
                  <button type="submit"
                          class="text-xs text-gray-400 hover:text-gray-700 transition-colors">
                    {product.is_active ? 'Deactivate' : 'Activate'}
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="px-5 py-10 text-center text-gray-400">No products yet.</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
