<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import {
    adminDeleteStockMutation,
    adminDuplicateStockMutation,
    adminExecuteStockMutation,
    StockMutationInsufficientStockError,
    type StockMutationSummary
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<StockMutationSummary | null>(null);
  let deleting = $state(false);
  let executingId = $state<string | null>(null);
  let duplicatingId = $state<string | null>(null);

  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL(page.url);
    mutate(url.searchParams);
    url.searchParams.delete('page');
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  function onSearch(q: string) {
    pushParams(p => { q ? p.set('q', q) : p.delete('q'); });
  }
  function setStatus(s: '' | 'draft' | 'executed') {
    pushParams(p => { s ? p.set('status', s) : p.delete('status'); });
  }
  function setType(t: '' | 'in' | 'out') {
    pushParams(p => { t ? p.set('type', t) : p.delete('type'); });
  }
  function setDate(key: 'from' | 'to', value: string) {
    pushParams(p => { value ? p.set(key, value) : p.delete(key); });
  }
  function clearAll() {
    pushParams(p => {
      p.delete('q'); p.delete('status'); p.delete('type');
      p.delete('from'); p.delete('to');
    });
  }

  async function confirmDelete() {
    if (!deleteTarget || !data.token) return;
    const t = deleteTarget;
    deleting = true;
    try {
      await adminDeleteStockMutation(data.token, t.id);
      notify.success(`已刪除 ${t.mutation_number}`);
      deleteTarget = null;
      await invalidateAll();
    } catch (e) {
      notify.error(
        `刪除 ${t.mutation_number} 失敗`,
        e instanceof Error ? e.message : 'unknown error'
      );
    } finally {
      deleting = false;
    }
  }

  async function execute(row: StockMutationSummary) {
    if (!data.token) return;
    if (!confirm(`確定執行 ${row.mutation_number}？執行後不能撤銷或修改。`)) return;
    executingId = row.id;
    try {
      await adminExecuteStockMutation(data.token, row.id);
      notify.success(`${row.mutation_number} 已執行`);
      await invalidateAll();
    } catch (e) {
      if (e instanceof StockMutationInsufficientStockError) {
        const lines = e.conflicts
          .map(c => `• ${c.product_name ?? c.variant_id} (${c.variant_sku ?? '—'}): 需要 ${c.requested}, 現有 ${c.available}`)
          .join('\n');
        notify.error(`${row.mutation_number} 庫存不足`, lines);
      } else {
        notify.error(`執行 ${row.mutation_number} 失敗`, e instanceof Error ? e.message : 'unknown error');
      }
    } finally {
      executingId = null;
    }
  }

  async function duplicate(row: StockMutationSummary) {
    if (!data.token) return;
    duplicatingId = row.id;
    try {
      const created = await adminDuplicateStockMutation(data.token, row.id);
      notify.success(`已複製為 ${created.mutation_number}`);
      goto(`/admin/stock-mutations/${created.id}`);
    } catch (e) {
      notify.error('複製失敗', e instanceof Error ? e.message : 'unknown error');
    } finally {
      duplicatingId = null;
    }
  }

  function fmtDateTime(iso: string | undefined | null) {
    if (!iso) return '—';
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
    }
  }

  const hasFilters = $derived(
    !!data.q || !!data.status || !!data.type || !!data.from || !!data.to
  );
</script>

