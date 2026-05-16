<script lang="ts">
  import type { PageData } from './$types';
  import type { QueueJobRow } from '$lib/api/admin';
  import { adminRetryQueueJob } from '$lib/api/admin';
  import { spotlight } from '$lib/actions/spotlight';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let statusFilter = $state(data.filters.status ?? '');
  let typeFilter = $state(data.filters.type ?? '');
  let expanded = $state<string | null>(null);
  let retryingId = $state<string | null>(null);

  function fmtDate(s?: string): string {
    if (!s) return '—';
    return new Date(s).toLocaleString();
  }

  function pretty(json: string): string {
    try {
      return JSON.stringify(JSON.parse(json), null, 2);
    } catch {
      return json;
    }
  }

  async function applyFilters() {
    const u = new URL($page.url);
    u.searchParams.delete('offset');
    u.searchParams.delete('page');
    if (statusFilter) u.searchParams.set('status', statusFilter); else u.searchParams.delete('status');
    if (typeFilter) u.searchParams.set('type', typeFilter); else u.searchParams.delete('type');
    await goto(u.pathname + u.search, { keepFocus: true });
  }

  function toggle(id: string) {
    expanded = expanded === id ? null : id;
  }

  async function retry(row: QueueJobRow) {
    if (retryingId) return;
    retryingId = row.id;
    try {
      const token = document.cookie.split('admin_token=')[1]?.split(';')[0] ?? '';
      await adminRetryQueueJob(token, row.id);
      await invalidateAll();
    } catch (e) {
      console.error('retry failed', e);
    } finally {
      retryingId = null;
    }
  }

  function statusBadge(s: string): string {
    switch (s) {
      case 'succeeded': return 'bg-emerald-50 text-emerald-700';
      case 'failed':    return 'bg-orange-50 text-orange-700';
      case 'dead':      return 'bg-red-50 text-red-700';
      case 'processing':return 'bg-blue-50 text-blue-700';
      default:          return 'bg-gray-100 text-gray-600';
    }
  }
</script>

<svelte:head><title>{m.admin_queue_jobs_title()}</title></svelte:head>

<div class="space-y-6">
  <div>
    <h2 class="text-xl font-bold text-gray-900">{m.admin_queue_jobs_heading()}</h2>
    <p class="text-sm text-gray-500 mt-0.5">{m.admin_queue_jobs_subtitle()}</p>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-4 flex flex-wrap items-end gap-4">
    <div class="flex-1 min-w-40">
      <label for="qj_status" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_queue_jobs_filter_status()}</label>
      <select id="qj_status" bind:value={statusFilter}
              class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                     focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
        <option value="">{m.admin_queue_jobs_filter_status_all()}</option>
        <option value="pending">pending</option>
        <option value="processing">processing</option>
        <option value="succeeded">succeeded</option>
        <option value="failed">failed</option>
        <option value="dead">dead</option>
      </select>
    </div>
    <div class="flex-1 min-w-48">
      <label for="qj_type" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_queue_jobs_filter_type()}</label>
      <input id="qj_type" type="text" bind:value={typeFilter} placeholder="send_email"
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm font-mono
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <button onclick={applyFilters}
            class="px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors">
      {m.admin_queue_jobs_apply()}
    </button>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    {#if data.list.items.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <p class="text-sm font-medium text-gray-400">{m.admin_queue_jobs_empty()}</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_queue_jobs_col_time()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_queue_jobs_col_type()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_queue_jobs_col_status()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_queue_jobs_col_attempts()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_queue_jobs_col_run_after()}</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.list.items as row}
            <tr class="js-row transition-colors">
              <td class="px-6 py-3 text-gray-600 text-xs whitespace-nowrap">{fmtDate(row.created_at)}</td>
              <td class="px-6 py-3 text-gray-700 font-mono text-xs">{row.type}</td>
              <td class="px-6 py-3 text-xs">
                <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {statusBadge(row.status)}">{row.status}</span>
              </td>
              <td class="px-6 py-3 text-gray-500 text-xs font-mono">{row.attempts} / {row.max_attempts}</td>
              <td class="px-6 py-3 text-gray-500 text-xs whitespace-nowrap">{fmtDate(row.run_after)}</td>
              <td class="px-6 py-3 text-right whitespace-nowrap">
                <button onclick={() => toggle(row.id)}
                        class="px-2 py-1 rounded-lg text-xs text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition-colors">
                  {expanded === row.id ? m.admin_queue_jobs_hide() : m.admin_queue_jobs_inspect()}
                </button>
                {#if row.status === 'dead' || row.status === 'failed'}
                  <button onclick={() => retry(row)} disabled={retryingId === row.id}
                          class="ml-1 px-2 py-1 rounded-lg text-xs text-gray-600 hover:text-gray-900 hover:bg-gray-100 transition-colors disabled:opacity-50">
                    {retryingId === row.id ? m.admin_queue_jobs_retrying() : m.admin_queue_jobs_retry()}
                  </button>
                {/if}
              </td>
            </tr>
            {#if expanded === row.id}
              <tr>
                <td colspan="6" class="px-6 py-4 bg-gray-50">
                  {#if row.last_error}
                    <div class="mb-3 p-3 rounded-lg bg-red-50 border border-red-100 text-xs text-red-700">
                      <span class="font-semibold">{m.admin_queue_jobs_last_error()}：</span>
                      <span class="font-mono">{row.last_error}</span>
                    </div>
                  {/if}
                  <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_queue_jobs_payload()}</p>
                  <pre class="text-xs bg-white border border-gray-100 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap">{pretty(row.payload)}</pre>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  {#if data.list.total > 0}
    <p class="text-xs text-gray-400">{m.admin_queue_jobs_total({ total: data.list.total })}</p>
  {/if}

  <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.page} />
</div>
