<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData } from './$types';
  import { spotlight } from '$lib/actions/spotlight';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';

  let { data }: { data: PageData } = $props();

  function onSearch(q: string) {
    const url = new URL(page.url);
    if (q) url.searchParams.set('q', q);
    else url.searchParams.delete('q');
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }
</script>

<svelte:head><title>Customers — Gyeon Admin</title></svelte:head>

<div class="max-w-5xl">
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold text-gray-900">Customers</h1>
    <span class="text-sm text-gray-400">{data.customers.length} {data.q ? 'match' + (data.customers.length === 1 ? '' : 'es') : 'total'}</span>
  </div>

  <div class="mb-4">
    <SearchInput value={data.q} placeholder="Search by name, email or phone…" onChange={onSearch} />
  </div>

  <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
       use:spotlight={{ selector: '.js-row' }}>
    {#if data.customers.length === 0}
      <div class="flex flex-col items-center justify-center py-16 text-center">
        <div class="w-12 h-12 rounded-2xl bg-gray-100 flex items-center justify-center mb-4">
          <svg class="w-6 h-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
          </svg>
        </div>
        {#if data.q}
          <p class="font-medium text-gray-900 mb-1">No matches for "{data.q}"</p>
          <p class="text-sm text-gray-400">Try a different name, email or phone fragment.</p>
        {:else}
          <p class="font-medium text-gray-900 mb-1">No customers yet</p>
          <p class="text-sm text-gray-400">Customers will appear here after they register.</p>
        {/if}
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-100">
          <tr>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Customer</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">Email</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden md:table-cell">Joined</th>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Status</th>
            <th class="px-5 py-3"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.customers as customer}
            <tr class="js-row transition-colors">
              <td class="px-5 py-3">
                <p class="font-medium text-gray-900">{customer.first_name} {customer.last_name}</p>
                <p class="text-xs text-gray-400 sm:hidden">{customer.email}</p>
              </td>
              <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">{customer.email}</td>
              <td class="px-5 py-3 text-gray-400 text-xs hidden md:table-cell">
                {new Date(customer.created_at).toLocaleDateString('en-HK')}
              </td>
              <td class="px-5 py-3">
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                             {customer.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
                  {customer.is_active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td class="px-5 py-3 text-right">
                <a href="/admin/customers/{customer.id}"
                   class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                  View
                </a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>
