<script lang="ts">
  import * as m from '$lib/paraglide/messages';
  import Sparkline from '$lib/components/admin/charts/Sparkline.svelte';

  interface Props {
    title: string;
    value: string; // pre-formatted (e.g. "HK$128,400", "55.2%", "—")
    deltaPct?: number | null; // fractional change; null → no pill
    goodWhenUp?: boolean; // default true; false for refunds / abandoned / failed
    series?: number[];
    accent?: string;
    badge?: 'all_time' | 'in_period' | null;
    disabled?: boolean; // placeholder mode (no data yet)
    hint?: string;
    // customise-mode chrome (all optional; inert unless `editing`)
    editing?: boolean;
    onToggleVisible?: () => void;
    onMoveUp?: () => void;
    onMoveDown?: () => void;
    canUp?: boolean;
    canDown?: boolean;
    hiddenInLayout?: boolean;
  }
  let {
    title,
    value,
    deltaPct = null,
    goodWhenUp = true,
    series = [],
    accent = '#6366f1',
    badge = null,
    disabled = false,
    hint = '',
    editing = false,
    onToggleVisible,
    onMoveUp,
    onMoveDown,
    canUp = true,
    canDown = true,
    hiddenInLayout = false
  }: Props = $props();

  const up = $derived((deltaPct ?? 0) >= 0);
  // A positive change is "good" (green) when the metric is good-when-up and rose,
  // or bad-when-up and fell. Otherwise red.
  const good = $derived(up === goodWhenUp);
  const deltaText = $derived(
    deltaPct == null ? '' : `${up ? '+' : '−'}${Math.abs(deltaPct * 100).toFixed(1)}%`
  );
  const badgeText = $derived(
    badge === 'all_time' ? m.dashboard_badge_all_time() : badge === 'in_period' ? m.dashboard_badge_in_period() : ''
  );
</script>

<div
  class="relative bg-white rounded-2xl border border-gray-100 p-4 shadow-sm transition-shadow
         {editing ? 'ring-1 ring-gray-200' : 'hover:shadow-md'} {hiddenInLayout ? 'opacity-40' : ''}"
  style="border-top: 2px solid {disabled ? '#e5e7eb' : accent};"
>
  {#if editing}
    <div class="absolute -top-2 -right-2 flex items-center gap-1">
      <button type="button" onclick={onMoveUp} disabled={!canUp} title="Move up"
              class="w-6 h-6 rounded-md bg-white border border-gray-200 shadow-sm text-gray-500 hover:bg-gray-50 disabled:opacity-30 flex items-center justify-center">
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 15.75l7.5-7.5 7.5 7.5"/></svg>
      </button>
      <button type="button" onclick={onMoveDown} disabled={!canDown} title="Move down"
              class="w-6 h-6 rounded-md bg-white border border-gray-200 shadow-sm text-gray-500 hover:bg-gray-50 disabled:opacity-30 flex items-center justify-center">
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5"/></svg>
      </button>
      <button type="button" onclick={onToggleVisible} title={hiddenInLayout ? 'Show' : 'Hide'}
              class="w-6 h-6 rounded-md bg-white border border-gray-200 shadow-sm text-gray-500 hover:bg-gray-50 flex items-center justify-center">
        {#if hiddenInLayout}
          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/></svg>
        {:else}
          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88"/></svg>
        {/if}
      </button>
    </div>
  {/if}

  <div class="flex items-start justify-between gap-2">
    <p class="text-xs text-gray-400 font-medium leading-tight">{title}</p>
    {#if badgeText}
      <span class="shrink-0 text-[9px] font-semibold tracking-wide text-gray-400 bg-gray-50 border border-gray-100 rounded px-1.5 py-0.5">{badgeText}</span>
    {/if}
  </div>

  {#if disabled}
    <p class="mt-3 text-2xl font-bold text-gray-300 tabular-nums">—</p>
    {#if hint}
      <p class="mt-1 text-[10px] text-gray-400 border border-dashed border-gray-200 rounded px-1.5 py-0.5 inline-block">{hint}</p>
    {/if}
  {:else}
    <div class="mt-2 flex items-end gap-2 flex-wrap">
      <p class="text-2xl font-bold text-gray-900 tabular-nums leading-none">{value}</p>
      {#if deltaPct != null}
        <span class="inline-flex items-center gap-0.5 text-xs font-semibold rounded-full px-1.5 py-0.5
                     {good ? 'text-emerald-700 bg-emerald-50' : 'text-red-600 bg-red-50'}">
          <svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
            {#if up}<path d="M12 5l7 7h-4v7h-6v-7H5z"/>{:else}<path d="M12 19l-7-7h4V5h6v7h4z"/>{/if}
          </svg>
          {deltaText}
        </span>
      {/if}
    </div>
    {#if series.length > 1}
      <div class="mt-3"><Sparkline data={series} color={accent} /></div>
    {/if}
  {/if}
</div>
