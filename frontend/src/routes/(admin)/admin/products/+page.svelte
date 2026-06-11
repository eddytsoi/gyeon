<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData } from './$types';
  import type { Product } from '$lib/types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import { spotlight } from '$lib/actions/spotlight';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<Product | null>(null);

  const KINDS = ['simple', 'bundle'] as const;
  const STOCK_STATES = ['in_stock', 'low_stock', 'out_of_stock'] as const;

  const kindLabel: Record<string, string> = {
    simple: m.admin_products_filter_kind_simple(),
    bundle: m.admin_products_filter_kind_bundle(),
  };
  const stockLabel: Record<string, string> = {
    in_stock:     m.admin_products_filter_stock_in(),
    low_stock:    m.admin_products_filter_stock_low(),
    out_of_stock: m.admin_products_filter_stock_out(),
  };
  const stockChipColour: Record<string, string> = {
    in_stock:     'bg-green-50 text-green-700',
    low_stock:    'bg-amber-50 text-amber-700',
    out_of_stock: 'bg-red-50 text-red-700',
  };

  const SORT_OPTIONS = [
    'updated_desc', 'updated_asc',
    'created_desc', 'created_asc',
    'name_asc',     'name_desc',
    'price_asc',    'price_desc',
    'stock_asc',    'stock_desc',
  ] as const;
  const sortLabel: Record<string, string> = {
    updated_desc: m.admin_products_sort_updated_desc(),
    updated_asc:  m.admin_products_sort_updated_asc(),
    created_desc: m.admin_products_sort_created_desc(),
    created_asc:  m.admin_products_sort_created_asc(),
    name_asc:     m.admin_products_sort_name_asc(),
    name_desc:    m.admin_products_sort_name_desc(),
    price_asc:    m.admin_products_sort_price_asc(),
    price_desc:   m.admin_products_sort_price_desc(),
    stock_asc:    m.admin_products_sort_stock_asc(),
    stock_desc:   m.admin_products_sort_stock_desc(),
  };

  const hasFilters = $derived(
    !!data.q || !!data.category || !!data.kind || !!data.stock || (!!data.sort && data.sort !== 'updated_desc')
  );

  // Filter changes reset to page 1 — otherwise narrowing the result set
  // could leave you stranded on an empty page.
  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL(page.url);
    mutate(url.searchParams);
    url.searchParams.delete('page');
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  function onSearch(q: string) {
    pushParams(p => { q ? p.set('q', q) : p.delete('q'); });
  }

  function onCategoryChange(slug: string) {
    pushParams(p => { slug ? p.set('category', slug) : p.delete('category'); });
  }

  function setKind(kind: string) {
    pushParams(p => { kind ? p.set('kind', kind) : p.delete('kind'); });
  }

  function setStock(state: string) {
    pushParams(p => { state ? p.set('stock', state) : p.delete('stock'); });
  }

  function setSort(value: string) {
    pushParams(p => {
      // Treat empty + the default ("updated_desc") the same so the URL
      // stays clean unless the admin explicitly picks something else.
      if (!value || value === 'updated_desc') p.delete('sort');
      else p.set('sort', value);
    });
  }

  function clearAll() {
    pushParams(p => {
      p.delete('q');
      p.delete('category');
      p.delete('kind');
      p.delete('stock');
      p.delete('sort');
    });
  }

  function formatPrice(value: number): string {
    return `HK$${value.toFixed(value % 1 === 0 ? 0 : 2)}`;
  }
</script>

<svelte:head><title>{m.admin_products_title()}</title></svelte:head>

<div class="flex items-center justify-between mb-6 gap-3">
  <div class="flex items-baseline gap-3 min-w-0">
    <h1 class="text-2xl font-bold text-gray-900">{m.admin_products_heading()}</h1>
    <span class="text-sm text-gray-400">{data.total}</span>
  </div>
  <NewButton label={m.admin_products_new()} href="/admin/products/new" />
</div>

