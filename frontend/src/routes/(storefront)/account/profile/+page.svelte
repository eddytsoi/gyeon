<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import type { LayoutData } from '../$types';
  import { page } from '$app/stores';
  import * as m from '$lib/paraglide/messages';
  import { siteName } from '$lib/seo';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // customer comes from the layout parent
  const customer = $derived(($page.data as LayoutData).customer);
  let loading = $state(false);
  let pwLoading = $state(false);
</script>

<svelte:head>
  <title>{m.account_profile_title({ brand: siteName(data.publicSettings) })}</title>
</svelte:head>

{#if (data.loyalty?.points ?? 0) > 0}
  <!-- P3 #24 — points balance card. Hidden until the customer has accrued any. -->
  <div class="bg-gradient-to-br from-gray-900 to-gray-700 rounded-2xl p-5 mb-6 text-white">
    <p class="text-xs uppercase tracking-widest opacity-60">{m.loyalty_balance_label()}</p>
    <p class="mt-1.5 text-3xl font-bold tabular-nums">{data.loyalty.points.toLocaleString()}</p>
    <p class="text-xs opacity-70 mt-1">{m.loyalty_balance_hint()}</p>
  </div>
{/if}

<div class="bg-white rounded-2xl border border-gray-100 p-6">
  <h1 class="text-xl font-bold text-gray-900 mb-6">{m.account_profile_heading()}</h1>

  {#if form?.profileError}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {form.profileError}
    </div>
  {/if}
  {#if form?.profileSuccess}
    <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">
      {m.account_profile_updated()}
    </div>
  {/if}

  <form
    method="POST"
    action="?/profile"
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

<div class="bg-white rounded-2xl border border-gray-100 p-6 mt-6">
  <h2 class="text-xl font-bold text-gray-900 mb-6">{m.account_password_heading()}</h2>

  {#if form?.passwordError}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {form.passwordError}
    </div>
  {/if}
  {#if form?.passwordSuccess}
    <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">
      {m.account_password_updated()}
    </div>
  {/if}

  <form
    method="POST"
    action="?/password"
    use:enhance={() => {
      pwLoading = true;
      return async ({ update }) => { await update(); pwLoading = false; };
    }}
    class="flex flex-col gap-4 max-w-md"
  >
    <div>
      <label for="current_password" class="block text-sm font-medium text-gray-700 mb-1">{m.account_password_current()}</label>
      <input
        id="current_password" name="current_password" type="password" required autocomplete="current-password"
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
      />
    </div>
    <div>
      <label for="new_password" class="block text-sm font-medium text-gray-700 mb-1">{m.account_password_new()}</label>
      <input
        id="new_password" name="new_password" type="password" required minlength="8" autocomplete="new-password"
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
      />
      <p class="mt-1 text-xs text-gray-400">{m.password_reset_hint()}</p>
    </div>
    <div>
      <label for="confirm" class="block text-sm font-medium text-gray-700 mb-1">{m.account_password_confirm()}</label>
      <input
        id="confirm" name="confirm" type="password" required minlength="8" autocomplete="new-password"
        class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
      />
    </div>
    <div class="pt-2">
      <button
        type="submit"
        disabled={pwLoading}
        class="px-6 py-2.5 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50"
      >
        {pwLoading ? m.common_saving() : m.account_password_submit()}
      </button>
    </div>
  </form>
</div>
