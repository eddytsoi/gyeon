<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import type { AdminForm } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
  let deleteTarget = $state<AdminForm | null>(null);
</script>

<svelte:head><title>{m.admin_forms_page_title()}</title></svelte:head>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">{m.admin_forms_heading()}</h2>
      <p class="text-sm text-gray-500 mt-0.5">
        {data.forms.length === 1
          ? m.admin_forms_count_one()
          : m.admin_forms_count_other({ n: data.forms.length })}
      </p>
    </div>
    <a
      href="/admin/forms/new"
      class="inline-flex items-center gap-2 rounded-xl bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800"
    >
      {m.admin_forms_new()}
    </a>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
    {#if data.forms.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <p class="text-sm font-medium text-gray-400">{m.admin_forms_empty()}</p>
        <a href="/admin/forms/new" class="mt-3 text-sm text-gray-900 underline underline-offset-2">
          {m.admin_forms_create_first()}
        </a>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-50">
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_col_title()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_col_slug()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_col_fields()}</th>
            <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_forms_col_updated()}</th>
            <th class="px-6 py-3.5"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.forms as form (form.id)}
            <tr>
              <td class="px-6 py-4">
                <p class="font-medium text-gray-900">{form.title}</p>
                <p class="text-xs text-gray-400 font-mono mt-0.5 select-all">
                  [contact-form id="{form.slug}"]
                </p>
              </td>
              <td class="px-6 py-4 text-gray-500 font-mono text-xs">/{form.slug}</td>
              <td class="px-6 py-4 text-gray-500">{form.fields?.length ?? 0}</td>
              <td class="px-6 py-4 text-gray-400 text-xs">
                {new Date(form.updated_at).toLocaleDateString()}
              </td>
              <td class="px-6 py-4 text-right space-x-3">
                <a href="/admin/forms/{form.id}/submissions" class="text-sm text-gray-500 hover:text-gray-900">
                  {m.admin_forms_submissions()}
                </a>
                <a href="/admin/forms/{form.id}" class="text-sm text-gray-900 underline underline-offset-2">
                  {m.admin_forms_edit()}
                </a>
                <button
                  onclick={() => (deleteTarget = form)}
                  class="text-sm text-red-500 hover:text-red-700"
                >
                  {m.admin_forms_delete()}
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

{#if deleteTarget}
  <div class="fixed inset-0 z-40 bg-black/50 flex items-center justify-center p-4" onclick={() => (deleteTarget = null)} onkeydown={() => {}} role="button" tabindex="-1">
    <div class="bg-white rounded-2xl p-6 max-w-md w-full" onclick={(e) => e.stopPropagation()} onkeydown={() => {}} role="dialog" tabindex="-1">
      <h3 class="text-lg font-semibold text-gray-900">
        {m.admin_forms_delete_title({ title: deleteTarget.title })}
      </h3>
      <p class="text-sm text-gray-500 mt-2">
        {m.admin_forms_delete_body()}
      </p>
      <form
        method="POST"
        action="?/delete"
        use:enhance={() => {
          return async ({ result, update }) => {
            if (result.type === 'failure' || result.type === 'error') {
              notify.error(m.admin_forms_delete_failure());
            } else {
              notify.success(m.admin_forms_deleted_success());
            }
            deleteTarget = null;
            await update();
          };
        }}
        class="mt-4 flex justify-end gap-2"
      >
        <input type="hidden" name="id" value={deleteTarget.id} />
        <button type="button" onclick={() => (deleteTarget = null)} class="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
          {m.admin_forms_cancel()}
        </button>
        <button type="submit" class="px-4 py-2 rounded-xl bg-red-600 text-white text-sm font-semibold hover:bg-red-700">
          {m.admin_forms_delete()}
        </button>
      </form>
    </div>
  </div>
{/if}
