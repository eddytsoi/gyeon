<script lang="ts">
  import type { PageData } from './$types';
  import type { AuditRow } from '$lib/api/admin';
  import { spotlight } from '$lib/actions/spotlight';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let actionFilter = $state(data.filters.action ?? '');
  let entityFilter = $state(data.filters.entity_type ?? '');
  let expanded = $state<string | null>(null);

  function fmtDate(s: string): string {
    return new Date(s).toLocaleString();
  }

  function pretty(json?: string): string {
    if (!json) return '—';
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
    if (actionFilter) u.searchParams.set('action', actionFilter); else u.searchParams.delete('action');
    if (entityFilter) u.searchParams.set('entity_type', entityFilter); else u.searchParams.delete('entity_type');
    await goto(u.pathname + u.search, { keepFocus: true });
  }

  function toggle(id: string) {
    expanded = expanded === id ? null : id;
  }
</script>

<svelte:head><title>{m.admin_audit_log_title()}</title></svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <div>
    <h2 class="text-xl font-bold text-gray-900">{m.admin_audit_log_heading()}</h2>
    <p class="text-sm text-gray-500 mt-0.5">{m.admin_audit_log_subtitle()}</p>
  </div>

  <!-- Filters -->
  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-4 flex flex-wrap items-end gap-4">
    <div class="flex-1 min-w-48">
      <label for="action_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_audit_log_filter_action()}</label>
      <input id="action_filter" type="text" bind:value={actionFilter} placeholder="e.g. order.refund"
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm font-mono
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <div class="flex-1 min-w-48">
      <label for="entity_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_audit_log_filter_entity()}</label>
      <input id="entity_filter" type="text" bind:value={entityFilter} placeholder="e.g. order"
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm font-mono
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <button onclick={applyFilters}
            class="px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors">
      {m.admin_audit_log_apply()}
    </button>
  </div>

  <!-- Table -->
  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    {#if data.list.items.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <p class="text-sm font-medium text-gray-400">{m.admin_audit_log_empty()}</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_audit_log_col_time()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_audit_log_col_actor()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_audit_log_col_action()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_audit_log_col_entity()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_audit_log_col_ip()}</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.list.items as row}
            <tr class="js-row transition-colors">
              <td class="px-6 py-3 text-gray-600 text-xs whitespace-nowrap">{fmtDate(row.created_at)}</td>
              <td class="px-6 py-3 text-gray-700 text-xs">
                {#if row.admin_email}
                  {row.admin_email}
                {:else}
                  <span class="text-gray-400 italic">{m.admin_audit_log_actor_deleted()}</span>
                {/if}
              </td>
              <td class="px-6 py-3 text-gray-700 font-mono text-xs">{row.action}</td>
              <td class="px-6 py-3 text-gray-500 text-xs">
                {row.entity_type}{row.entity_id ? ` · ${row.entity_id.slice(0, 8)}` : ''}
              </td>
              <td class="px-6 py-3 text-gray-400 text-xs font-mono">{row.ip ?? '—'}</td>
              <td class="px-6 py-3 text-right">
                <button onclick={() => toggle(row.id)}
                        class="px-2 py-1 rounded-lg text-xs text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition-colors">
                  {expanded === row.id ? m.admin_audit_log_hide() : m.admin_audit_log_inspect()}
                </button>
              </td>
            </tr>
            {#if expanded === row.id}
              <tr>
                <td colspan="6" class="px-6 py-4 bg-gray-50">
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_audit_log_before()}</p>
                      <pre class="text-xs bg-white border border-gray-100 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap">{pretty(row.before)}</pre>
                    </div>
                    <div>
                      <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_audit_log_after()}</p>
                      <pre class="text-xs bg-white border border-gray-100 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap">{pretty(row.after)}</pre>
                    </div>
                  </div>
                  {#if row.user_agent}
                    <p class="mt-3 text-xs text-gray-400 truncate">{row.user_agent}</p>
                  {/if}
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  {#if data.list.total > 0}
    <p class="text-xs text-gray-400">{m.admin_audit_log_total({ total: data.list.total })}</p>
  {/if}

  <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.page} />
</div>
