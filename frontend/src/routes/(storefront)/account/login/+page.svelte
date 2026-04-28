<script lang="ts">
  import { enhance } from '$app/forms';
  import { page } from '$app/state';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let loading = $state(false);

  let resetSuccess = $derived(page.url.searchParams.get('reset') === '1');

  let forgotOpen = $state(false);
  let forgotSending = $state(false);
  let forgotEmail = $state('');

  const forgotResult = $derived(form && 'forgot' in form ? form.forgot : null);

  function openForgot() {
    forgotEmail = '';
    forgotOpen = true;
  }

  function closeForgot() {
    if (forgotSending) return;
    forgotOpen = false;
  }
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

    {#if resetSuccess}
      <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">
        密碼已重設，請以新密碼登入。
      </div>
    {/if}

    {#if forgotResult?.sent}
      <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">
        若該電郵已註冊，重設密碼連結已寄出至 {forgotResult.email}。請查看您的信箱。
      </div>
    {/if}

    {#if form?.error}
      <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
        {form.error}
      </div>
    {/if}

    <form
      method="POST"
      action="?/login"
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
        <div class="flex items-center justify-between mb-1">
          <label for="password" class="block text-sm font-medium text-gray-700">Password</label>
          <button
            type="button"
            onclick={openForgot}
            class="text-xs font-medium text-gray-600 hover:text-gray-900 hover:underline"
          >
            Forgot password?
          </button>
        </div>
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

{#if forgotOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
    onclick={closeForgot}
    onkeydown={(e) => { if (e.key === 'Escape') closeForgot(); }}
    role="presentation"
  >
    <div
      class="bg-white rounded-2xl shadow-xl max-w-sm w-full p-6"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="forgot-pw-title"
      tabindex="-1"
    >
      <h3 id="forgot-pw-title" class="font-semibold text-gray-900 mb-2">忘記密碼</h3>
      <p class="text-sm text-gray-600 mb-4">
        請輸入您的註冊電郵，我們會寄出重設密碼連結。
      </p>

      {#if forgotResult?.error}
        <div class="mb-3 px-3 py-2 bg-red-50 border border-red-100 rounded-lg text-xs text-red-600">
          {forgotResult.error}
        </div>
      {/if}

      <form
        method="POST"
        action="?/forgotPassword"
        use:enhance={() => {
          forgotSending = true;
          return async ({ update }) => {
            await update({ reset: false });
            forgotSending = false;
            forgotOpen = false;
          };
        }}
      >
        <label for="forgot-email" class="block text-sm font-medium text-gray-700 mb-1">Email</label>
        <input
          id="forgot-email"
          name="email"
          type="email"
          required
          autocomplete="email"
          bind:value={forgotEmail}
          class="w-full px-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 mb-5"
        />
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onclick={closeForgot}
            disabled={forgotSending}
            class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={forgotSending}
            class="px-4 py-2 text-sm font-semibold text-white bg-gray-900 rounded-lg hover:bg-gray-700 disabled:opacity-50"
          >
            {forgotSending ? '寄送中…' : 'Reset Password'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