<svelte:head><title>Stock Mutations · Admin</title></svelte:head>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-semibold text-gray-900">Stock Mutations</h1>
    <NewButton label="New Mutation" href="/admin/stock-mutations/new" />
  </div>

  <!-- Filters -->
  <div class="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
    <div class="flex flex-wrap gap-3 items-center">
      <div class="flex-1 min-w-[240px]">
        <SearchInput value={data.q} placeholder="Search MUT-#, SKU or product name" onChange={onSearch} />
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.status === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('')}>All</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'draft' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('draft')}>Draft</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'executed' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('executed')}>Executed</button>
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.type === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setType('')}>All types</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}" onclick={() => setType('in')}>Stock In</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}" onclick={() => setType('out')}>Stock Out</button>
      </div>

      <input type="date" value={data.from} onchange={(e) => setDate('from', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="from date" />
      <span class="text-gray-400 text-xs">→</span>
      <input type="date" value={data.to} onchange={(e) => setDate('to', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="to date" />

      {#if hasFilters}
        <button onclick={clearAll} class="text-xs text-gray-500 underline hover:text-gray-700">Clear</button>
      {/if}
    </div>
  </div>

  <!-- Table -->
  <div class="bg-white border border-gray-200 rounded-xl overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
          <tr>
            <th class="px-4 py-2 font-medium">Mutation #</th>
            <th class="px-4 py-2 font-medium">Type</th>
            <th class="px-4 py-2 font-medium">Status</th>
            <th class="px-4 py-2 font-medium text-right">Items</th>
            <th class="px-4 py-2 font-medium text-right">Total qty</th>
            <th class="px-4 py-2 font-medium">Created</th>
            <th class="px-4 py-2 font-medium">Executed</th>
            <th class="px-4 py-2 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#if data.list.items.length === 0}
            <tr>
              <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-400">
                未有 stock mutation。{#if !hasFilters}試吓建立一張新嘅。{:else}試吓清除 filter。{/if}
              </td>
            </tr>
          {:else}
            {#each data.list.items as row (row.id)}
              <tr class="border-t border-gray-100 hover:bg-gray-50">
                <td class="px-4 py-2 font-mono text-sm">
                  <a href="/admin/stock-mutations/{row.id}" class="text-gray-900 hover:underline">{row.mutation_number}</a>
                </td>
                <td class="px-4 py-2">
                  {#if row.type === 'in'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-50 text-emerald-700">Stock In</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-50 text-red-700">Stock Out</span>
                  {/if}
                </td>
                <td class="px-4 py-2">
                  {#if row.status === 'draft'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-50 text-amber-700">Draft</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700">Executed</span>
                  {/if}
                </td>
                <td class="px-4 py-2 text-right tabular-nums">{row.item_count}</td>
                <td class="px-4 py-2 text-right tabular-nums">{row.total_quantity}</td>
                <td class="px-4 py-2 text-xs text-gray-500">
                  <div>{fmtDateTime(row.created_at)}</div>
                  {#if row.created_by_email}<div class="text-gray-400">{row.created_by_email}</div>{/if}
                </td>
                <td class="px-4 py-2 text-xs text-gray-500">
                  {#if row.executed_at}
                    <div>{fmtDateTime(row.executed_at)}</div>
                    {#if row.executed_by_email}<div class="text-gray-400">{row.executed_by_email}</div>{/if}
                  {:else}
                    <span class="text-gray-300">—</span>
                  {/if}
                </td>
                <td class="px-4 py-2 text-right">
                  <div class="inline-flex items-center gap-2 text-xs">
                    <a href="/admin/stock-mutations/{row.id}" class="text-gray-700 hover:underline">{row.status === 'draft' ? 'Edit' : 'View'}</a>
                    {#if row.status === 'draft'}
                      <button class="text-emerald-700 hover:underline disabled:opacity-50"
                              disabled={executingId === row.id}
                              onclick={() => execute(row)}>
                        {executingId === row.id ? '…' : 'Execute'}
                      </button>
                      <button class="text-red-600 hover:underline" onclick={() => (deleteTarget = row)}>Delete</button>
                    {/if}
                    <button class="text-gray-500 hover:underline disabled:opacity-50"
                            disabled={duplicatingId === row.id}
                            onclick={() => duplicate(row)}>
                      {duplicatingId === row.id ? '…' : 'Duplicate'}
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.page} />
</div>

{#if deleteTarget}
  <div class="fixed inset-0 z-40 bg-black/40 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-white rounded-xl shadow-xl w-full max-w-md p-5 space-y-4">
      <h2 class="text-lg font-semibold">刪除 {deleteTarget.mutation_number}？</h2>
      <p class="text-sm text-gray-600">呢個操作不能撤銷。Draft 才能刪除，已執行嘅 mutation 不能刪除。</p>
      <div class="flex justify-end gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (deleteTarget = null)} disabled={deleting}>取消</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-red-600 text-white hover:bg-red-700 disabled:opacity-50"
                onclick={confirmDelete} disabled={deleting}>{deleting ? '…' : '確定刪除'}</button>
      </div>
    </div>
  </div>
{/if}
