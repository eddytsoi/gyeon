<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import { COUNTRY_BY_CODE } from '$lib/data/countries';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let loading = $state(false);

  let country = $state(form?.values?.country ?? data.shippingCountries[0] ?? 'HK');
  const cityListId = 'address-new-city-options';
  const cityOptions = $derived(country === 'HK' ? HK_DISTRICTS : []);
</script>

<svelte:head>
  <title>Add Address — Gyeon</title>
</svelte:head>

<div class="bg-white rounded-2xl border border-gray-100 p-6">
  <div class="flex items-center gap-3 mb-6">
    <a href="/account/addresses" class="text-gray-400 hover:text-gray-700 transition-colors text-sm">← Back</a>
    <h1 class="text-xl font-bold text-gray-900">Add Address</h1>
  </div>

  {#if form?.error}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {form.error}
    </div>
  {/if}

  <form
    method="POST"
    use:enhance={() => {
      loading = true;
      return async ({ update }) => { await update(); loading = false; };
    }}
    class="flex flex-col gap-4 max-w-md"
  >
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="first_name" class="block text-sm font-medium text-gray-700 mb-1">First name *</label>
        <input id="first_name" name="first_name" type="text" required value={form?.values?.first_name ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div>
        <label for="last_name" class="block text-sm font-medium text-gray-700 mb-1">Last name <span class="text-gray-400 font-normal">(optional)</span></label>
        <input id="last_name" name="last_name" type="text" value={form?.values?.last_name ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>
    <div>
      <label for="phone" class="block text-sm font-medium text-gray-700 mb-1">Phone <span class="text-gray-400 font-normal">(optional)</span></label>
      <input id="phone" name="phone" type="tel" value={form?.values?.phone ?? ''}
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </div>
    <div>
      <label for="line1" class="block text-sm font-medium text-gray-700 mb-1">詳細地址 *</label>
      <input id="line1" name="line1" type="text" required value={form?.values?.line1 ?? ''}
        placeholder="街道、門牌、樓層、單位"
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </div>
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="city" class="block text-sm font-medium text-gray-700 mb-1">City / District <span class="text-gray-400 font-normal">(optional)</span></label>
        <input id="city" name="city" type="text" value={form?.values?.city ?? ''}
          list={cityOptions.length > 0 ? cityListId : undefined}
          placeholder={country === 'HK' ? '例：九龍城區' : ''}
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
        <label for="postal_code" class="block text-sm font-medium text-gray-700 mb-1">Postal code <span class="text-gray-400 font-normal">(optional)</span></label>
        <input id="postal_code" name="postal_code" type="text" value={form?.values?.postal_code ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label for="state" class="block text-sm font-medium text-gray-700 mb-1">State / Region <span class="text-gray-400 font-normal">(optional)</span></label>
        <input id="state" name="state" type="text" value={form?.values?.state ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
      <div>
        <label for="country" class="block text-sm font-medium text-gray-700 mb-1">Country *</label>
        <select id="country" name="country" required bind:value={country}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm bg-white focus:outline-none focus:ring-2 focus:ring-gray-900">
          {#each data.shippingCountries as code}
            <option value={code}>{COUNTRY_BY_CODE[code] ?? code} ({code})</option>
          {/each}
        </select>
      </div>
    </div>
    <label class="flex items-center gap-2 cursor-pointer">
      <input type="checkbox" name="is_default" class="rounded" />
      <span class="text-sm text-gray-700">Set as default address</span>
    </label>
    <div class="pt-2 flex gap-3">
      <button type="submit" disabled={loading}
        class="px-6 py-2.5 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
        {loading ? 'Saving…' : 'Save address'}
      </button>
      <a href="/account/addresses"
        class="px-6 py-2.5 border border-gray-200 text-gray-700 font-medium rounded-xl hover:bg-gray-50 transition-colors text-sm">
        Cancel
      </a>
    </div>
  </form>
</div>
