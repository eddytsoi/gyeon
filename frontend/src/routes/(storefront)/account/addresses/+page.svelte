<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.account_addresses_title()}</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-bold text-gray-900">{m.account_addresses_heading()}</h1>
    <a
      href="/account/addresses/new"
      class="px-4 py-2 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors"
    >
      {m.account_addresses_add()}
    </a>
  </div>

  {#if data.addresses.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">{m.account_addresses_empty()}</p>
      <a
        href="/account/addresses/new"
        class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline"
      >
        {m.account_addresses_add_first()}
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
                <span class="inline-block mt-1 px-2 py-0.5 bg-gray-100 text-gray-600 text-xs rounded-full">{m.common_default()}</span>
              {/if}
            </div>
            <div class="flex items-center gap-2">
              <a
                href="/account/addresses/{addr.id}/edit"
                aria-label={m.account_addresses_aria_edit()}
                title={m.common_edit()}
                class="text-gray-400 hover:text-gray-700 transition-colors p-1 -m-1"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
                     stroke="currentColor" stroke-width="2"
                     stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                  <path d="M16.862 4.487l1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.862 4.487Z" />
                  <path d="M19.5 7.125l-2.625-2.625" />
                </svg>
              </a>
              <form
                method="POST"
                action="?/delete"
                use:enhance={() => {
                  return async ({ update }) => { await update(); };
                }}
              >
                <input type="hidden" name="id" value={addr.id} />
                <button
                  type="submit"
                  aria-label={m.account_addresses_aria_delete()}
                  title={m.common_delete()}
                  onclick={(e) => { if (!confirm(m.account_addresses_delete_confirm())) e.preventDefault(); }}
                  class="text-gray-400 hover:text-red-600 transition-colors p-1 -m-1"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
                       stroke="currentColor" stroke-width="2"
                       stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <path d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                  </svg>
                </button>
              </form>
            </div>
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
