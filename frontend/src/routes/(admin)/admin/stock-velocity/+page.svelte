<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const DEFAULT_SORT = 'gross_sold_desc';

  // Sortable columns. `defDir` is the direction applied the first time a column
  // is clicked (numerics → high-to-low; variation & days-left → ascending, so
  // the soonest-to-run-out variant surfaces first).
  const COLS = [
    { key: 'variation',        label: m.admin_stock_velocity_col_variation(),        align: 'left',  defDir: 'asc'  },
    { key: 'stock',            label: m.admin_stock_velocity_col_stock(),            align: 'right', defDir: 'desc' },
    { key: 'gross_sales',      label: m.admin_stock_velocity_col_gross_sales(),      align: 'right', defDir: 'desc' },
    { key: 'gross_sold',       label: m.admin_stock_velocity_col_gross_sold(),       align: 'right', defDir: 'desc' },
    { key: 'daily_gross_sold', label: m.admin_stock_velocity_col_daily_gross_sold(), align: 'right', defDir: 'desc' },
    { key: 'days_left',        label: m.admin_stock_velocity_col_days_left(),        align: 'right', defDir: 'asc'  },
  ] as const;

  const activeKey = $derived(data.sort.slice(0, data.sort.lastIndexOf('_')));
  const activeDir = $derived(data.sort.slice(data.sort.lastIndexOf('_') + 1));

  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL(page.url);
    mutate(url.searchParams);
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  function setDays(n: number) {
    pushParams((p) => { n === 30 ? p.delete('days') : p.set('days', String(n)); });
  }

  function sortBy(key: string, defDir: string) {
    const dir = activeKey === key ? (activeDir === 'asc' ? 'desc' : 'asc') : defDir;
    const next = `${key}_${dir}`;
    pushParams((p) => { next === DEFAULT_SORT ? p.delete('sort') : p.set('sort', next); });
  }

  function ariaSort(key: string): 'ascending' | 'descending' | 'none' {
    if (activeKey !== key) return 'none';
    return activeDir === 'asc' ? 'ascending' : 'descending';
  }

  const csvHref = $derived(
    `/admin/stock-velocity/export.csv?${new URLSearchParams({ days: String(data.days), sort: data.sort }).toString()}`
  );

  const formatMoney = (v: number) => `HK$${v.toFixed(2)}`;
  const formatDaily = (v: number) => v.toFixed(2);

  function formatDate(daysLeft: number): string {
    const d = new Date(Date.now() + daysLeft * 86_400_000);
    const mo = String(d.getMonth() + 1).padStart(2, '0');
    const da = String(d.getDate()).padStart(2, '0');
    return `${d.getFullYear()}-${mo}-${da}`;
  }

  // Runway colour: urgent (red) ≤14d, soon (amber) ≤30d, else neutral.
  function daysColour(n: number | null | undefined): string {
    if (n == null) return 'text-gray-400';
    if (n <= 14) return 'text-red-600';
    if (n <= 30) return 'text-amber-600';
    return 'text-gray-900';
  }
</script>

<svelte:head><title>{m.admin_stock_velocity_title()}</title></svelte:head>

<div class="flex items-start justify-between mb-6 gap-3">
  <div class="min-w-0">
    <div class="flex items-baseline gap-3">
      <h1 class="text-2xl font-bold text-gray-900">{m.admin_stock_velocity_heading()}</h1>
      <span class="text-sm text-gray-400">{data.total}</span>
    </div>
    <p class="text-sm text-gray-400 mt-0.5">{m.admin_stock_velocity_subtitle()}</p>
  </div>
  <a
    href={csvHref}
    data-sveltekit-reload
    class="shrink-0 inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium
           hover:bg-gray-800 transition-colors">
    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
    </svg>
    {m.admin_stock_velocity_export_csv()}
  </a>
</div>

<!-- Window selector -->
<div class="mb-4 flex flex-wrap items-center gap-2">
  <span class="text-xs text-gray-500 mr-1">{m.admin_stock_velocity_window_label()}</span>
  {#each data.windows as n}
    <button
      type="button"
      onclick={() => setDays(n)}
      class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
             {data.days === n
               ? 'bg-gray-900 text-white border-gray-900'
               : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
      {m.admin_stock_velocity_window_days({ n })}
    </button>
  {/each}
</div>

{#if data.capped}
  <div class="mb-3 px-4 py-2.5 rounded-xl bg-amber-50 text-amber-800 text-sm border border-amber-200">
    {m.admin_stock_velocity_capped({ n: data.total })}
  </div>
{/if}

<!-- Velocity table -->
<div class="bg-white rounded-2xl border border-gray-100 overflow-x-auto">
  <table class="w-full text-sm">
    <thead class="bg-gray-50 border-b border-gray-100">
      <tr>
        {#each COLS as col}
          <th
            class="px-5 py-3 font-medium text-gray-500 {col.align === 'right' ? 'text-right' : 'text-left'}"
            aria-sort={ariaSort(col.key)}>
            <button
              type="button"
              onclick={() => sortBy(col.key, col.defDir)}
              aria-label={m.admin_stock_velocity_sort_aria({ column: col.label })}
              class="inline-flex items-center gap-1 hover:text-gray-900 transition-colors
                     {col.align === 'right' ? 'flex-row-reverse' : ''}">
              <span>{col.label}</span>
              <span class="text-[10px] {activeKey === col.key ? 'text-gray-900' : 'text-gray-300'}">
                {activeKey === col.key ? (activeDir === 'asc' ? '▲' : '▼') : '↕'}
              </span>
            </button>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody class="divide-y divide-gray-50">
      {#each data.rows as r}
        <tr class="transition-colors hover:bg-gray-50/60">
          <td class="px-5 py-3">
            <a
              href="/admin/products/{r.product_id}"
              class="font-medium text-gray-900 hover:text-indigo-700 hover:underline underline-offset-2">
              {r.product_name}
            </a>
            <p class="text-xs text-gray-400">
              {#if r.variation !== r.sku}<span class="text-gray-500">{r.variation}</span> · {/if}<span class="font-mono">{r.sku}</span>
            </p>
          </td>
          <td class="px-5 py-3 text-right">
            <div class="tabular-nums text-gray-900">{r.stock_qty}</div>
            <span
              class="inline-flex items-center mt-0.5 px-2 py-0.5 rounded-full text-xs font-medium
                     {r.in_stock ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}">
              {r.in_stock ? m.admin_stock_velocity_in_stock() : m.admin_stock_velocity_out_of_stock()}
            </span>
          </td>
          <td class="px-5 py-3 text-right tabular-nums text-gray-900">{formatMoney(r.gross_sales)}</td>
          <td class="px-5 py-3 text-right tabular-nums text-gray-900">{r.gross_sold}</td>
          <td class="px-5 py-3 text-right tabular-nums text-gray-700">{formatDaily(r.daily_gross_sold)}</td>
          <td class="px-5 py-3 text-right">
            <div class="tabular-nums font-medium {daysColour(r.days_left)}">{r.days_left ?? '—'}</div>
            {#if r.days_left != null}
              <p class="text-xs text-gray-400">{m.admin_stock_velocity_stockout_on({ date: formatDate(r.days_left) })}</p>
            {/if}
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan={COLS.length} class="px-5 py-10 text-center text-gray-400">
            {m.admin_stock_velocity_empty()}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