<!-- Filters -->
<div class="mb-4 space-y-3">
  <div class="flex flex-wrap items-center gap-3">
    <SearchInput value={data.q} placeholder={m.admin_products_search_placeholder()} onChange={onSearch} />

    <select
      value={data.category}
      onchange={(e) => onCategoryChange((e.currentTarget as HTMLSelectElement).value)}
      aria-label={m.admin_products_filter_category_aria()}
      class="text-sm px-3 py-2 rounded-xl border border-gray-200 bg-white
             focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900">
      <option value="">{m.admin_products_filter_category_all()}</option>
      {#each data.categories as c}
        <option value={c.slug}>{c.name}</option>
      {/each}
    </select>

    <div class="flex items-center gap-2">
      <label class="text-xs text-gray-500" for="products-sort">{m.admin_products_sort_label()}</label>
      <select
        id="products-sort"
        value={data.sort || 'updated_desc'}
        onchange={(e) => setSort((e.currentTarget as HTMLSelectElement).value)}
        class="text-sm px-3 py-2 rounded-xl border border-gray-200 bg-white
               focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900">
        {#each SORT_OPTIONS as opt}
          <option value={opt}>{sortLabel[opt]}</option>
        {/each}
      </select>
    </div>

    {#if hasFilters}
      <button type="button" onclick={clearAll}
              class="text-xs text-gray-500 hover:text-gray-900 underline-offset-2 hover:underline">
        {m.admin_products_filter_clear()}
      </button>
    {/if}
  </div>

  <!-- Row 2: kind + stock chips -->
  <div class="flex flex-wrap items-center gap-2">
    <button type="button" onclick={() => setKind('')}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                   {data.kind === ''
                     ? 'bg-gray-900 text-white border-gray-900'
                     : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
      {m.admin_products_filter_kind_all()}
    </button>
    {#each KINDS as k}
      <button type="button" onclick={() => setKind(k)}
              class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                     {data.kind === k
                       ? 'bg-indigo-50 text-indigo-700 border-current'
                       : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
        {kindLabel[k]}
      </button>
    {/each}

    <span class="h-4 w-px bg-gray-200 mx-1" aria-hidden="true"></span>

    <button type="button" onclick={() => setStock('')}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                   {data.stock === ''
                     ? 'bg-gray-900 text-white border-gray-900'
                     : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
      {m.admin_products_filter_stock_all()}
    </button>
    {#each STOCK_STATES as s}
      <button type="button" onclick={() => setStock(s)}
              class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                     {data.stock === s
                       ? `${stockChipColour[s]} border-current`
                       : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
        {stockLabel[s]}
      </button>
    {/each}
  </div>
</div>

<!-- Products table -->
<div class="bg-white rounded-2xl border border-gray-100 overflow-x-auto"
     use:spotlight={{ selector: '.js-row' }}>
  <table class="w-full text-sm">
    <thead class="bg-gray-50 border-b border-gray-100">
      <tr>
        <th class="text-left px-5 py-3 font-medium text-gray-500 w-[72px]" aria-label={m.admin_products_col_image()}></th>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_products_col_product()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">{m.admin_products_col_category()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden lg:table-cell">{m.admin_products_col_variants()}</th>
        <th class="text-right px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">{m.admin_products_col_price()}</th>
        <th class="text-right px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">{m.admin_products_col_stock()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_products_col_status()}</th>
        <th class="px-5 py-3"></th>
      </tr>
    </thead>
    <tbody class="divide-y divide-gray-50">
      {#each data.products as product}
        {@const stockQty = product.default_variant_stock_qty}
        {@const isOut = stockQty === 0}
        {@const isLow = stockQty != null && stockQty > 0 && stockQty < 5}
        {@const hasMulti = (product.variant_count ?? 0) > 1}
        {@const priceValue = hasMulti ? product.min_price : product.default_variant_price}
        {@const compareAt = hasMulti ? product.min_compare_at_price : product.default_variant_compare_at_price}
        <tr class="js-row transition-colors">
          <td class="px-5 py-3">
            <div class="w-12 h-12 rounded-lg bg-gray-100 overflow-hidden flex items-center justify-center">
              {#if product.primary_image_url}
                <img src={product.primary_image_url} alt={product.name}
                     class="w-full h-full object-cover" loading="lazy" />
              {:else}
                <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"/>
                </svg>
              {/if}
            </div>
          </td>
          <td class="px-5 py-3">
            <a href="/admin/products/{product.id}"
               class="font-medium text-gray-900 hover:text-indigo-700 hover:underline underline-offset-2">
              {product.name}
            </a>
            <p class="text-xs text-gray-400 font-mono">PRD-{product.number}</p>
          </td>
          <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">
            {data.categories.find(c => c.id === product.category_id)?.name ?? m.admin_products_dash()}
          </td>
          <td class="px-5 py-3 text-gray-500 hidden lg:table-cell">
            {product.variant_count === 1 ? m.admin_products_variants_one({ count: product.variant_count }) : m.admin_products_variants_many({ count: product.variant_count ?? 0 })}
          </td>
          <td class="px-5 py-3 text-right tabular-nums hidden sm:table-cell">
            {#if priceValue != null}
              <span class="inline-flex items-baseline gap-1.5">
                <span class="font-medium text-gray-900">
                  {hasMulti ? m.admin_products_price_from({ price: formatPrice(priceValue) }) : formatPrice(priceValue)}
                </span>
                {#if compareAt != null && compareAt > priceValue}
                  <span class="line-through text-gray-400 text-xs">{formatPrice(compareAt)}</span>
                {/if}
              </span>
            {:else}
              <span class="text-gray-300">{m.admin_products_dash()}</span>
            {/if}
          </td>
          <td class="px-5 py-3 text-right tabular-nums hidden sm:table-cell">
            {#if stockQty == null}
              <span class="text-gray-300">{m.admin_products_dash()}</span>
            {:else if isOut}
              <span class="text-gray-400 text-xs uppercase tracking-wide">{m.admin_products_stock_out()}</span>
            {:else if isLow}
              <span class="text-red-600 font-medium">{stockQty}</span>
            {:else}
              <span class="text-gray-700">{stockQty}</span>
            {/if}
          </td>
          <td class="px-5 py-3 whitespace-nowrap">
            <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                         {product.status === 'active' ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
              {product.status === 'active' ? m.admin_products_status_active() : m.admin_products_status_inactive()}
            </span>
          </td>
          <td class="px-5 py-3">
            <div class="flex items-center justify-end gap-1">
              <!-- Edit -->
              <a href="/admin/products/{product.id}"
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                 title={m.admin_products_action_edit()}>
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                </svg>
              </a>
              <!-- Preview -->
              <a href="/products/{product.slug}" target="_blank"
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                 title={m.admin_products_action_preview()}>
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178Z"/>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
                </svg>
              </a>
              <!-- Delete -->
              <button onclick={() => deleteTarget = product}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                      title={m.admin_products_action_delete()}>
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
          <td colspan="8" class="px-5 py-10 text-center text-gray-400">
            {data.q ? m.admin_products_no_match({ query: data.q }) : m.admin_products_empty()}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<Pagination total={data.total} pageSize={data.pageSize} currentPage={data.page} />

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteTarget = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_products_delete_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_products_delete_body_pre()}<span class="font-medium text-gray-700">{deleteTarget.name}</span>{m.admin_products_delete_body_post()}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          {m.common_cancel()}
        </button>
        <form method="POST" action="?/delete" class="flex-1"
              use:enhance={() => {
                const targetName = deleteTarget?.name ?? '';
                return async ({ result, update }) => {
                  showResult(result, m.admin_products_deleted_success({ name: targetName }), m.admin_products_deleted_failure({ name: targetName }));
                  await update();
                  deleteTarget = null;
                };
              }}>
          <input type="hidden" name="id" value={deleteTarget.id} />
          <button type="submit"
                  class="w-full px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors">
            {m.admin_products_action_delete()}
          </button>
        </form>
      </div>
    </div>
  </div>
{/if}
