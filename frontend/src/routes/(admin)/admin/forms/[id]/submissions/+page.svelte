<script lang="ts">
  import { enhance } from '$app/forms';
  import { browser } from '$app/environment';
  import type { PageData } from './$types';
  import type { FormSubmissionRow } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
  let viewing = $state<FormSubmissionRow | null>(null);

  // Build the export URL: the admin token lives in a cookie so a same-origin
  // fetch picks it up automatically when wrapped with credentials, but the
  // route is mounted under adminMW which expects the bearer header. Instead
  // we hit the endpoint via fetch with the token from a cookie-aware load
  // function would be cleaner; for now we do a client fetch with the token
  // read from page.data.token (not exposed) — so we fall back to the simpler
  // pattern: open a blob with token from cookies via a server-proxied route
  // is overkill. Use a tiny inline fetch that reads the token via document.
  async function downloadCsv() {
    if (!browser) return;
    const token = (document.cookie.match(/(?:^|;\s*)admin_token=([^;]+)/) || [])[1] || '';
    const res = await fetch(`/api/v1/admin/forms/${data.form.id}/submissions.csv`, {
      headers: { Authorization: `Bearer ${decodeURIComponent(token)}` }
    });
    if (!res.ok) {
      notify.error(m.admin_forms_submissions_export_failure());
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${data.form.slug}-submissions.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  function fmt(date: string): string {
    return new Date(date).toLocaleString();
  }
</script>

<svelte:head><title>{m.admin_forms_submissions_title({ title: data.form.title })}</title></svelte:head>

<div class="space-y-6">
  <div class="flex items-center gap-4">
    <a href="/admin/forms" class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100" aria-label={m.admin_forms_submissions_back_aria()}>
      ←
    </a>
    <div class="flex-1">
      <h2 class="text-xl font-bold text-gray-900">{data.form.title}</h2>
      <p class="text-sm text-gray-500 mt-0.5">{m.admin_forms_submissions_count({ n: data.submissions.total })}</p>
    </div>
    <a href="/admin/forms/{data.form.id}" class="text-sm text-gray-500 hover:text-gray-900 underline underline-offset-2">
      {m.admin_forms_submissions_edit_form()}
    </a>
    <button
      type="button"
      onclick={downloadCsv}
      class="rounded-xl bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800"
    >
      {m.admin_forms_submissions_export()}
    </button>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    {#if data.submissions.items.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <p class="text-sm font-medium text-gray-400">{m.admin_forms_submissions_empty()}</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_submissions_col_when()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_submissions_col_preview()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_submissions_col_mail()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_submissions_col_score()}</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.submissions.items as s (s.id)}
            <tr>
              <td class="px-6 py-3 text-xs text-gray-500 whitespace-nowrap">{fmt(s.created_at)}</td>
              <td class="px-6 py-3 text-xs text-gray-700 max-w-md truncate">
                {Object.entries(s.data).slice(0, 2).map(([k, v]) => `${k}: ${v}`).join(' · ')}
              </td>
              <td class="px-6 py-3 text-xs">
                {#if s.mail_sent}
                  <span class="text-emerald-600">{m.admin_forms_submissions_mail_sent()}</span>
                {:else}
                  <span class="text-red-500" title={s.mail_error}>{m.admin_forms_submissions_mail_failed()}</span>
                {/if}
              </td>
              <td class="px-6 py-3 text-xs text-gray-500">{s.recaptcha_score?.toFixed(2) ?? '—'}</td>
              <td class="px-6 py-3 text-right">
                <button onclick={() => (viewing = s)} class="text-sm text-gray-900 underline underline-offset-2">
                  {m.admin_forms_submissions_view()}
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  {#if data.submissions.total > data.submissions.items.length || data.offset > 0}
    <div class="flex justify-between items-center text-sm text-gray-500">
      <a
        href="?offset={Math.max(0, data.offset - data.limit)}&limit={data.limit}"
        class="hover:text-gray-900 {data.offset === 0 ? 'pointer-events-none opacity-40' : ''}"
      >
        {m.admin_forms_submissions_prev()}
      </a>
      <span>{m.admin_forms_submissions_offset({ offset: data.offset })}</span>
      <a
        href="?offset={data.offset + data.limit}&limit={data.limit}"
        class="hover:text-gray-900 {data.offset + data.limit >= data.submissions.total ? 'pointer-events-none opacity-40' : ''}"
      >
        {m.admin_forms_submissions_next()}
      </a>
    </div>
  {/if}
</div>

{#if viewing}
  <div
    class="fixed inset-0 z-40 bg-black/50 flex items-end md:items-center justify-center p-4"
    onclick={() => (viewing = null)}
    onkeydown={(e) => { if (e.key === 'Escape') viewing = null; }}
    role="button"
    tabindex="-1"
  >
    <div class="bg-white rounded-2xl p-6 max-w-xl w-full max-h-[80vh] overflow-auto" onclick={(e) => e.stopPropagation()} onkeydown={() => {}} role="dialog" tabindex="-1">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold text-gray-900">{m.admin_forms_submissions_detail_heading()}</h3>
        <button onclick={() => (viewing = null)} class="text-gray-400 hover:text-gray-700">✕</button>
      </div>
      <p class="text-xs text-gray-400 mb-4">
        {fmt(viewing.created_at)}{viewing.ip ? ' · IP ' + viewing.ip : ''}
      </p>
      <dl class="space-y-3">
        {#each Object.entries(viewing.data) as [k, v]}
          <div>
            <dt class="text-xs uppercase tracking-wide text-gray-400">{k}</dt>
            <dd class="text-sm text-gray-900 whitespace-pre-wrap">{v}</dd>
          </div>
        {/each}
      </dl>
      {#if viewing.mail_error}
        <div class="mt-4 rounded-xl border border-red-100 bg-red-50 px-3 py-2 text-xs text-red-700">
          {m.admin_forms_submissions_mail_error({ error: viewing.mail_error })}
        </div>
      {/if}
      <form
        method="POST"
        action="?/delete"
        use:enhance={() => {
          return async ({ result, update }) => {
            if (result.type === 'success') {
              notify.success(m.admin_forms_submissions_deleted());
              viewing = null;
              await update();
            } else {
              notify.error(m.admin_forms_submissions_delete_failure());
            }
          };
        }}
        class="mt-6 flex justify-end"
      >
        <input type="hidden" name="sid" value={viewing.id} />
        <button type="submit" class="text-sm text-red-500 hover:text-red-700">{m.admin_forms_submissions_delete()}</button>
      </form>
    </div>
  </div>
{/if}
