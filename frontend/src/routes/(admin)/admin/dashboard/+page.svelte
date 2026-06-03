<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import LineChart from '$lib/components/admin/charts/LineChart.svelte';
  import BarChart from '$lib/components/admin/charts/BarChart.svelte';
  import { orderStatusLabel } from '$lib/orderStatus';
  import { customerRoleLabel } from '$lib/types';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  let { data }: { data: PageData } = $props();

  function fmtHK(n: number): string {
    return `HK$${n.toLocaleString('en-HK', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;
  }
  function fmtPct(n: number): string {
    return `${(n * 100).toFixed(1)}%`;
  }
  function localISO(d: Date): string {
    const y = d.getFullYear();
    const mo = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${mo}-${day}`;
  }

  // ── Filter helpers (URL-driven, mirrors the orders list pattern) ──────────────
  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL($page.url);
    mutate(url.searchParams);
    goto(url.pathname + url.search, { keepFocus: true, noScroll: true });
  }
  function setDate(key: 'from' | 'to', value: string) {
    pushParams((p) => (value ? p.set(key, value) : p.delete(key)));
  }
  function setRangeDates(from: string, to: string) {
    pushParams((p) => {
      p.set('from', from);
      p.set('to', to);
    });
  }
  function presetDays(n: number) {
    const to = new Date();
    const from = new Date(to.getTime() - (n - 1) * 86_400_000);
    setRangeDates(localISO(from), localISO(to));
  }
  function presetToday() {
    const t = new Date();
    setRangeDates(localISO(t), localISO(t));
  }
  function presetThisMonth() {
    const t = new Date();
    setRangeDates(localISO(new Date(t.getFullYear(), t.getMonth(), 1)), localISO(t));
  }
  function presetLastMonth() {
    const t = new Date();
    const from = new Date(t.getFullYear(), t.getMonth() - 1, 1);
    const end = new Date(t.getFullYear(), t.getMonth(), 0);
    setRangeDates(localISO(from), localISO(end));
  }
  function presetYTD() {
    const t = new Date();
    setRangeDates(localISO(new Date(t.getFullYear(), 0, 1)), localISO(t));
  }
  function toggleRole(role: string) {
    pushParams((p) => {
      const cur = new Set((p.get('role') ?? '').split(',').filter(Boolean));
      cur.has(role) ? cur.delete(role) : cur.add(role);
      cur.size ? p.set('role', [...cur].join(',')) : p.delete('role');
    });
  }
  function setCategory(slug: string) {
    pushParams((p) => (slug ? p.set('category', slug) : p.delete('category')));
  }
  function setSortBy(by: 'qty' | 'revenue') {
    pushParams((p) => p.set('by', by));
  }
  function clearFilters() {
    pushParams((p) => {
      p.delete('from');
      p.delete('to');
      p.delete('role');
      p.delete('category');
    });
  }

  function roleLabel(l: string): string {
    if (l === 'installer') return m.admin_role_installer();
    if (l === 'installer_v2') return m.admin_role_installer_v2();
    if (l === 'customer') return m.admin_role_customer();
    return m.dashboard_role_guest();
  }

  function dayLabel(iso: string): string {
    return iso.slice(5); // MM-DD
  }
  const revenueSeries = $derived(
    (data.revenue ?? []).map((p) => ({ x: dayLabel(p.date), y: p.revenue }))
  );
  const statusBars = $derived(
    (data.statusBreakdown ?? []).map((s) => ({ label: orderStatusLabel(s.status), value: s.count }))
  );

  // Cumulative conversion funnel derived from the status breakdown (no extra
  // query): an order that reached a later stage also passed the earlier ones.
  const statusMap = $derived(
    Object.fromEntries((data.statusBreakdown ?? []).map((s) => [s.status, s.count])) as Record<
      string,
      number
    >
  );
  const g = (k: string) => statusMap[k] ?? 0;
  const funnelBars = $derived([
    { label: m.dashboard_funnel_pending(), value: g('pending') },
    { label: m.dashboard_funnel_paid(), value: g('paid') + g('processing') + g('shipped') + g('delivered') },
    { label: m.dashboard_funnel_shipped(), value: g('shipped') + g('delivered') },
    { label: m.dashboard_funnel_delivered(), value: g('delivered') }
  ]);

  const categoryBars = $derived((data.byCategory ?? []).map((r) => ({ label: r.label, value: r.value })));
  const roleBars = $derived((data.byRole ?? []).map((r) => ({ label: roleLabel(r.label), value: r.value })));
  const carrierBars = $derived((data.byCarrier ?? []).map((r) => ({ label: r.label, value: r.value })));
</script>

<svelte:head><title>{m.dashboard_title()}</title></svelte:head>

