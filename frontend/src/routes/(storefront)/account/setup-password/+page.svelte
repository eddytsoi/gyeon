<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let loading = $state(false);
</script>

<svelte:head>
  <title>{m.password_setup_title()}</title>
</svelte:head>

<div class="min-h-[60vh] flex items-center justify-center px-4 py-12">
  <div class="w-full max-w-sm">
    <h1 class="text-2xl font-bold text-gray-900 mb-2 text-center">{m.password_setup_heading()}</h1>
    <p class="text-sm text-gray-500 text-center mb-8">
      {m.password_setup_subheading()}
    </p>

    {#if !data.token}
      <div class="px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
        {m.password_setup_invalid_link()}
      </div>
    {:else}
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
        <input type="hidden" name="token" value={data.token} />

        <div>
          <label for="password" class="block text-sm font-medium text-gray-700 mb-1">{m.password_setup_new_label()}</label>
          <input id="password" name="password" type="password" required minlength="8" autocomplete="new-password"
                 class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
          <p class="mt-1 text-xs text-gray-400">{m.password_setup_hint()}</p>
        </div>

        <div>
          <label for="confirm" class="block text-sm font-medium text-gray-700 mb-1">{m.password_setup_confirm_label()}</label>
          <input id="confirm" name="confirm" type="password" required minlength="8" autocomplete="new-password"
                 class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>

        <button type="submit" disabled={loading}
                class="mt-2 w-full py-2.5 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
          {loading ? m.common_processing() : m.password_setup_submit()}
        </button>
      </form>
    {/if}
  </div>
</div>
