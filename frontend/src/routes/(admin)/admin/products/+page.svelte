<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import type { Product } from '$lib/types';
  import { showResult } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<Product | null>(null);
</script>

<svelte:head><title>Products — Gyeon Admin</title></svelte:head>

<div class="flex items-center justify-between mb-8">
  <h1 class="text-2xl font-bold text-gray-900">Products</h1>
  <a href="/admin/products/new"
     class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
            text-sm font-medium hover:bg-gray-700 transition-colors">
    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15"/>
    </svg>
    New Product
  </a>
</div>

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
            <p class="text-xs text-gray-400 font-mono">PRD-{product.number}</p>
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
          <td class="px-5 py-3">
            <div class="flex items-center justify-end gap-1">
              <!-- Edit -->
              <a href="/admin/products/{product.id}"
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                 title="Edit">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                </svg>
              </a>
              <!-- Preview -->
              <a href="/products/{product.slug}" target="_blank"
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                 title="Preview">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178Z"/>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
                </svg>
              </a>
              <!-- Delete -->
              <button onclick={() => deleteTarget = product}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                      title="Delete">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                </svg>
              </button>
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

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteTarget = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">Delete product?</h3>
      <p class="text-sm text-gray-500 mb-5">
        "<span class="font-medium text-gray-700">{deleteTarget.name}</span>" will be permanently deleted.
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          Cancel
        </button>
        <form method="POST" action="?/delete" class="flex-1"
              use:enhance={() => {
                const targetName = deleteTarget?.name ?? '';
                return async ({ result, update }) => {
                  showResult(result, `Product '${targetName}' deleted`, `Failed to delete product '${targetName}'`);
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
