<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const r = data.redirect;
  const isNew = !r;
  let saving = $state(false);

  let fromPath = $state(r?.from_path ?? '');
  let toPath = $state(r?.to_path ?? '');
  let code = $state<301 | 302>(r?.code ?? 301);
  let isActive = $state<boolean>(r?.is_active ?? true);
  let note = $state(r?.note ?? '');
</script>

<div class="max-w-2xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <a href="/admin/redirects"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900">
      {isNew ? m.admin_redirects_new_heading() : m.admin_redirects_edit_heading()}
    </h2>
  </div>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          const fp = fromPath;
          return async ({ result, update }) => {
            showResult(result,
              isNew ? m.admin_redirects_create_success({ path: fp }) : m.admin_redirects_save_success({ path: fp }),
              isNew ? m.admin_redirects_create_failure({ path: fp }) : m.admin_redirects_save_failure({ path: fp }));
            await update();
            saving = false;
          };
        }}>
    <!-- Paths -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_redirects_label_from()}</label>
        <input type="text" name="from_path" bind:value={fromPath} required placeholder="/old-page"
               class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
        <p class="mt-1.5 text-xs text-gray-400">{m.admin_redirects_help_from()}</p>
      </div>
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_redirects_label_to()}</label>
        <input type="text" name="to_path" bind:value={toPath} required placeholder="/new-page"
               class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
        <p class="mt-1.5 text-xs text-gray-400">{m.admin_redirects_help_to()}</p>
      </div>
    </div>

    <!-- Behavior -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_redirects_label_code()}</label>
        <select name="code" bind:value={code}
                class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
          <option value={301}>{m.admin_redirects_code_301()}</option>
          <option value={302}>{m.admin_redirects_code_302()}</option>
        </select>
      </div>
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
          {m.admin_redirects_label_note()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
        </label>
        <textarea name="note" bind:value={note} rows="2"
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm resize-none
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"></textarea>
      </div>
    </div>

    <!-- Active + submit -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 flex flex-col sm:flex-row sm:items-center gap-4">
      <label class="flex items-center gap-3 cursor-pointer select-none">
        <div class="relative">
          <input type="checkbox" class="sr-only peer" bind:checked={isActive} />
          <input type="hidden" name="is_active" value={isActive ? 'true' : 'false'} />
          <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
          <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                      transition-transform peer-checked:translate-x-4"></div>
        </div>
        <span class="text-sm font-medium text-gray-700">
          {isActive ? m.admin_redirects_status_active() : m.admin_redirects_status_inactive()}
        </span>
      </label>
      <div class="sm:ml-auto flex gap-3">
        <a href="/admin/redirects"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          {m.common_cancel()}
        </a>
        <SaveButton loading={saving}
                class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                       text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
          {isNew ? m.admin_redirects_create_button() : m.common_save_changes()}
        </SaveButton>
      </div>
    </div>
  </form>
</div>
