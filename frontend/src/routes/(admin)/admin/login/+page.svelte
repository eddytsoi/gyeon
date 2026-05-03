<script lang="ts">
  import type { ActionData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { form }: { form: ActionData } = $props();
  let loading = $state(false);
</script>

<svelte:head><title>{m.admin_login_title()}</title></svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
  <div class="w-full max-w-sm">
    <div class="text-center mb-8">
      <h1 class="text-2xl font-bold text-gray-900">{m.admin_login_brand()}</h1>
      <p class="text-sm text-gray-500 mt-1">{m.admin_login_subtitle()}</p>
    </div>

    <form method="POST" onsubmit={() => loading = true}
          class="bg-white rounded-2xl border border-gray-100 shadow-sm p-8 flex flex-col gap-5">
      {#if form?.error}
        <p class="text-sm text-red-500 bg-red-50 rounded-lg px-4 py-3">{form.error}</p>
      {/if}

      <div class="flex flex-col gap-1.5">
        <label for="email" class="text-sm font-medium text-gray-700">{m.admin_login_email_label()}</label>
        <input id="email" name="email" type="email" required autocomplete="email"
               class="w-full border border-gray-200 rounded-xl px-4 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                      placeholder:text-gray-300"
               placeholder="admin@example.com" />
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="password" class="text-sm font-medium text-gray-700">{m.admin_login_password_label()}</label>
        <input id="password" name="password" type="password" required autocomplete="current-password"
               class="w-full border border-gray-200 rounded-xl px-4 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                      placeholder:text-gray-300"
               placeholder="••••••••" />
      </div>

      <button type="submit" disabled={loading}
              class="w-full py-2.5 bg-gray-900 text-white font-semibold rounded-xl
                     hover:bg-gray-700 transition-colors disabled:opacity-60 text-sm">
        {loading ? m.admin_login_submitting() : m.admin_login_submit()}
      </button>
    </form>

    <p class="text-center text-xs text-gray-400 mt-4">
      <a href="/" class="hover:text-gray-700 transition-colors">{m.admin_login_back_to_store()}</a>
    </p>
  </div>
</div>
