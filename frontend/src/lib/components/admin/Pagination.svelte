<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    total: number;
    pageSize: number;
    /** Current page, 1-indexed. */
    currentPage: number;
    /** URL search-param name used to encode the page. */
    paramName?: string;
  }

  let { total, pageSize, currentPage, paramName = 'page' }: Props = $props();

  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  const safePage = $derived(Math.min(Math.max(1, currentPage), totalPages));

  // Build a compact list of page numbers with ellipses. We always show
  // the first and last, and a window of ±2 around the current page.
  const pages = $derived(buildPages(safePage, totalPages));

  function buildPages(cur: number, last: number): Array<number | 'ellipsis'> {
    if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);
    const out: Array<number | 'ellipsis'> = [1];
    const start = Math.max(2, cur - 2);
    const end = Math.min(last - 1, cur + 2);
    if (start > 2) out.push('ellipsis');
    for (let i = start; i <= end; i++) out.push(i);
    if (end < last - 1) out.push('ellipsis');
    out.push(last);
    return out;
  }

  function goTo(n: number) {
    if (n === safePage || n < 1 || n > totalPages) return;
    const url = new URL(page.url);
    if (n === 1) url.searchParams.delete(paramName);
    else url.searchParams.set(paramName, String(n));
    goto(url.pathname + url.search, { replaceState: false, keepFocus: true, noScroll: true });
  }
</script>

{#if totalPages > 1}
  <nav aria-label={m.admin_pagination_aria()}
       class="flex items-center justify-between gap-3 mt-4 text-sm">
    <p class="text-gray-500">
      {m.admin_pagination_status({ page: safePage, total: totalPages })}
    </p>

    <div class="flex items-center gap-1">
      <button type="button"
              onclick={() => goTo(safePage - 1)}
              disabled={safePage === 1}
              class="px-3 py-1.5 rounded-lg border border-gray-200 text-gray-700
                     hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed
                     transition-colors">
        {m.admin_pagination_prev()}
      </button>

      {#each pages as p}
        {#if p === 'ellipsis'}
          <span class="px-2 text-gray-400 select-none">…</span>
        {:else}
          <button type="button"
                  onclick={() => goTo(p)}
                  aria-current={p === safePage ? 'page' : undefined}
                  class="min-w-[2.25rem] px-3 py-1.5 rounded-lg border text-sm
                         transition-colors
                         {p === safePage
                           ? 'bg-gray-900 text-white border-gray-900'
                           : 'border-gray-200 text-gray-700 hover:bg-gray-50'}">
            {p}
          </button>
        {/if}
      {/each}

      <button type="button"
              onclick={() => goTo(safePage + 1)}
              disabled={safePage === totalPages}
              class="px-3 py-1.5 rounded-lg border border-gray-200 text-gray-700
                     hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed
                     transition-colors">
        {m.admin_pagination_next()}
      </button>
    </div>
  </nav>
{/if}
