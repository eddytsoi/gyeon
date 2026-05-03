<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import type { LayoutData } from '../$types';
  import { page } from '$app/stores';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // customer comes from the layout parent
  const customer = $derived(($page.data as LayoutData).customer);
  let loading = $state(false);
</script>

<svelte:head>
  <title>{m.account_profile_title()}</title>
</svelte:head>

<div class="bg-white rounded-2xl border border-gray-100 p-6">
  <h1 class="text-xl font-bold text-gray-900 mb-6">{m.account_profile_heading()}</h1>

  {#if form?.error}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {form.error}
    </div>
  {/if}
  {#if form?.success}
    <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">
      {m.account_profile_updated()}
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
        <label for="first_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_profile_first_name()}</label>
        <input
          id="first_name" name="first_name" type="text" required
          value={customer?.first_name ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="last_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_profile_last_name()}</label>
        <input
          id="last_name" name="last_name" type="text" required
          value={customer?.last_name ?? ''}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
    </div>
    <div>
      <label for="email" class="block text-sm font-medium text-gray-700 mb-1">{m.account_profile_email()}</label>
      <input
        id="email" type="email" disabled
        value={customer?.email ?? ''}
        class="w-full px-4 py-2.5 border border-gray-100 rounded-xl text-sm bg-gray-50 text-gray-400 cursor-not-allowed"
      />
    </div>
    <div>
      <label for="phone" class="block text-sm font-medium text-gray-700 mb-1">
        {m.account_profile_phone()} <span class="text-gray-400 font-normal">{m.common_optional()}</span>
      </label>
      <input
        id="phone" name="phone" type="tel"
        value={customer?.phone ?? ''}
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
      />
    </div>
    <div class="pt-2">
      <button
        type="submit"
        disabled={loading}
        class="px-6 py-2.5 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50"
      >
        {loading ? m.common_saving() : m.common_save_changes()}
      </button>
    </div>
  </form>
</div>
