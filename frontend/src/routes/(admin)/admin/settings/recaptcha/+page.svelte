<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { notify } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();

  let enabled = $state(data.values.recaptcha_enabled === 'true');
  let siteKey = $state(data.values.recaptcha_site_key);
  let secretKey = $state(data.values.recaptcha_secret_key);
  let minScore = $state(data.values.recaptcha_min_score);
</script>

<svelte:head><title>reCAPTCHA · Admin</title></svelte:head>

<div class="max-w-2xl mx-auto space-y-6">
  <div>
    <h2 class="text-xl font-bold text-gray-900">Google reCAPTCHA v3</h2>
    <p class="text-sm text-gray-500 mt-0.5">
      Spam protection for contact forms. Get keys at
      <a href="https://www.google.com/recaptcha/admin" class="underline" target="_blank" rel="noopener">
        google.com/recaptcha/admin</a>.
    </p>
  </div>

  <form
    method="POST"
    action="?/save"
    use:enhance={() => async ({ result, update }) => {
      if (result.type === 'success') notify.success('reCAPTCHA settings saved');
      else notify.error('Save failed');
      await update();
    }}
    class="bg-white rounded-2xl border border-gray-100 p-6 space-y-4"
  >
    <label class="inline-flex items-center gap-2 text-sm">
      <input type="checkbox" name="recaptcha_enabled" value="true" bind:checked={enabled} class="rounded" />
      <span class="font-medium">Enable reCAPTCHA verification</span>
    </label>

    <div>
      <label for="site_key" class="block text-sm font-medium text-gray-700">Site key (public)</label>
      <input
        id="site_key"
        name="recaptcha_site_key"
        bind:value={siteKey}
        placeholder="6Lc..."
        class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
      />
    </div>

    <div>
      <label for="secret_key" class="block text-sm font-medium text-gray-700">Secret key (server-only)</label>
      <input
        id="secret_key"
        name="recaptcha_secret_key"
        type="password"
        bind:value={secretKey}
        placeholder="6Lc..."
        class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
      />
    </div>

    <div>
      <label for="min_score" class="block text-sm font-medium text-gray-700">Minimum score (0.0 – 1.0)</label>
      <input
        id="min_score"
        name="recaptcha_min_score"
        type="number"
        step="0.1"
        min="0"
        max="1"
        bind:value={minScore}
        class="mt-1 block w-32 rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
      />
      <p class="text-xs text-gray-400 mt-1">
        Submissions scoring below this are rejected. 0.5 is a sensible starting point.
      </p>
    </div>

    <div class="flex justify-end pt-2">
      <button type="submit" class="rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-semibold text-white hover:bg-gray-800">
        Save settings
      </button>
    </div>
  </form>
</div>
