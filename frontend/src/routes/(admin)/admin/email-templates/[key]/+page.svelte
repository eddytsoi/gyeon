<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult, notify } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import {
    adminTestEmailTemplate,
    adminPreviewEmailTemplate
  } from '$lib/api/admin';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
  const t = data.detail;
  const token = $derived(data.token ?? '');

  // Initial values: prefer override → fall back to compiled-in defaults
  let subject = $state(t.override?.subject ?? t.defaults.subject);
  let html = $state(t.override?.html ?? t.defaults.html);
  let text = $state(t.override?.text ?? t.defaults.text);
  let isEnabled = $state(t.override?.is_enabled ?? true);
  let saving = $state(false);

  let testEmail = $state('');
  let testing = $state(false);
  let previewHTML = $state('');
  let showingPreview = $state(false);
  let resetting = $state<HTMLFormElement | null>(null);

  async function sendTest() {
    if (!testEmail || testing) return;
    testing = true;
    try {
      await adminTestEmailTemplate(token, t.key, testEmail);
      notify.success(m.admin_email_templates_test_success({ email: testEmail }), '');
    } catch (e) {
      notify.error(m.admin_email_templates_test_failure(), e instanceof Error ? e.message : '');
    } finally {
      testing = false;
    }
  }

  async function showPreview() {
    try {
      const r = await adminPreviewEmailTemplate(token, t.key);
      previewHTML = r.html;
      showingPreview = true;
    } catch (e) {
      notify.error('Preview failed', e instanceof Error ? e.message : '');
    }
  }
</script>

<div class="max-w-5xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <a href="/admin/email-templates"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <div>
      <h2 class="text-xl font-bold text-gray-900">{t.display_name}</h2>
      <p class="text-xs text-gray-400 mt-0.5 font-mono">{t.key}</p>
    </div>
    {#if t.override}
      <span class="ml-auto inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-50 text-amber-700">{m.admin_email_templates_status_custom()}</span>
    {:else}
      <span class="ml-auto inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-500">{m.admin_email_templates_status_default()}</span>
    {/if}
  </div>

  <form method="POST" action="?/save" class="grid grid-cols-1 lg:grid-cols-3 gap-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          return async ({ result, update }) => {
            showResult(result, m.admin_email_templates_save_success(), m.admin_email_templates_save_failure());
            await update();
            saving = false;
          };
        }}>
    <!-- Editor -->
    <div class="lg:col-span-2 space-y-4">
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <label for="subject" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_email_templates_label_subject()}</label>
        <input id="subject" name="subject" type="text" bind:value={subject} required
               class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
      </div>

      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <label for="html" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_email_templates_label_html()}</label>
        <textarea id="html" name="html" bind:value={html} rows="20" required
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-xs font-mono leading-relaxed
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"></textarea>
      </div>

      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <label for="text" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_email_templates_label_text()}</label>
        <textarea id="text" name="text" bind:value={text} rows="8"
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-xs font-mono leading-relaxed
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"></textarea>
      </div>

      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 flex items-center gap-4">
        <label class="flex items-center gap-3 cursor-pointer select-none">
          <div class="relative">
            <input type="checkbox" class="sr-only peer" bind:checked={isEnabled} />
            <input type="hidden" name="is_enabled" value={isEnabled ? 'true' : 'false'} />
            <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
            <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                        transition-transform peer-checked:translate-x-4"></div>
          </div>
          <span class="text-sm font-medium text-gray-700">
            {isEnabled ? m.admin_email_templates_enabled() : m.admin_email_templates_disabled()}
          </span>
        </label>

        <div class="ml-auto flex gap-3">
          <a href="/admin/email-templates"
             class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                    text-gray-700 hover:bg-gray-50 transition-colors">
            {m.common_cancel()}
          </a>
          <SaveButton loading={saving}
                  class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                         text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
            {m.common_save_changes()}
          </SaveButton>
        </div>
      </div>
    </div>

    <!-- Sidebar -->
    <div class="space-y-4">
      <div class="bg-white rounded-2xl border border-gray-100 px-5 py-4">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-2">{m.admin_email_templates_variables()}</p>
        <p class="text-xs text-gray-500 mb-3">{m.admin_email_templates_variables_hint()}</p>
        <div class="flex flex-wrap gap-1.5">
          {#each t.variables as v}
            <code class="px-2 py-1 rounded-md bg-gray-100 text-gray-700 text-xs font-mono">{`{{${v}}}`}</code>
          {/each}
        </div>
      </div>

      <div class="bg-white rounded-2xl border border-gray-100 px-5 py-4 space-y-3">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_email_templates_actions()}</p>
        <button type="button" onclick={showPreview}
                class="w-full px-3 py-2 rounded-xl border border-gray-200 text-sm text-gray-700 hover:bg-gray-50 transition-colors">
          {m.admin_email_templates_preview()}
        </button>
        <div class="space-y-1.5">
          <input type="email" bind:value={testEmail} placeholder="you@example.com"
                 class="w-full px-3 py-2 rounded-xl border border-gray-200 text-xs
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
          <button type="button" onclick={sendTest} disabled={!testEmail || testing}
                  class="w-full px-3 py-2 rounded-xl border border-gray-200 text-sm text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50">
            {testing ? m.admin_email_templates_test_loading() : m.admin_email_templates_test_send()}
          </button>
        </div>
      </div>

      {#if t.override}
        <form method="POST" action="?/reset"
              use:enhance={() => async ({ result, update }) => {
                showResult(result, m.admin_email_templates_reset_success(), m.admin_email_templates_reset_failure());
                await update();
              }}>
          <button type="submit"
                  class="w-full px-3 py-2 rounded-xl border border-red-200 text-sm text-red-600 hover:bg-red-50 transition-colors">
            {m.admin_email_templates_reset()}
          </button>
        </form>
      {/if}
    </div>
  </form>
</div>

{#if showingPreview}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showingPreview = false} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-3xl max-h-[80vh] flex flex-col">
      <div class="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
        <h3 class="text-base font-bold text-gray-900">{m.admin_email_templates_preview()}</h3>
        <button onclick={() => showingPreview = false}
                class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>
      <div class="flex-1 overflow-y-auto">
        <iframe srcdoc={previewHTML} title="Email preview" class="w-full h-[60vh] border-0"></iframe>
      </div>
    </div>
  </div>
{/if}
