<script lang="ts">
  import '../app.css';
  import { page } from '$app/state';
  import { onMount } from 'svelte';

  // When a load() fails offline (typically the SPA __data.json fetch on
  // PWA installs that have lost network), trigger a full document reload.
  // The service worker's navigate-mode handler will then serve the cached
  // /offline.html as the fallback while preserving the user's intended
  // URL — so when they click Retry once back online, reload() re-requests
  // the page they actually wanted, not /offline.html (which is a real
  // static file and would just serve the offline shell again).
  onMount(() => {
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      window.location.reload();
    }
  });
</script>

<svelte:head>
  <title>Error · Gyeon</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 px-6">
  <div class="max-w-md text-center">
    <p class="text-sm font-semibold text-gray-500">{page.status}</p>
    <h1 class="mt-2 text-2xl font-semibold text-gray-900">
      {page.error?.message || '發生錯誤 / Something went wrong'}
    </h1>
    <p class="mt-3 text-sm text-gray-600">
      請稍後再試，或返回首頁。<br />Please try again, or return to the homepage.
    </p>
    <div class="mt-6">
      <a
        href="/"
        class="inline-flex items-center rounded-lg bg-gray-900 px-5 py-2.5 text-sm font-semibold
               text-white hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-gray-900
               focus:ring-offset-2"
      >
        返回首頁 / Home
      </a>
    </div>
  </div>
</div>
