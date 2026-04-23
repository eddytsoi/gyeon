<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head><title>Products — Gyeon Admin</title></svelte:head>

<div class="max-w-5xl">
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Products</h1>
    <a href="/admin/products/new"
       class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl
              hover:bg-gray-700 transition-colors">
      + New Product
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
              <div class="flex items-center justify-end gap-3">
                <a href="/admin/products/{product.id}"
                   class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                  Edit
                </a>
                <a href="/products/{product.slug}" target="_blank"
                   class="text-xs text-gray-400 hover:text-gray-700 transition-colors">
                  Preview ↗
                </a>
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
