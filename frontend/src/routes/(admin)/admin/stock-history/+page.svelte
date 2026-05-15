<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import StockMovementTable from '$lib/components/admin/StockMovementTable.svelte';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let fromFilter = $state(data.filters.from ?? '');
  let toFilter = $state(data.filters.to ?? '');
  let sourceFilter = $state<'all' | 'admin' | 'order'>(
    data.filters.source ?? 'all'
  );
  let qFilter = $state(data.filters.q ?? '');
  let actorFilter = $state(data.filters.actor_user_id ?? '');

  async function applyFilters() {
    const u = new URL($page.url);
    u.searchParams.delete('page');
    const set = (k: string, v: string) => {
      if (v) u.searchParams.set(k, v);
      else u.searchParams.delete(k);
    };
    set('from', fromFilter);
    set('to', toFilter);
    set('source', sourceFilter === 'all' ? '' : sourceFilter);
    set('q', qFilter);
    set('actor_user_id', actorFilter);
    await goto(u.pathname + u.search, { keepFocus: true });
  }

  async function resetFilters() {
    fromFilter = '';
    toFilter = '';
    sourceFilter = 'all';
    qFilter = '';
    actorFilter = '';
    const u = new URL($page.url);
    u.search = '';
    await goto(u.pathname, { keepFocus: true });
  }
</script>

<svelte:head><title>{m.admin_stock_history_title()}</title></svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <div>
    <h2 class="text-xl font-bold text-gray-900">{m.admin_stock_history_heading()}</h2>
    <p class="text-sm text-gray-500 mt-0.5">{m.admin_stock_history_subtitle()}</p>
  </div>

  <!-- Filters -->
  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-4">
    <div class="w-full sm:w-auto sm:min-w-44">
      <label for="from_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_stock_history_filter_from()}</label>
      <input id="from_filter" type="date" bind:value={fromFilter}
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <div class="w-full sm:w-auto sm:min-w-44">
      <label for="to_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_stock_history_filter_to()}</label>
      <input id="to_filter" type="date" bind:value={toFilter}
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <div class="w-full sm:w-auto sm:min-w-40">
      <label for="source_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_stock_history_filter_source()}</label>
      <select id="source_filter" bind:value={sourceFilter}
              class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm bg-white
                     focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
        <option value="all">{m.admin_stock_history_source_all()}</option>
        <option value="admin">{m.admin_stock_history_source_admin()}</option>
        <option value="order">{m.admin_stock_history_source_order()}</option>
      </select>
    </div>
    <div class="w-full sm:flex-1 sm:min-w-48">
      <label for="q_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_stock_history_filter_search()}</label>
      <input id="q_filter" type="text" bind:value={qFilter} placeholder="Product or SKU"
             onkeydown={(e) => { if (e.key === 'Enter') applyFilters(); }}
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <div class="flex gap-2 w-full sm:w-auto sm:contents">
      <button onclick={applyFilters}
              class="flex-1 sm:flex-none px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors">
        {m.admin_stock_history_apply()}
      </button>
      <button onclick={resetFilters} type="button"
              class="flex-1 sm:flex-none px-3 py-2 rounded-xl border border-gray-200 text-gray-600 text-sm hover:bg-gray-50 transition-colors">
        {m.admin_stock_history_reset()}
      </button>
    </div>
  </div>

  <!-- Table -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    <StockMovementTable items={data.list.items} />
  </div>

  {#if data.list.total > 0}
    <p class="text-xs text-gray-400">{m.admin_stock_history_total({ total: data.list.total })}</p>
    <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.currentPage} />
  {/if}
</div>
