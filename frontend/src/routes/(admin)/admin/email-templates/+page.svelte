<script lang="ts">
  import type { PageData } from './$types';
  import { spotlight } from '$lib/actions/spotlight';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head><title>{m.admin_email_templates_title()}</title></svelte:head>

<div class="space-y-6">
  <div>
    <h2 class="text-xl font-bold text-gray-900">{m.admin_email_templates_heading()}</h2>
    <p class="text-sm text-gray-500 mt-0.5">{m.admin_email_templates_subtitle()}</p>
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-gray-50">
          <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_email_templates_col_name()}</th>
          <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_email_templates_col_updated()}</th>
          <th class="px-6 py-3.5"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-50">
        {#each data.items as t}
          <tr class="js-row transition-colors">
            <td class="px-6 py-4 text-gray-900 font-medium">{t.display_name}</td>
            <td class="px-6 py-4 text-gray-400 text-xs">
              {t.updated_at ? new Date(t.updated_at).toLocaleString() : '—'}
            </td>
            <td class="px-6 py-4 text-right">
              <a href="/admin/email-templates/{t.key}"
                 class="inline-flex items-center justify-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium
                        bg-gray-900 text-white hover:bg-gray-700 transition-colors">
                {m.admin_email_templates_edit()}
              </a>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
