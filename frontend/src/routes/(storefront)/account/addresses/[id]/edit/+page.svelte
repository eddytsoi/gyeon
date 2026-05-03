<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let loading = $state(false);
  let deleting = $state(false);

  const addr = $derived(form?.values ?? data.address);
  let country = $state((form?.values?.country ?? data.address.country) || data.shippingCountries[0] || 'HK');
  const cityListId = 'address-edit-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);

  // Ensure the saved country is selectable even if it's no longer in the configured list
  const countryOptions = $derived(
    data.shippingCountries.includes(addr.country)
      ? data.shippingCountries
      : [addr.country, ...data.shippingCountries]
  );
</script>

<svelte:head>
  <title>{m.account_address_edit_title()}</title>
</svelte:head>

<div class="bg-white rounded-2xl border border-gray-100 p-6">
  <div class="flex items-center gap-3 mb-6">
    <a href="/account/addresses" class="text-gray-400 hover:text-gray-700 transition-colors text-sm">{m.common_back_arrow()}</a>
    <h1 class="text-xl font-bold text-gray-900">{m.account_address_edit_heading()}</h1>
  </div>

  {#if form?.error}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {form.error}
    </div>
  {/if}

  <form
    method="POST"
    action="?/update"
    use:enhance={() => {
      loading = true;
      return async ({ update }) => { await update(); loading = false; };
    }}
    class="flex flex-col gap-4 max-w-md"
  >
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="first_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_first_name()} *</label>
        <input id="first_name" name="first_name" type="text" required value={addr.first_name}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div>
        <label for="last_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_last_name()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
        <input id="last_name" name="last_name" type="text" value={addr.last_name ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>
    <div>
      <label for="phone" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_phone()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
      <input id="phone" name="phone" type="tel" value={addr.phone ?? ''}
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </div>
    <div>
      <label for="line1" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_line1()} *</label>
      <input id="line1" name="line1" type="text" required value={addr.line1}
        placeholder={m.account_address_form_line1_placeholder()}
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </div>
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="city" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_city()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
        <input id="city" name="city" type="text" value={addr.city}
          list={cityOptions.length > 0 ? cityListId : undefined}
          placeholder={country === 'HK' ? m.checkout_address_city_placeholder_hk() : ''}
          autocomplete="off"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
        {#if cityOptions.length > 0}
          <datalist id={cityListId}>
            {#each cityOptions as opt}
              <option value={opt}></option>
            {/each}
          </datalist>
        {/if}
      </div>
      <div>
        <label for="postal_code" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_postal()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
        <input id="postal_code" name="postal_code" type="text" value={addr.postal_code}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="state" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_state()} <span class="text-gray-400 font-normal">{m.common_optional()}</span></label>
        <input id="state" name="state" type="text" value={addr.state ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div>
        <label for="country" class="block text-sm font-medium text-gray-700 mb-1">{m.account_address_form_country()} *</label>
        <select id="country" name="country" required bind:value={country}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm bg-white focus:outline-none focus:ring-2 focus:ring-gray-900">
          {#each countryOptions as code}
            <option value={code}>{COUNTRY_BY_CODE[code] ?? code} ({code})</option>
          {/each}
        </select>
      </div>
    </div>
    <label class="flex items-center gap-2 cursor-pointer">
      <input type="checkbox" name="is_default" checked={addr.is_default} class="rounded" />
      <span class="text-sm text-gray-700">{m.account_address_form_set_default()}</span>
    </label>
    <div class="pt-2 flex items-center gap-3">
      <button type="submit" disabled={loading}
        class="px-6 py-2.5 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
        {loading ? m.common_saving() : m.common_save_changes()}
      </button>
      <a href="/account/addresses"
        class="px-6 py-2.5 border border-gray-200 text-gray-700 font-medium rounded-xl hover:bg-gray-50 transition-colors text-sm">
        {m.common_cancel()}
      </a>
    </div>
  </form>

  <!-- Delete -->
  <div class="mt-8 pt-6 border-t border-gray-100">
    <form
      method="POST"
      action="?/delete"
      use:enhance={() => {
        deleting = true;
        return async ({ update }) => { await update(); deleting = false; };
      }}
    >
      <button
        type="submit"
        disabled={deleting}
        onclick={(e) => { if (!confirm(m.account_addresses_delete_confirm())) e.preventDefault(); }}
        class="text-sm text-red-500 hover:text-red-700 transition-colors disabled:opacity-50"
      >
        {deleting ? m.account_address_form_deleting() : m.account_address_form_delete()}
      </button>
    </form>
  </div>
</div>
