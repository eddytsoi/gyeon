<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let loading = $state(false);
</script>

<svelte:head>
  <title>Sign In — Gyeon</title>
</svelte:head>

<div class="min-h-[60vh] flex items-center justify-center px-4 py-12">
  <div class="w-full max-w-sm">
    <h1 class="text-2xl font-bold text-gray-900 mb-2 text-center">Sign in</h1>
    <p class="text-sm text-gray-500 text-center mb-8">
      Don't have an account?
      <a href="/account/register" class="text-gray-900 font-medium hover:underline">Register</a>
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
      <div>
        <label for="email" class="block text-sm font-medium text-gray-700 mb-1">Email</label>
        <input
          id="email" name="email" type="email" required autocomplete="email"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="password" class="block text-sm font-medium text-gray-700 mb-1">Password</label>
        <input
          id="password" name="password" type="password" required autocomplete="current-password"
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <button
        type="submit"
        disabled={loading}
        class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50"
      >
        {loading ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  </div>
</div>
