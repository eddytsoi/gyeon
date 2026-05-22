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
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<StockMutationSummary | null>(null);
  let deleting = $state(false);
  let executeTarget = $state<StockMutationSummary | null>(null);
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
      notify.success(m.admin_stock_mutations_deleted_success({ id: t.mutation_number }));
      deleteTarget = null;
      await invalidateAll();
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_delete_failure({ id: t.mutation_number }),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
    } finally {
      deleting = false;
    }
  }

  async function confirmExecute() {
    if (!executeTarget || !data.token) return;
    const row = executeTarget;
    executingId = row.id;
    try {
      await adminExecuteStockMutation(data.token, row.id);
      notify.success(m.admin_stock_mutations_executed_success({ id: row.mutation_number }));
      executeTarget = null;
      await invalidateAll();
    } catch (e) {
      if (e instanceof StockMutationInsufficientStockError) {
        const lines = e.conflicts
          .map(c => `• ${c.product_name ?? c.variant_id} (${c.variant_sku ?? '—'}): ${m.admin_stock_mutations_conflict_line({ requested: String(c.requested), available: String(c.available) })}`)
          .join('\n');
        notify.error(m.admin_stock_mutations_insufficient_stock_title({ id: row.mutation_number }), lines);
        executeTarget = null;
      } else {
        notify.error(
          m.admin_stock_mutations_execute_failure({ id: row.mutation_number }),
          e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
        );
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
      notify.success(m.admin_stock_mutations_duplicated_success({ id: created.mutation_number }));
      goto(`/admin/stock-mutations/${created.id}`);
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_duplicate_failure(),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
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

<svelte:head><title>{m.admin_stock_mutations_title()}</title></svelte:head>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-semibold text-gray-900">{m.admin_stock_mutations_heading()}</h1>
    <NewButton label={m.admin_stock_mutations_new()} href="/admin/stock-mutations/new" />
  </div>

  <!-- Filters -->
  <div class="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
    <div class="flex flex-wrap gap-3 items-center">
      <div class="flex-1 min-w-[240px]">
        <SearchInput value={data.q} placeholder={m.admin_stock_mutations_search_placeholder()} onChange={onSearch} />
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.status === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('')}>{m.admin_stock_mutations_filter_all()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'draft' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('draft')}>{m.admin_stock_mutations_status_draft()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'executed' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('executed')}>{m.admin_stock_mutations_status_executed()}</button>
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.type === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setType('')}>{m.admin_stock_mutations_filter_all_types()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}" onclick={() => setType('in')}>{m.admin_stock_mutations_type_in()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}" onclick={() => setType('out')}>{m.admin_stock_mutations_type_out()}</button>
      </div>

      <input type="date" value={data.from} onchange={(e) => setDate('from', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="from date" />
      <span class="text-gray-400 text-xs">→</span>
      <input type="date" value={data.to} onchange={(e) => setDate('to', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="to date" />

      {#if hasFilters}
        <button onclick={clearAll} class="text-xs text-gray-500 underline hover:text-gray-700">{m.admin_stock_mutations_filter_clear()}</button>
      {/if}
    </div>
  </div>

  <!-- Table -->
  <div class="bg-white border border-gray-200 rounded-xl overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
          <tr>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_number()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_type()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_status()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_items()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_total_qty()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_created()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_executed()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_actions()}</th>
          </tr>
        </thead>
        <tbody>
          {#if data.list.items.length === 0}
            <tr>
              <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-400">
                {hasFilters ? m.admin_stock_mutations_empty_with_filters() : m.admin_stock_mutations_empty_no_filters()}
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
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-50 text-emerald-700">{m.admin_stock_mutations_type_in()}</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-50 text-red-700">{m.admin_stock_mutations_type_out()}</span>
                  {/if}
                </td>
                <td class="px-4 py-2">
                  {#if row.status === 'draft'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-50 text-amber-700">{m.admin_stock_mutations_status_draft()}</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700">{m.admin_stock_mutations_status_executed()}</span>
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
                    <a href="/admin/stock-mutations/{row.id}" class="text-gray-700 hover:underline">{row.status === 'draft' ? m.admin_stock_mutations_action_edit() : m.admin_stock_mutations_action_view()}</a>
                    {#if row.status === 'draft'}
                      <button class="text-emerald-700 hover:underline disabled:opacity-50"
                              disabled={executingId === row.id}
                              onclick={() => (executeTarget = row)}>
                        {executingId === row.id ? '…' : m.admin_stock_mutations_action_execute()}
                      </button>
                      <button class="text-red-600 hover:underline" onclick={() => (deleteTarget = row)}>{m.admin_stock_mutations_action_delete()}</button>
                    {/if}
                    <button class="text-gray-500 hover:underline disabled:opacity-50"
                            disabled={duplicatingId === row.id}
                            onclick={() => duplicate(row)}>
                      {duplicatingId === row.id ? '…' : m.admin_stock_mutations_action_duplicate()}
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
      <h2 class="text-lg font-semibold">{m.admin_stock_mutations_delete_modal_title({ id: deleteTarget.mutation_number })}</h2>
      <p class="text-sm text-gray-600">{m.admin_stock_mutations_delete_modal_body()}</p>
      <div class="flex justify-end gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (deleteTarget = null)} disabled={deleting}>{m.admin_stock_mutations_cancel()}</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-red-600 text-white hover:bg-red-700 disabled:opacity-50"
                onclick={confirmDelete} disabled={deleting}>{deleting ? '…' : m.admin_stock_mutations_confirm_delete_btn()}</button>
      </div>
    </div>
  </div>
{/if}

{#if executeTarget}
  {@const busy = executingId === executeTarget.id}
  <div class="fixed inset-0 z-40 bg-black/40 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-white rounded-xl shadow-xl w-full max-w-md p-5 space-y-4">
      <h2 class="text-lg font-semibold">{m.admin_stock_mutations_execute_modal_title({ id: executeTarget.mutation_number })}</h2>
      <p class="text-sm text-gray-600">{m.admin_stock_mutations_execute_modal_body()}</p>
      <div class="flex justify-end gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (executeTarget = null)} disabled={busy}>{m.admin_stock_mutations_cancel()}</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-emerald-600 text-white hover:bg-emerald-700 disabled:opacity-50"
                onclick={confirmExecute} disabled={busy}>{busy ? '…' : m.admin_stock_mutations_confirm_execute_btn()}</button>
      </div>
    </div>
  </div>
{/if}
