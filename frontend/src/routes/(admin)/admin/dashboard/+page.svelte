<script lang="ts">
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';
  import LineChart from '$lib/components/admin/charts/LineChart.svelte';
  import BarChart from '$lib/components/admin/charts/BarChart.svelte';
  import HeroBand from '$lib/components/admin/dashboard/HeroBand.svelte';
  import DashboardGrid from '$lib/components/admin/dashboard/DashboardGrid.svelte';
  import {
    buildDefaultLayout,
    mergeLayout,
    fmtHK,
    type DashLayout,
    type SectionKey
  } from '$lib/components/admin/dashboard/widgets';
  import { adminSaveDashboardPrefs, type DashboardPreset } from '$lib/api/admin';
  import { orderStatusLabel } from '$lib/orderStatus';
  import { customerRoleLabel } from '$lib/types';
  import { productDisplayName } from '$lib/variant';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';

  let { data }: { data: PageData } = $props();

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
  function presetLastYear() {
    const y = new Date().getFullYear() - 1;
    setRangeDates(localISO(new Date(y, 0, 1)), localISO(new Date(y, 11, 31)));
  }
  function presetAll() {
    setRangeDates('2000-01-01', localISO(new Date()));
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
  function setCompare(mode: string) {
    pushParams((p) => p.set('compare', mode));
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

  // ── Customise / per-admin layout presets (persisted server-side) ──────────────
  let editing = $state(false);
  let saving = $state(false);
  let savedTick = $state(0);

  // Client mirror of the saved presets + active selection. Compare mode is
  // URL-driven (SSR refetches on change); we persist it as the per-admin default.
  let presets = $state<DashboardPreset[]>(
    (data.prefs?.presets ?? []).map((p) => ({ id: p.id, name: p.name, is_default: p.is_default, layout: p.layout }))
  );
  let activePresetId = $state<string | null>(data.prefs?.active_preset_id ?? null);
  // Compare mode is URL-driven (SSR refetches the metrics on change); track it
  // reactively so saves carry the current value, and persist it as the per-admin
  // default whenever it diverges from what was last saved.
  const compareMode = $derived(data.compare);
  let savedCompare = data.prefs?.compare_mode ?? 'prev_month';

  function activePreset(): DashboardPreset | undefined {
    return presets.find((p) => p.id === activePresetId);
  }
  function initialLayout(): DashLayout {
    return mergeLayout((activePreset()?.layout as DashLayout) ?? null);
  }
  let layoutState = $state<DashLayout>(initialLayout());

  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  function scheduleSave() {
    if (!data.token) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(doSave, 600);
  }
  // Fold the current editable layout back into the active preset (creating an
  // implicit "Default" the first time an admin customises anything).
  function syncActiveLayout() {
    let ap = activePreset();
    if (!ap) {
      ap = { id: crypto.randomUUID(), name: m.dashboard_preset_default(), is_default: true, layout: {} };
      presets = [ap, ...presets];
      activePresetId = ap.id;
    }
    ap.layout = $state.snapshot(layoutState);
  }
  async function doSave() {
    if (!data.token) return;
    syncActiveLayout();
    saving = true;
    try {
      await adminSaveDashboardPrefs(data.token, {
        presets: $state.snapshot(presets),
        active_preset_id: activePresetId,
        compare_mode: compareMode
      });
      savedTick = Date.now();
    } catch (e) {
      console.error('dashboard prefs save failed', e);
    } finally {
      saving = false;
    }
  }

  // Persist the compare mode as the per-admin default whenever it changes
  // (covers both a same-route navigation from the selector and an SSR load that
  // arrived with a ?compare= param differing from what was saved).
  $effect(() => {
    if (data.token && data.compare !== savedCompare) {
      savedCompare = data.compare;
      scheduleSave();
    }
  });

  function toggleCollapse(section: SectionKey) {
    const s = layoutState.sections.find((x) => x.key === section);
    if (s) {
      s.collapsed = !s.collapsed;
      scheduleSave();
    }
  }
  function toggleVisible(section: SectionKey, widget: string) {
    const w = layoutState.sections.find((x) => x.key === section)?.widgets.find((y) => y.key === widget);
    if (w) {
      w.visible = !w.visible;
      scheduleSave();
    }
  }
  function move(section: SectionKey, widget: string, dir: -1 | 1) {
    const s = layoutState.sections.find((x) => x.key === section);
    if (!s) return;
    const i = s.widgets.findIndex((y) => y.key === widget);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= s.widgets.length) return;
    [s.widgets[i], s.widgets[j]] = [s.widgets[j], s.widgets[i]];
    scheduleSave();
  }
  function resetLayout() {
    layoutState = buildDefaultLayout();
    scheduleSave();
  }

  // ── Preset management ─────────────────────────────────────────────────────────
  function selectPreset(id: string) {
    activePresetId = id;
    layoutState = mergeLayout((activePreset()?.layout as DashLayout) ?? null);
    scheduleSave();
  }
  function newPreset() {
    const name = (prompt(m.dashboard_preset_name_prompt()) ?? '').trim();
    if (!name) return;
    const p: DashboardPreset = { id: crypto.randomUUID(), name, is_default: false, layout: $state.snapshot(buildDefaultLayout()) };
    presets = [...presets, p];
    activePresetId = p.id;
    layoutState = buildDefaultLayout();
    scheduleSave();
  }
  function duplicatePreset() {
    const cur = activePreset();
    const name = (prompt(m.dashboard_preset_name_prompt(), `${cur?.name ?? ''} copy`) ?? '').trim();
    if (!name) return;
    const p: DashboardPreset = { id: crypto.randomUUID(), name, is_default: false, layout: $state.snapshot(layoutState) };
    presets = [...presets, p];
    activePresetId = p.id;
    scheduleSave();
  }
  function renamePreset() {
    const cur = activePreset();
    if (!cur) return;
    const name = (prompt(m.dashboard_preset_name_prompt(), cur.name) ?? '').trim();
    if (!name) return;
    cur.name = name;
    scheduleSave();
  }
  function deletePreset() {
    const cur = activePreset();
    if (!cur || presets.length <= 1) return;
    presets = presets.filter((p) => p.id !== cur.id);
    activePresetId = presets[0]?.id ?? null;
    layoutState = mergeLayout((activePreset()?.layout as DashLayout) ?? null);
    scheduleSave();
  }

  // ── Derived data for the hero + detail panels ────────────────────────────────
  const dash = $derived(data.dashboard);
  const revenueSeries = $derived(
    (dash?.series?.net_revenue ?? []).map((p) => ({ x: p.date.slice(5), y: p.value }))
  );
  const statusBars = $derived(
    (data.statusBreakdown ?? []).map((s) => ({ label: orderStatusLabel(s.status), value: s.count }))
  );
  const statusMap = $derived(
    Object.fromEntries((data.statusBreakdown ?? []).map((s) => [s.status, s.count])) as Record<string, number>
  );
  const g = (k: string) => statusMap[k] ?? 0;
  const funnelBars = $derived([
    { label: m.dashboard_funnel_pending(), value: g('pending') },
    { label: m.dashboard_funnel_paid(), value: g('paid') + g('processing') + g('prepared') + g('shipped') + g('delivered') },
    { label: m.dashboard_funnel_shipped(), value: g('shipped') + g('delivered') },
    { label: m.dashboard_funnel_delivered(), value: g('delivered') }
  ]);

  const courierNameByUid = $derived(new Map((data.couriers ?? []).map((c) => [c.uid, c.name])));
  function carrierLabel(l: string): string {
    if (!l) return m.dashboard_carrier_no_record();
    return courierNameByUid.get(l) ?? l;
  }
  const categoryBars = $derived((data.byCategory ?? []).map((r) => ({ label: r.label, value: r.value })));
  const roleBars = $derived((data.byRole ?? []).map((r) => ({ label: roleLabel(r.label), value: r.value })));
  const carrierBars = $derived((data.byCarrier ?? []).map((r) => ({ label: carrierLabel(r.label), value: r.value })));
</script>

<svelte:head><title>{m.dashboard_title()}</title></svelte:head>

<div class="max-w-7xl space-y-6">

  <!-- Header -->
  <div class="flex items-start justify-between gap-3">
    <div>
      <h2 class="text-2xl font-bold text-gray-900">{m.dashboard_greeting()}</h2>
      <p class="text-sm text-gray-500 mt-1">{m.dashboard_greeting_sub()}</p>
    </div>
    <div class="shrink-0 flex items-center gap-2">
      {#if saving}
        <span class="text-[11px] text-gray-400">{m.dashboard_saving()}</span>
      {:else if savedTick}
        <span class="text-[11px] text-emerald-600">{m.dashboard_saved()}</span>
      {/if}
      {#if presets.length > 0}
        <select value={activePresetId} onchange={(e) => selectPreset(e.currentTarget.value)}
                class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-700 focus:border-gray-400 focus:outline-none">
          {#each presets as p}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      {/if}
      <button
        onclick={() => (editing = !editing)}
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors
               {editing ? 'bg-gray-900 text-white' : 'bg-gray-100 text-gray-700 hover:bg-gray-200'}"
      >
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456c.516-.194 1.094.014 1.37.49l1.296 2.247c.275.476.165 1.083-.26 1.43l-1.004.827c-.292.241-.437.613-.43.992a7.723 7.723 0 0 1 0 .255c-.007.378.138.75.43.991l1.004.828c.424.347.534.954.26 1.43l-1.297 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.397-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 0 1 0-.255c.007-.378-.138-.75-.43-.991l-1.004-.828a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.077-.124.072-.044.146-.086.22-.128.331-.183.581-.495.644-.869l.213-1.281Z"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
        </svg>
        {editing ? m.dashboard_customise_done() : m.dashboard_customise()}
      </button>
    </div>
  </div>

  <!-- Filter bar -->
  <div class="bg-white rounded-2xl border border-gray-100 p-4 shadow-sm space-y-3">
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
      <label class="flex flex-col gap-1">
        <span class="text-xs font-medium text-gray-400">{m.dashboard_compare_label()}</span>
        <select value={data.compare} onchange={(e) => setCompare(e.currentTarget.value)}
                class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-sm text-gray-900 focus:border-gray-400 focus:outline-none">
          <option value="prev_month">{m.dashboard_compare_prev_month()}</option>
          <option value="prev_period">{m.dashboard_compare_prev_period()}</option>
          <option value="prev_year">{m.dashboard_compare_prev_year()}</option>
          <option value="none">{m.dashboard_compare_none()}</option>
        </select>
      </label>
      <div class="flex flex-wrap gap-1.5">
        <button onclick={presetToday} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_today()}</button>
        <button onclick={() => presetDays(7)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '7' })}</button>
        <button onclick={() => presetDays(30)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '30' })}</button>
        <button onclick={() => presetDays(90)} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_range_days({ n: '90' })}</button>
        <button onclick={presetThisMonth} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_this_month()}</button>
        <button onclick={presetLastMonth} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_last_month()}</button>
        <button onclick={presetYTD} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_ytd()}</button>
        <button onclick={presetLastYear} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_last_year()}</button>
        <button onclick={presetAll} class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors">{m.dashboard_filter_preset_all()}</button>
      </div>
    </div>

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

  {#if dash}
    <!-- Hero: this-month-vs-last-month net revenue + must-use cards -->
    <HeroBand hero={dash.hero} metrics={dash.metrics} series={dash.series} />

    {#if editing}
      <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 bg-amber-50 border border-amber-100 rounded-xl px-4 py-2">
        <span class="text-amber-700">{m.dashboard_customise_hint()}</span>
        <div class="ml-auto flex items-center gap-1.5">
          <button onclick={newPreset} class="px-2 py-1 rounded-md bg-white border border-gray-200 text-gray-600 hover:bg-gray-50">{m.dashboard_preset_new()}</button>
          <button onclick={renamePreset} disabled={!activePreset()} class="px-2 py-1 rounded-md bg-white border border-gray-200 text-gray-600 hover:bg-gray-50 disabled:opacity-40">{m.dashboard_preset_rename()}</button>
          <button onclick={duplicatePreset} class="px-2 py-1 rounded-md bg-white border border-gray-200 text-gray-600 hover:bg-gray-50">{m.dashboard_preset_duplicate()}</button>
          <button onclick={deletePreset} disabled={presets.length <= 1} class="px-2 py-1 rounded-md bg-white border border-gray-200 text-red-600 hover:bg-red-50 disabled:opacity-40">{m.dashboard_preset_delete()}</button>
          <button onclick={resetLayout} class="px-2 py-1 rounded-md bg-white border border-gray-200 text-gray-600 hover:bg-gray-50">{m.dashboard_customise_reset()}</button>
        </div>
      </div>
    {/if}

    <!-- Customisable KPI grid: Sales / Customers / Carts -->
    <DashboardGrid
      layout={layoutState}
      metrics={dash.metrics}
      series={dash.series}
      {editing}
      onToggleCollapse={toggleCollapse}
      onToggleVisible={toggleVisible}
      onMove={move}
    />

    <!-- Revenue trend (selected range) -->
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
                  <td class="py-2.5 text-gray-900 truncate max-w-[14rem]">{v.product_name ? productDisplayName(v.product_name, v.name) : (v.name || '—')}</td>
                  <td class="py-2.5 text-gray-400 font-mono text-xs">{v.sku}</td>
                  <td class="py-2.5 text-right font-mono {v.stock_qty <= 0 ? 'text-red-600' : 'text-amber-600'}">{v.stock_qty}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>

  {:else}
    <div class="h-40 flex items-center justify-center text-gray-400 text-sm">
      {m.dashboard_stats_unavailable()}
    </div>
  {/if}

</div>
