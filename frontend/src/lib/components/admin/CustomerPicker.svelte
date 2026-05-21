<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import { adminGetCustomers, adminGetCustomerAddresses } from '$lib/api/admin';
  import type { Customer, Address } from '$lib/types';

  // Discriminated union the page reads to know whether the admin picked an
  // existing customer (customer_id flow) or is entering guest details
  // (customer_info flow).
  export type CustomerSelection =
    | { kind: 'existing'; customer: Customer; addresses: Address[] }
    | { kind: 'guest'; firstName: string; lastName: string; email: string; phone: string }
    | { kind: 'none' };

  let { token, value = $bindable<CustomerSelection>({ kind: 'none' }) }: {
    token: string;
    value?: CustomerSelection;
  } = $props();

  // Search state — only meaningful while value.kind === 'none'
  let query = $state('');
  let results = $state<Customer[]>([]);
  let searching = $state(false);
  let searched = $state(false); // true once user has typed something
  let mode = $state<'search' | 'guest'>('search');

  // Guest form state — only meaningful while mode === 'guest'
  let gFirst = $state('');
  let gLast = $state('');
  let gEmail = $state('');
  let gPhone = $state('');

  let timer: ReturnType<typeof setTimeout> | undefined;

  function runSearch(q: string) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(async () => {
      const trimmed = q.trim();
      if (!trimmed) {
        results = [];
        searched = false;
        return;
      }
      searching = true;
      try {
        const res = await adminGetCustomers(token, 8, 0, trimmed);
        results = res.items ?? [];
      } catch {
        results = [];
      } finally {
        searching = false;
        searched = true;
      }
    }, 300);
  }

  function onQueryInput(e: Event) {
    query = (e.currentTarget as HTMLInputElement).value;
    runSearch(query);
  }

  async function pickExisting(c: Customer) {
    let addresses: Address[] = [];
    try {
      addresses = await adminGetCustomerAddresses(token, c.id);
    } catch {
      // non-fatal — admin can still type a new address
    }
    value = { kind: 'existing', customer: c, addresses };
  }

  function reset() {
    value = { kind: 'none' };
    mode = 'search';
    query = '';
    results = [];
    searched = false;
    gFirst = gLast = gEmail = gPhone = '';
  }

  // Sync guest form → bound value so the parent's summary updates live.
  function syncGuest() {
    if (mode !== 'guest') return;
    value = {
      kind: 'guest',
      firstName: gFirst,
      lastName: gLast,
      email: gEmail,
      phone: gPhone
    };
  }
  $effect(syncGuest);
</script>

{#if value.kind === 'existing'}
  <!-- Selected card -->
  <div class="flex items-start justify-between gap-3">
    <div class="flex-1 min-w-0">
      <p class="font-medium text-gray-900 truncate">
        {value.customer.first_name} {value.customer.last_name}
      </p>
      <p class="text-xs text-gray-500 truncate">
        {value.customer.email}{value.customer.phone ? ` · ${value.customer.phone}` : ''}
      </p>
    </div>
    <button type="button" onclick={reset}
            class="text-xs font-medium text-gray-500 hover:text-gray-900 transition-colors">
      {m.admin_order_create_customer_change()}
    </button>
  </div>
{:else if mode === 'guest'}
  <!-- Guest inline form -->
  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
    <label class="flex flex-col gap-1.5">
      <span class="text-xs font-medium text-gray-600">{m.admin_order_create_customer_first_name()}</span>
      <input type="text" bind:value={gFirst}
             class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </label>
    <label class="flex flex-col gap-1.5">
      <span class="text-xs font-medium text-gray-600">{m.admin_order_create_customer_last_name()}</span>
      <input type="text" bind:value={gLast}
             class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </label>
    <label class="flex flex-col gap-1.5">
      <span class="text-xs font-medium text-gray-600">{m.admin_order_create_customer_email()}</span>
      <input type="email" bind:value={gEmail}
             class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </label>
    <label class="flex flex-col gap-1.5">
      <span class="text-xs font-medium text-gray-600">{m.admin_order_create_customer_phone()}</span>
      <input type="tel" bind:value={gPhone}
             class="border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </label>
  </div>
  <button type="button" onclick={() => { mode = 'search'; value = { kind: 'none' }; }}
          class="mt-3 text-xs font-medium text-gray-500 hover:text-gray-900 transition-colors">
    {m.admin_order_create_customer_back_search()}
  </button>
{:else}
  <!-- Search mode -->
  <div class="relative">
    <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-400">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
        <path stroke-linecap="round" stroke-linejoin="round"
              d="m21 21-4.3-4.3M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z" />
      </svg>
    </span>
    <input
      type="search"
      value={query}
      oninput={onQueryInput}
      placeholder={m.admin_order_create_customer_search_placeholder()}
      autocomplete="off"
      class="w-full pl-9 pr-3 py-2 text-sm rounded-xl border border-gray-200 bg-white
             focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900
             placeholder:text-gray-400" />
  </div>

  {#if query.trim() !== '' && (searching || results.length > 0 || searched)}
    <div class="mt-2 border border-gray-200 rounded-xl bg-white overflow-hidden">
      {#if searching && results.length === 0}
        <div class="px-3 py-2 text-xs text-gray-400">…</div>
      {:else if results.length === 0}
        <div class="px-3 py-2 text-xs text-gray-400">{m.admin_order_create_customer_no_results()}</div>
      {:else}
        <ul class="divide-y divide-gray-100">
          {#each results as c (c.id)}
            <li>
              <button type="button" onclick={() => pickExisting(c)}
                      class="w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors">
                <p class="text-sm font-medium text-gray-900 truncate">
                  {c.first_name} {c.last_name}
                </p>
                <p class="text-xs text-gray-500 truncate">
                  {c.email}{c.phone ? ` · ${c.phone}` : ''}
                </p>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}

  <button type="button" onclick={() => { mode = 'guest'; }}
          class="mt-3 text-xs font-medium text-gray-700 hover:text-gray-900 transition-colors">
    {m.admin_order_create_customer_use_guest()}
  </button>
{/if}
