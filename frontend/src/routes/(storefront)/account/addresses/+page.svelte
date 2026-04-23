<script lang="ts">
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>My Addresses — Gyeon</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-bold text-gray-900">Addresses</h1>
    <a
      href="/account/addresses/new"
      class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors"
    >
      + Add address
    </a>
  </div>

  {#if data.addresses.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">No saved addresses yet.</p>
      <a
        href="/account/addresses/new"
        class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline"
      >
        Add your first address →
      </a>
    </div>
  {:else}
    <div class="grid sm:grid-cols-2 gap-4">
      {#each data.addresses as addr}
        <div class="bg-white rounded-2xl border border-gray-100 p-5 flex flex-col gap-3">
          <div class="flex items-start justify-between">
            <div>
              <p class="font-medium text-gray-900">{addr.first_name} {addr.last_name}</p>
              {#if addr.is_default}
                <span class="inline-block mt-1 px-2 py-0.5 bg-gray-100 text-gray-600 text-xs rounded-full">Default</span>
              {/if}
            </div>
            <a
              href="/account/addresses/{addr.id}/edit"
              class="text-xs text-gray-400 hover:text-gray-700 transition-colors"
            >
              Edit
            </a>
          </div>
          <address class="not-italic text-sm text-gray-600 leading-relaxed">
            {addr.line1}{#if addr.line2}, {addr.line2}{/if}<br />
            {addr.city}{#if addr.state}, {addr.state}{/if} {addr.postal_code}<br />
            {addr.country}
          </address>
          {#if addr.phone}
            <p class="text-sm text-gray-500">{addr.phone}</p>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>
