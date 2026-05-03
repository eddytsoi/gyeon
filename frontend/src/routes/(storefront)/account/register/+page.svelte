<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { form }: { form: ActionData } = $props();
  let loading = $state(false);
</script>

<svelte:head>
  <title>{m.account_register_title()}</title>
</svelte:head>

<div class="min-h-[60vh] flex items-center justify-center px-4 py-12">
  <div class="w-full max-w-sm">
    <h1 class="text-2xl font-bold text-gray-900 mb-2 text-center">{m.account_register_heading()}</h1>
    <p class="text-sm text-gray-500 text-center mb-8">
      {m.account_register_have_account()}
      <a href="/account/login" class="text-gray-900 font-medium hover:underline">{m.account_register_login_link()}</a>
    </p>

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
      class="flex flex-col gap-4"
    >
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="first_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_register_first_name()}</label>
          <input
            id="first_name" name="first_name" type="text" required autocomplete="given-name"
            class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
        <div>
          <label for="last_name" class="block text-sm font-medium text-gray-700 mb-1">{m.account_register_last_name()}</label>
          <input
            id="last_name" name="last_name" type="text" required autocomplete="family-name"
            class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
          />
        </div>
      </div>
      <div>
        <label for="email" class="block text-sm font-medium text-gray-700 mb-1">{m.account_register_email()}</label>
        <input
          id="email" name="email" type="email" required autocomplete="email"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="phone" class="block text-sm font-medium text-gray-700 mb-1">
          {m.account_register_phone()} <span class="text-gray-400 font-normal">{m.common_optional()}</span>
        </label>
        <input
          id="phone" name="phone" type="tel" autocomplete="tel"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="password" class="block text-sm font-medium text-gray-700 mb-1">{m.account_register_password()}</label>
        <input
          id="password" name="password" type="password" required
          minlength="8" autocomplete="new-password"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
        <p class="mt-1 text-xs text-gray-400">{m.account_register_password_hint()}</p>
      </div>
      <button
        type="submit"
        disabled={loading}
        class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50"
      >
        {loading ? m.account_register_submitting() : m.account_register_submit()}
      </button>
    </form>
  </div>
</div>
