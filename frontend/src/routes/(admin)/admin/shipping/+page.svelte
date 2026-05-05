<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import { COUNTRIES } from '$lib/data/countries';
  import { showResult, notify } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import { adminTestShipanyConnection } from '$lib/api/admin';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
  const token = $derived(data.token ?? '');

  function val(key: string): string {
    return data.settings.find((s) => s.key === key)?.value ?? '';
  }

  let enabled = $state(val('shipany_enabled') === 'true');
  let userID = $state(val('shipany_user_id'));
  let apiKey = $state(val('shipany_api_key'));
  let region = $state(val('shipany_region') || 'HK');
  let webhookSecret = $state(val('shipany_webhook_secret'));

  function parseCountryList(raw: string | undefined): string[] {
    if (!raw) return ['HK'];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((v) => typeof v === 'string') : ['HK'];
    } catch {
      return ['HK'];
    }
  }

  let countries = $state<string[]>(parseCountryList(val('shipping_countries')));
  const countryOptions = COUNTRIES.map((c) => ({ value: c.code, label: `${c.name} (${c.code})` }));

  let saving = $state(false);
  let testing = $state(false);
  let showApiKey = $state(false);

  const REGION_OPTIONS = [
    { value: 'HK', label: 'Hong Kong (HK)' },
    { value: 'TW', label: 'Taiwan (TW)' },
    { value: 'SG', label: 'Singapore (SG)' },
    { value: 'MY', label: 'Malaysia (MY)' }
  ];

  async function testConnection() {
    if (testing) return;
    testing = true;
    try {
      const res = await adminTestShipanyConnection(token);
      if (res.ok) {
        notify.success(m.admin_shipping_test_success_title(), res.message || '');
      } else {
        notify.error(m.admin_shipping_test_failure_title(), res.message || '');
      }
    } catch (e) {
      notify.error(m.admin_shipping_test_failure_title(), e instanceof Error ? e.message : 'Network error');
    } finally {
      testing = false;
    }
  }
</script>

<div class="max-w-2xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <h2 class="text-xl font-bold text-gray-900">{m.admin_shipping_heading()}</h2>
  </div>
  <p class="text-sm text-gray-500 -mt-3">{m.admin_shipping_subtitle()}</p>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          return async ({ result, update }) => {
            showResult(result, m.admin_shipping_save_success(), m.admin_shipping_save_failure());
            await update();
            saving = false;
          };
        }}>
    <!-- Coverage -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-3">
      <div>
        <p class="text-sm font-semibold text-gray-900">{m.admin_shipping_countries_label()}</p>
        <p class="text-xs text-gray-400 mt-0.5">{m.admin_shipping_countries_hint()}</p>
      </div>
      <MultiSelect options={countryOptions} selected={countries}
                   onChange={(next) => countries = next}
                   placeholder={m.admin_shipping_countries_placeholder()} />
      <input type="hidden" name="shipping_countries" value={JSON.stringify(countries)} />
    </div>

    <!-- ShipAny provider -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <label class="flex items-center justify-between cursor-pointer select-none">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_shipping_shipany_enable_label()}</p>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_shipping_shipany_enable_hint()}</p>
        </div>
        <div class="relative">
          <input type="checkbox" class="sr-only peer" bind:checked={enabled} />
          <input type="hidden" name="shipany_enabled" value={enabled ? 'true' : 'false'} />
          <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
          <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                      transition-transform peer-checked:translate-x-4"></div>
        </div>
      </label>

      <div class:opacity-50={!enabled} class="space-y-4">
        <div>
          <label for="shipany_user_id" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_shipping_user_id_label()}</label>
          <input id="shipany_user_id" name="shipany_user_id" type="text" bind:value={userID} disabled={!enabled}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent disabled:bg-gray-50" />
        </div>
        <div>
          <label for="shipany_api_key" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_shipping_api_key_label()}</label>
          <div class="relative">
            <input id="shipany_api_key" name="shipany_api_key" type={showApiKey ? 'text' : 'password'}
                   bind:value={apiKey} disabled={!enabled}
                   class="w-full px-3.5 py-2.5 pr-20 rounded-xl border border-gray-200 text-sm font-mono
                          focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent disabled:bg-gray-50" />
            <button type="button" onclick={() => showApiKey = !showApiKey} disabled={!enabled}
                    class="absolute inset-y-0 right-2.5 my-1 px-2 rounded-lg text-xs text-gray-500 hover:bg-gray-100 transition-colors">
              {showApiKey ? m.admin_shipping_api_key_hide() : m.admin_shipping_api_key_show()}
            </button>
          </div>
        </div>
        <div>
          <label for="shipany_region" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_shipping_region_label()}</label>
          <select id="shipany_region" name="shipany_region" bind:value={region} disabled={!enabled}
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm bg-white
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent disabled:bg-gray-50">
            {#each REGION_OPTIONS as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
        </div>
        <div>
          <label for="shipany_webhook_secret" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_shipping_webhook_label()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
          </label>
          <input id="shipany_webhook_secret" name="shipany_webhook_secret" type="password" bind:value={webhookSecret} disabled={!enabled}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent disabled:bg-gray-50" />
          <p class="mt-1.5 text-xs text-gray-400">{m.admin_shipping_webhook_hint()}</p>
        </div>
        <button type="button" onclick={testConnection} disabled={!enabled || testing}
                class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl border border-gray-200 text-sm
                       text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50">
          {testing ? m.admin_shipping_test_loading() : m.admin_shipping_test_button()}
        </button>
      </div>
    </div>

    <!-- Advanced hint -->
    <div class="rounded-xl border border-blue-100 bg-blue-50/50 px-4 py-3 text-xs text-blue-900">
      {m.admin_shipping_advanced_pre()}<a href="/admin/settings#logistics" class="underline font-medium">{m.admin_shipping_advanced_link()}</a>{m.admin_shipping_advanced_post()}
    </div>

    <!-- Submit -->
    <div class="flex justify-end gap-3">
      <SaveButton loading={saving}
              class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                     text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
        {m.common_save_changes()}
      </SaveButton>
    </div>
  </form>
</div>
