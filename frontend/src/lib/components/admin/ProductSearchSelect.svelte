<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import { adminGetProducts } from '$lib/api/admin';
  import type { AdminProductRow } from '$lib/api/admin';

  // Product-only search box (no variant/qty). Clicking a result fires
  // onSelect with the picked product and clears the query — used by the
  // 優惠套裝 picker, where the association is product-to-product. Mirrors the
  // search half of ProductPicker.svelte for a consistent look.
  let { token, kind = '', excludeIds = [], onSelect, placeholder }: {
    token: string;
    kind?: string;
    excludeIds?: string[];
    onSelect: (product: AdminProductRow) => void;
    placeholder?: string;
  } = $props();

  let query = $state('');
  let results = $state<AdminProductRow[]>([]);
  let searching = $state(false);
  let searched = $state(false);

  const visibleResults = $derived(results.filter((p) => !excludeIds.includes(p.id)));

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
        const res = await adminGetProducts(token, 8, 0, trimmed, '', kind);
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

  function pick(p: AdminProductRow) {
    onSelect(p);
    query = '';
    results = [];
    searched = false;
  }
</script>

<!-- Search input -->
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
    placeholder={placeholder ?? m.admin_order_create_items_search_placeholder()}
    autocomplete="off"
    class="w-full pl-9 pr-3 py-2 text-sm rounded-xl border border-gray-200 bg-white
           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900
           placeholder:text-gray-400" />
</div>

<!-- Search dropdown -->
{#if query.trim() !== '' && (searching || visibleResults.length > 0 || searched)}
  <div class="mt-2 border border-gray-200 rounded-xl bg-white overflow-hidden">
    {#if searching && visibleResults.length === 0}
      <div class="px-3 py-2 text-xs text-gray-400">…</div>
    {:else if visibleResults.length === 0}
      <div class="px-3 py-2 text-xs text-gray-400">{m.admin_order_create_items_no_results()}</div>
    {:else}
      <ul class="divide-y divide-gray-100 max-h-80 overflow-y-auto">
        {#each visibleResults as p (p.id)}
          {@const price = p.default_variant_price ?? p.min_price ?? null}
          {@const compareAt = p.default_variant_compare_at_price ?? p.min_compare_at_price ?? null}
          <li>
            <button type="button" onclick={() => pick(p)}
                    class="w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors flex items-center gap-3">
              {#if p.primary_image_url}
                <img src={p.primary_image_url} alt="" class="w-10 h-10 rounded-lg object-cover bg-gray-100 flex-shrink-0" />
              {:else}
                <div class="w-10 h-10 rounded-lg bg-gray-100 flex-shrink-0"></div>
              {/if}
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium text-gray-900 truncate">{p.name}</p>
                <p class="text-xs text-gray-500 truncate">
                  {#if price != null}
                    {#if compareAt != null && compareAt > price}
                      <span class="text-gray-400 line-through">HK${compareAt.toFixed(2)}</span>
                      <span class="text-red-600 font-medium ml-1.5">HK${price.toFixed(2)}</span>
                    {:else}
                      <span>HK${price.toFixed(2)}</span>
                    {/if}
                  {:else}
                    <span class="font-mono">{p.slug}</span>
                  {/if}
                </p>
              </div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
