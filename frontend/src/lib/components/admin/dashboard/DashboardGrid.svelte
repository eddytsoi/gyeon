<script lang="ts">
  import KpiCard from './KpiCard.svelte';
  import {
    GRID_SECTIONS,
    widgetByKey,
    formatValue,
    type DashLayout,
    type SectionKey
  } from './widgets';

  interface MetricValue {
    current: number | null;
    previous: number | null;
    delta_pct: number | null;
    disabled?: boolean;
  }
  interface Props {
    layout: DashLayout;
    metrics: Record<string, MetricValue>;
    series: Record<string, { date: string; value: number }[]>;
    editing?: boolean;
    onToggleCollapse?: (section: SectionKey) => void;
    onToggleVisible?: (section: SectionKey, widget: string) => void;
    onMove?: (section: SectionKey, widget: string, dir: -1 | 1) => void;
  }
  let {
    layout,
    metrics,
    series,
    editing = false,
    onToggleCollapse,
    onToggleVisible,
    onMove
  }: Props = $props();

  const sectionTitle = (key: SectionKey) => GRID_SECTIONS.find((s) => s.key === key)?.title() ?? key;

  function metricVal(key: string): MetricValue {
    return metrics?.[key] ?? { current: null, previous: null, delta_pct: null };
  }
  function seriesVals(key: string): number[] {
    return (series?.[key] ?? []).map((p) => p.value);
  }
</script>

{#each layout.sections as section (section.key)}
  {@const visibleCount = section.widgets.filter((w) => w.visible).length}
  {#if editing || visibleCount > 0}
    <section class="space-y-3">
      <button
        type="button"
        onclick={() => onToggleCollapse?.(section.key)}
        class="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-gray-400 hover:text-gray-700 transition-colors"
      >
        <svg class="w-3.5 h-3.5 transition-transform {section.collapsed ? '-rotate-90' : ''}"
             fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5"/>
        </svg>
        {sectionTitle(section.key)}
        {#if editing}
          <span class="font-normal lowercase text-gray-300">({visibleCount}/{section.widgets.length})</span>
        {/if}
      </button>

      {#if !section.collapsed}
        <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4">
          {#each section.widgets as ws, i (ws.key)}
            {@const def = widgetByKey(ws.key)}
            {#if def && (editing || ws.visible)}
              {@const mv = metricVal(ws.key)}
              <KpiCard
                title={def.title()}
                value={formatValue(def.fmt, mv.current ?? 0)}
                deltaPct={def.comparison ? mv.delta_pct : null}
                goodWhenUp={def.goodWhenUp ?? true}
                series={def.hasSeries ? seriesVals(ws.key) : []}
                accent={def.accent}
                badge={def.badge ?? null}
                {editing}
                hiddenInLayout={!ws.visible}
                canUp={i > 0}
                canDown={i < section.widgets.length - 1}
                onToggleVisible={() => onToggleVisible?.(section.key, ws.key)}
                onMoveUp={() => onMove?.(section.key, ws.key, -1)}
                onMoveDown={() => onMove?.(section.key, ws.key, 1)}
              />
            {/if}
          {/each}
        </div>
      {/if}
    </section>
  {/if}
{/each}
