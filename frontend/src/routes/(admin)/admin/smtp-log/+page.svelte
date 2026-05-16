<script lang="ts">
  import type { PageData } from './$types';
  import type { SmtpLogRow } from '$lib/api/admin';
  import { adminResendSmtpLog } from '$lib/api/admin';
  import { spotlight } from '$lib/actions/spotlight';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  let statusFilter = $state(data.filters.status ?? '');
  let templateFilter = $state(data.filters.template_key ?? '');
  let recipientFilter = $state(data.filters.recipient ?? '');
  let expanded = $state<string | null>(null);
  let resendingId = $state<string | null>(null);
  let bodyTab = $state<Record<string, 'html' | 'text'>>({});

  function fmtDate(s: string): string {
    return new Date(s).toLocaleString();
  }

  function truncate(s: string, n: number): string {
    return s.length > n ? s.slice(0, n) + '…' : s;
  }

  async function applyFilters() {
    const u = new URL($page.url);
    u.searchParams.delete('offset');
    u.searchParams.delete('page');
    if (statusFilter) u.searchParams.set('status', statusFilter); else u.searchParams.delete('status');
    if (templateFilter) u.searchParams.set('template_key', templateFilter); else u.searchParams.delete('template_key');
    if (recipientFilter) u.searchParams.set('recipient', recipientFilter); else u.searchParams.delete('recipient');
    await goto(u.pathname + u.search, { keepFocus: true });
  }

  function toggle(id: string) {
    expanded = expanded === id ? null : id;
    if (expanded && !bodyTab[id]) bodyTab[id] = 'html';
  }

  async function resend(row: SmtpLogRow) {
    if (resendingId) return;
    resendingId = row.id;
    try {
      const token = document.cookie.split('admin_token=')[1]?.split(';')[0] ?? '';
      await adminResendSmtpLog(token, row.id);
      await invalidateAll();
    } catch (e) {
      console.error('resend failed', e);
    } finally {
      resendingId = null;
    }
  }
</script>

<svelte:head><title>{m.admin_smtp_log_title()}</title></svelte:head>

<div class="space-y-6">
  <div>
    <h2 class="text-xl font-bold text-gray-900">{m.admin_smtp_log_heading()}</h2>
    <p class="text-sm text-gray-500 mt-0.5">{m.admin_smtp_log_subtitle()}</p>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 px-6 py-4 flex flex-wrap items-end gap-4">
    <div class="flex-1 min-w-40">
      <label for="status_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_smtp_log_filter_status()}</label>
      <select id="status_filter" bind:value={statusFilter}
              class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                     focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
        <option value="">{m.admin_smtp_log_filter_status_all()}</option>
        <option value="sent">{m.admin_smtp_log_status_sent()}</option>
        <option value="failed">{m.admin_smtp_log_status_failed()}</option>
      </select>
    </div>
    <div class="flex-1 min-w-48">
      <label for="template_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_smtp_log_filter_template()}</label>
      <input id="template_filter" type="text" bind:value={templateFilter} placeholder="order_confirmation"
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm font-mono
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <div class="flex-1 min-w-48">
      <label for="recipient_filter" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_smtp_log_filter_recipient()}</label>
      <input id="recipient_filter" type="text" bind:value={recipientFilter} placeholder="customer@example.com"
             class="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
    </div>
    <button onclick={applyFilters}
            class="px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 transition-colors">
      {m.admin_smtp_log_apply()}
    </button>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    {#if data.list.items.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <p class="text-sm font-medium text-gray-400">{m.admin_smtp_log_empty()}</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_time()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_title()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_recipient()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_subject()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_status()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_smtp_log_col_trigger()}</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.list.items as row}
            <tr class="js-row transition-colors">
              <td class="px-6 py-3 text-gray-600 text-xs whitespace-nowrap">{fmtDate(row.created_at)}</td>
              <td class="px-6 py-3 text-gray-700 font-mono text-xs">{row.template_key ?? '—'}</td>
              <td class="px-6 py-3 text-gray-700 text-xs">{row.recipient}</td>
              <td class="px-6 py-3 text-gray-500 text-xs">{truncate(row.subject, 50)}</td>
              <td class="px-6 py-3 text-xs">
                {#if row.status === 'sent'}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 font-medium">
                    {m.admin_smtp_log_status_sent()}
                  </span>
                {:else}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-red-50 text-red-700 font-medium">
                    {m.admin_smtp_log_status_failed()}
                  </span>
                {/if}
              </td>
              <td class="px-6 py-3 text-gray-500 font-mono text-xs">{row.trigger_condition}</td>
              <td class="px-6 py-3 text-right whitespace-nowrap">
                <button onclick={() => toggle(row.id)}
                        class="px-2 py-1 rounded-lg text-xs text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition-colors">
                  {expanded === row.id ? m.admin_smtp_log_hide() : m.admin_smtp_log_inspect()}
                </button>
                <button onclick={() => resend(row)} disabled={resendingId === row.id}
                        class="ml-1 px-2 py-1 rounded-lg text-xs text-gray-600 hover:text-gray-900 hover:bg-gray-100 transition-colors disabled:opacity-50">
                  {resendingId === row.id ? m.admin_smtp_log_resending() : m.admin_smtp_log_resend()}
                </button>
              </td>
            </tr>
            {#if expanded === row.id}
              <tr>
                <td colspan="7" class="px-6 py-4 bg-gray-50">
                  {#if row.failure_reason}
                    <div class="mb-3 p-3 rounded-lg bg-red-50 border border-red-100 text-xs text-red-700">
                      <span class="font-semibold">{m.admin_smtp_log_failure_reason()}：</span>
                      <span class="font-mono">{row.failure_reason}</span>
                    </div>
                  {/if}
                  <div class="mb-3 flex items-center gap-4 text-xs text-gray-600">
                    <span><span class="font-semibold">{m.admin_smtp_log_from()}：</span>{row.from_email}{row.from_name ? ` (${row.from_name})` : ''}</span>
                    {#if row.reply_to}<span><span class="font-semibold">{m.admin_smtp_log_reply_to()}：</span>{row.reply_to}</span>{/if}
                    <span><span class="font-semibold">{m.admin_smtp_log_subject_label()}：</span>{row.subject}</span>
                  </div>
                  <div class="flex items-center gap-2 mb-2">
                    <button onclick={() => (bodyTab[row.id] = 'html')}
                            class="px-2 py-1 rounded-lg text-xs {bodyTab[row.id] !== 'text' ? 'bg-gray-900 text-white' : 'bg-white border border-gray-200 text-gray-600'}">
                      HTML
                    </button>
                    <button onclick={() => (bodyTab[row.id] = 'text')}
                            class="px-2 py-1 rounded-lg text-xs {bodyTab[row.id] === 'text' ? 'bg-gray-900 text-white' : 'bg-white border border-gray-200 text-gray-600'}">
                      Plain text
                    </button>
                  </div>
                  {#if bodyTab[row.id] === 'text'}
                    <pre class="text-xs bg-white border border-gray-100 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap">{row.body_text}</pre>
                  {:else}
                    <div class="bg-white border border-gray-100 rounded-lg overflow-hidden">
                      <iframe srcdoc={row.body_html} title="email body" class="w-full h-96 bg-white" sandbox=""></iframe>
                    </div>
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
    <p class="text-xs text-gray-400">{m.admin_smtp_log_total({ total: data.list.total })}</p>
  {/if}

  <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.page} />
</div>
