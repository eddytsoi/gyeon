<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import LineChart from '$lib/components/admin/charts/LineChart.svelte';
  import KpiCard from './KpiCard.svelte';
  import { HERO_SECONDARY, fmtHK, formatValue } from './widgets';

  interface MetricValue {
    current: number | null;
    previous: number | null;
    delta_pct: number | null;
    disabled?: boolean;
  }
  interface DayPoint { day: number; value: number }
  interface HeroPeriod { label: string; total: number; daily: DayPoint[] }
  interface Props {
    hero: { current: HeroPeriod; previous: HeroPeriod };
    metrics: Record<string, MetricValue>;
    series: Record<string, { date: string; value: number }[]>;
  }
  let { hero, metrics, series }: Props = $props();

  const curTotal = $derived(hero?.current?.total ?? 0);
  const prevTotal = $derived(hero?.previous?.total ?? 0);
  const heroDelta = $derived(prevTotal ? (curTotal - prevTotal) / prevTotal : null);
  const heroUp = $derived((heroDelta ?? 0) >= 0);

  // Densify the two day-of-month series into equal-length, gap-filled arrays so
  // the dual line overlays cleanly (months with no orders on a day → 0).
  function densify(p: HeroPeriod | undefined, maxDay: number) {
    const byDay = new Map((p?.daily ?? []).map((d) => [d.day, d.value]));
    return Array.from({ length: maxDay }, (_, i) => ({ x: String(i + 1), y: byDay.get(i + 1) ?? 0 }));
  }
  const maxDay = $derived(
    Math.max(
      1,
      ...(hero?.current?.daily ?? []).map((d) => d.day),
      ...(hero?.previous?.daily ?? []).map((d) => d.day)
    )
  );
  const curLine = $derived(densify(hero?.current, maxDay));
  const prevLine = $derived(densify(hero?.previous, maxDay));

  function metricVal(key: string): MetricValue {
    return metrics?.[key] ?? { current: null, previous: null, delta_pct: null };
  }
  function seriesVals(key: string): number[] {
    return (series?.[key] ?? []).map((p) => p.value);
  }
</script>

<section class="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
  <div class="border-t-2 border-indigo-500"></div>
  <div class="p-5 grid grid-cols-1 lg:grid-cols-2 gap-5">
    <!-- Headline: this month vs same period last month -->
    <div class="flex flex-col justify-center">
      <p class="text-xs font-semibold uppercase tracking-wide text-gray-400">
        {m.dashboard_m_net_revenue()} · {m.dashboard_hero_this_month()}
      </p>
      <div class="mt-2 flex items-end gap-3 flex-wrap">
        <span class="text-4xl font-bold text-gray-900 tabular-nums leading-none">{fmtHK(curTotal)}</span>
        {#if heroDelta != null}
          <span class="inline-flex items-center gap-0.5 text-sm font-semibold rounded-full px-2 py-0.5
                       {heroUp ? 'text-emerald-700 bg-emerald-50' : 'text-red-600 bg-red-50'}">
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
              {#if heroUp}<path d="M12 5l7 7h-4v7h-6v-7H5z"/>{:else}<path d="M12 19l-7-7h4V5h6v7h4z"/>{/if}
            </svg>
            {heroUp ? '+' : '−'}{Math.abs((heroDelta ?? 0) * 100).toFixed(1)}%
          </span>
        {/if}
      </div>
      <p class="mt-1.5 text-sm text-gray-500">
        {m.dashboard_hero_vs()}: <span class="font-medium text-gray-700 tabular-nums">{fmtHK(prevTotal)}</span>
      </p>
    </div>

    <!-- Dual-line: this month vs last month, overlaid by day-of-month -->
    <div>
      <LineChart
        data={curLine}
        compare={prevLine}
        height={150}
        formatY={fmtHK}
        color="#6366f1"
        compareColor="#c7d2fe"
        label={hero?.current?.label ?? m.dashboard_hero_this_month()}
        compareLabel={hero?.previous?.label ?? m.dashboard_hero_last_month()}
      />
    </div>
  </div>

  <!-- Secondary must-use cards: carts abandoned + the two profit placeholders -->
  <div class="px-5 pb-5 grid grid-cols-1 sm:grid-cols-3 gap-4">
    {#each HERO_SECONDARY as c}
      {@const mv = metricVal(c.key)}
      <KpiCard
        title={c.title()}
        value={c.disabled ? '—' : formatValue(c.fmt, mv.current ?? 0)}
        deltaPct={c.disabled ? null : mv.delta_pct}
        goodWhenUp={c.goodWhenUp ?? true}
        series={c.disabled ? [] : seriesVals(c.key)}
        accent={c.accent}
        disabled={c.disabled}
        hint={c.hint ? c.hint() : ''}
      />
    {/each}
  </div>
</section>