<div class="max-w-7xl space-y-8">

  <!-- Greeting -->
  <div>
    <h2 class="text-2xl font-bold text-gray-900">{m.dashboard_greeting()}</h2>
    <p class="text-sm text-gray-500 mt-1">{m.dashboard_greeting_sub()}</p>
  </div>

  <!-- Filter bar -->
  <div class="bg-white rounded-2xl border border-gray-100 p-4 shadow-sm space-y-3">
    <!-- Date range + presets -->
    <div class="flex flex-wrap items-end gap-3">
      <label class="flex flex-col gap-1">
        <span class="text-xs font-medium text-gray-400">{m.dashboard_filter_date_from()}</span>
        <input type="date" value={data.fromISO} onchange={(e) => setDate('from', e.currentTarget.value)}
               class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-sm text-gray-900 focus:border-gray-400 focus:outline-none" />
      </label>
      <label class="flex flex-col gap-1">
        <span class="text-xs font-medium text-gray-400">{m.dashboard_filter_date_to()}</span>
        <input type="date" value={data.toISO} onchange={(e) => setDate('to', e.currentTarget.value)}
               class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-sm text-gray-900 focus:border-gray-400 focus:outline-none" />
      </label>
      <div class="flex flex-wrap gap-1.5">
        <button onclick={presetToday} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_today()}</button>
        <button onclick={() => presetDays(7)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '7' })}</button>
        <button onclick={() => presetDays(30)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '30' })}</button>
        <button onclick={() => presetDays(90)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '90' })}</button>
        <button onclick={presetThisMonth} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_this_month()}</button>
        <button onclick={presetLastMonth} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_last_month()}</button>
        <button onclick={presetYTD} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_ytd()}</button>
      </div>
    </div>

    <!-- Role + category -->
    <div class="flex flex-wrap items-center gap-x-6 gap-y-3 pt-1 border-t border-gray-50">
      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-gray-400">{m.dashboard_filter_role_label()}</span>
        {#each ['customer', 'installer', 'installer_v2'] as role}
          <button onclick={() => toggleRole(role)}
                  class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors
                         {data.roles.includes(role) ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}">
            {customerRoleLabel(role)}
          </button>
        {/each}
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-gray-400">{m.dashboard_filter_category_label()}</span>
        <select value={data.category} onchange={(e) => setCategory(e.currentTarget.value)}
                class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-sm text-gray-900 focus:border-gray-400 focus:outline-none">
          <option value="">{m.dashboard_filter_category_all()}</option>
          {#each data.categories as c}
            <option value={c.slug}>{c.name}</option>
          {/each}
        </select>
      </div>
      <button onclick={clearFilters} class="ml-auto text-xs font-medium text-gray-400 hover:text-gray-900 transition-colors">
        {m.dashboard_filter_clear()}
      </button>
    </div>
  </div>

  {#if data.stats}
    <!-- Stats grid — static class names so Tailwind includes them -->
    <div class="grid grid-cols-2 xl:grid-cols-5 gap-4">

      <!-- Total Products (snapshot) -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-violet-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.total_products}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">{m.dashboard_stats_total_products()}</p>
        <p class="text-[10px] text-gray-300 mt-0.5">{m.dashboard_snapshot_hint()}</p>
      </div>

      <!-- Total Orders -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-blue-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.total_orders}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">{m.dashboard_stats_total_orders()}</p>
      </div>

      <!-- Pending Orders -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-amber-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-3xl font-bold text-gray-900 tabular-nums">{data.stats.pending_orders}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">{m.dashboard_stats_pending_orders()}</p>
      </div>

      <!-- Total Revenue -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-emerald-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
          </svg>
        </div>
        <p class="mt-4 text-2xl font-bold text-gray-900 tabular-nums">
          HK${data.stats.total_revenue.toLocaleString('en-HK', { minimumFractionDigits: 0 })}
        </p>
        <p class="text-xs text-gray-400 font-medium mt-1">{m.dashboard_stats_total_revenue()}</p>
      </div>

      <!-- Total Refunds -->
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm hover:shadow-md transition-shadow">
        <div class="w-10 h-10 rounded-xl bg-red-500 flex items-center justify-center shadow-sm">
          <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3"/>
          </svg>
        </div>
        <p class="mt-4 text-2xl font-bold text-gray-900 tabular-nums">{fmtHK(data.refundTotal ?? 0)}</p>
        <p class="text-xs text-gray-400 font-medium mt-1">{m.dashboard_stats_refunds()}</p>
      </div>

    </div>

    <!-- Secondary KPIs: AOV, new customers, repeat ratio, refund rate -->
    <div class="grid grid-cols-2 xl:grid-cols-4 gap-4">
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm">
        <p class="text-xs text-gray-400 font-medium">{m.dashboard_aov()}</p>
        <p class="mt-2 text-2xl font-bold text-gray-900 tabular-nums">{fmtHK(data.summary?.aov ?? 0)}</p>
      </div>
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm">
        <p class="text-xs text-gray-400 font-medium">{m.dashboard_new_customers()}</p>
        <p class="mt-2 text-2xl font-bold text-gray-900 tabular-nums">{data.summary?.new_customers ?? 0}</p>
      </div>
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm">
        <p class="text-xs text-gray-400 font-medium">{m.dashboard_repeat_ratio()}</p>
        <p class="mt-2 text-2xl font-bold text-gray-900 tabular-nums">{fmtPct(data.summary?.repeat_ratio ?? 0)}</p>
      </div>
      <div class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm">
        <p class="text-xs text-gray-400 font-medium">{m.dashboard_refund_rate()}</p>
        <p class="mt-2 text-2xl font-bold text-gray-900 tabular-nums">{fmtPct(data.refund?.refund_order_rate ?? 0)}</p>
        <p class="text-[11px] text-gray-400 mt-0.5">{m.dashboard_refund_rate_sub({ pct: fmtPct(data.refund?.refund_amount_rate ?? 0) })}</p>
      </div>
    </div>

    <!-- Revenue trend -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-sm font-semibold text-gray-900">{m.dashboard_revenue_trend_heading()}</h3>
        <p class="text-xs text-gray-400">{data.fromISO} → {data.toISO}</p>
      </div>
      <LineChart data={revenueSeries} formatY={fmtHK} />
    </div>

    <!-- Revenue breakdown -->
    <div>
      <h3 class="text-sm font-semibold text-gray-900 mb-4">{m.dashboard_revenue_breakdown_heading()}</h3>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.dashboard_revenue_breakdown_by_category()}</p>
          <BarChart data={categoryBars} formatValue={fmtHK} />
        </div>
        <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.dashboard_revenue_breakdown_by_role()}</p>
          <BarChart data={roleBars} formatValue={fmtHK} />
        </div>
        <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">{m.dashboard_revenue_breakdown_by_carrier()}</p>
          <BarChart data={carrierBars} formatValue={fmtHK} />
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Top Products -->
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-sm font-semibold text-gray-900">{m.dashboard_top_products_heading()}</h3>
          <div class="inline-flex bg-gray-100 rounded-lg p-0.5">
            <button onclick={() => setSortBy('qty')}
                    class="px-2.5 py-1 rounded-md text-xs font-medium transition-colors
                           {data.sortBy === 'qty' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500'}">
              {m.dashboard_top_products_by_qty()}
            </button>
            <button onclick={() => setSortBy('revenue')}
                    class="px-2.5 py-1 rounded-md text-xs font-medium transition-colors
                           {data.sortBy === 'revenue' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500'}">
              {m.dashboard_top_products_by_revenue()}
            </button>
          </div>
        </div>
        {#if (data.topProducts ?? []).length === 0}
          <p class="text-sm text-gray-400 py-6 text-center">{m.dashboard_no_data()}</p>
        {:else}
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-50">
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_products_col_name()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_products_col_qty()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_products_col_revenue()}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50">
              {#each data.topProducts as p}
                <tr>
                  <td class="py-2.5">
                    <p class="text-gray-900 truncate max-w-xs">{p.product_name}</p>
                    <p class="text-xs text-gray-400 font-mono">{p.variant_sku}</p>
                  </td>
                  <td class="py-2.5 text-right text-gray-700 font-mono">{p.qty_sold}</td>
                  <td class="py-2.5 text-right text-gray-700 font-mono">{fmtHK(p.revenue)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <!-- Top Customers -->
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <h3 class="text-sm font-semibold text-gray-900 mb-4">{m.dashboard_top_customers_heading()}</h3>
        {#if (data.topCustomers ?? []).length === 0}
          <p class="text-sm text-gray-400 py-6 text-center">{m.dashboard_no_data()}</p>
        {:else}
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-50">
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_customers_col_name()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_customers_col_orders()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_top_customers_col_spent()}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50">
              {#each data.topCustomers as c}
                <tr>
                  <td class="py-2.5">
                    <p class="text-gray-900 truncate max-w-xs">{c.name || '—'}</p>
                    <p class="text-xs text-gray-400 truncate max-w-xs">{c.email}</p>
                  </td>
                  <td class="py-2.5 text-right text-gray-700 font-mono">{c.order_count}</td>
                  <td class="py-2.5 text-right text-gray-700 font-mono">{fmtHK(c.total_spent)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>

    <!-- Order status breakdown + conversion funnel -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <h3 class="text-sm font-semibold text-gray-900 mb-4">{m.dashboard_status_breakdown_heading()}</h3>
        <BarChart data={statusBars} formatValue={(n) => String(n)} />
      </div>
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <h3 class="text-sm font-semibold text-gray-900 mb-4">{m.dashboard_funnel_heading()}</h3>
        <BarChart data={funnelBars} formatValue={(n) => String(n)} />
      </div>
    </div>

    <!-- Recent orders + low stock -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Recent orders -->
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <h3 class="text-sm font-semibold text-gray-900 mb-4">{m.dashboard_recent_orders_heading()}</h3>
        {#if (data.recentOrders ?? []).length === 0}
          <p class="text-sm text-gray-400 py-6 text-center">{m.dashboard_no_data()}</p>
        {:else}
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-50">
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_recent_orders_col_order()}</th>
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_recent_orders_col_status()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_recent_orders_col_total()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_recent_orders_col_date()}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50">
              {#each data.recentOrders as o}
                <tr class="hover:bg-gray-50 cursor-pointer" onclick={() => goto(`/admin/orders/${o.id}`)}>
                  <td class="py-2.5">
                    <p class="text-gray-900 font-mono">{o.order_number || `#${o.number}`}</p>
                    <p class="text-xs text-gray-400 truncate max-w-[12rem]">{o.customer_name || o.customer_email || '—'}</p>
                  </td>
                  <td class="py-2.5 text-gray-700">{orderStatusLabel(o.status)}</td>
                  <td class="py-2.5 text-right text-gray-700 font-mono">{fmtHK(o.total)}</td>
                  <td class="py-2.5 text-right text-gray-400 text-xs">{o.created_at.slice(0, 10)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <!-- Low stock (snapshot) -->
      <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-sm font-semibold text-gray-900">{m.dashboard_low_stock_heading()}</h3>
          <span class="text-[10px] text-gray-300">{m.dashboard_low_stock_snapshot()}</span>
        </div>
        {#if (data.lowStock ?? []).length === 0}
          <p class="text-sm text-gray-400 py-6 text-center">{m.dashboard_no_data()}</p>
        {:else}
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-50">
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_low_stock_col_product()}</th>
                <th class="text-left text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_low_stock_col_sku()}</th>
                <th class="text-right text-xs font-semibold text-gray-400 uppercase tracking-wide pb-2">{m.dashboard_low_stock_col_stock()}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50">
              {#each data.lowStock as v}
                <tr>
                  <td class="py-2.5 text-gray-900 truncate max-w-[14rem]">{v.product_name || v.name || '—'}</td>
                  <td class="py-2.5 text-gray-400 font-mono text-xs">{v.sku}</td>
                  <td class="py-2.5 text-right font-mono {v.stock_qty <= 0 ? 'text-red-600' : 'text-amber-600'}">{v.stock_qty}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>

    <!-- Quick actions -->
    <div>
      <p class="text-xs font-semibold text-gray-400 uppercase tracking-widest mb-4">{m.dashboard_quick_actions()}</p>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">

        <a href="/admin/products"
           class="group bg-white rounded-2xl border border-gray-100 p-5 shadow-sm
                  hover:shadow-md hover:border-gray-200 transition-all flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-gray-100 group-hover:bg-gray-900
                      flex items-center justify-center transition-colors flex-shrink-0">
            <svg class="w-5 h-5 text-gray-500 group-hover:text-white transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z"/>
            </svg>
          </div>
          <div class="flex-1 min-w-0">
            <p class="font-semibold text-gray-900 text-sm">{m.dashboard_qa_manage_products()}</p>
            <p class="text-xs text-gray-400 mt-0.5">{m.dashboard_qa_manage_products_desc()}</p>
          </div>
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-600 group-hover:translate-x-0.5
                      transition-all flex-shrink-0"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5"/>
          </svg>
        </a>

        <a href="/admin/orders"
           class="group bg-white rounded-2xl border border-gray-100 p-5 shadow-sm
                  hover:shadow-md hover:border-gray-200 transition-all flex items-center gap-4">
          <div class="w-10 h-10 rounded-xl bg-gray-100 group-hover:bg-gray-900
                      flex items-center justify-center transition-colors flex-shrink-0">
            <svg class="w-5 h-5 text-gray-500 group-hover:text-white transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"/>
            </svg>
          </div>
          <div class="flex-1 min-w-0">
            <p class="font-semibold text-gray-900 text-sm">{m.dashboard_qa_view_orders()}</p>
            <p class="text-xs text-gray-400 mt-0.5">{m.dashboard_qa_view_orders_desc()}</p>
          </div>
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-600 group-hover:translate-x-0.5
                      transition-all flex-shrink-0"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5"/>
          </svg>
        </a>

      </div>
    </div>

  {:else}
    <div class="h-40 flex items-center justify-center text-gray-400 text-sm">
      {m.dashboard_stats_unavailable()}
    </div>
  {/if}

</div>
