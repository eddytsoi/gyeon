<script lang="ts">
  import * as m from '$lib/paraglide/messages';

  let { googleEnabled = false, appleEnabled = false }: { googleEnabled?: boolean; appleEnabled?: boolean } =
    $props();

  // Plain top-level navigations to the backend OAuth start endpoint. The
  // backend redirects to the provider and, on callback, sets the
  // customer_token cookie itself. data-sveltekit-reload forces a full-page
  // navigation so the SvelteKit client router doesn't try to resolve
  // /api/v1/... as an app route.
</script>

{#if googleEnabled || appleEnabled}
  <div class="my-6 flex items-center gap-3" aria-hidden="true">
    <span class="h-px flex-1 bg-gray-200"></span>
    <span class="text-xs uppercase tracking-wide text-gray-400">{m.account_oauth_divider()}</span>
    <span class="h-px flex-1 bg-gray-200"></span>
  </div>

  <div class="flex flex-col gap-3">
    {#if googleEnabled}
      <a
        href="/api/v1/customers/oauth/google/start"
        data-sveltekit-reload
        class="flex w-full items-center justify-center gap-3 rounded-xl border border-gray-200 bg-white py-3 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-50"
      >
        <svg class="h-5 w-5" viewBox="0 0 18 18" aria-hidden="true">
          <path fill="#4285F4" d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844a4.14 4.14 0 0 1-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z"/>
          <path fill="#34A853" d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.583-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z"/>
          <path fill="#FBBC05" d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z"/>
          <path fill="#EA4335" d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z"/>
        </svg>
        {m.account_oauth_google()}
      </a>
    {/if}

    {#if appleEnabled}
      <a
        href="/api/v1/customers/oauth/apple/start"
        data-sveltekit-reload
        class="flex w-full items-center justify-center gap-3 rounded-xl border border-gray-200 bg-white py-3 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-50"
      >
        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M16.365 1.43c0 1.14-.417 2.2-1.12 2.99-.79.886-2.073 1.572-3.16 1.487a3.18 3.18 0 0 1-.024-.375c0-1.094.486-2.235 1.165-2.95.78-.83 2.116-1.452 3.119-1.488.013.246.02.49.02.736zM20.5 17.02c-.55 1.272-.815 1.84-1.524 2.964-.99 1.572-2.385 3.53-4.114 3.545-1.536.014-1.932-1.003-4.018-.99-2.086.011-2.52 1.008-4.058.994-1.729-.016-3.05-1.78-4.04-3.351-2.77-4.394-3.06-9.55-1.351-12.29 1.214-1.951 3.13-3.092 4.93-3.092 1.832 0 2.984 1.005 4.5 1.005 1.47 0 2.366-1.007 4.484-1.007 1.602 0 3.3.873 4.51 2.381-3.963 2.173-3.32 7.83.681 9.34z"/>
        </svg>
        {m.account_oauth_apple()}
      </a>
    {/if}
  </div>
{/if}
