<script lang="ts">
  // Minimal inline sparkline for KPI cards — no axes, labels, or ticks. Just a
  // single accent-coloured line with a dot on the last point.
  interface Props {
    data?: number[];
    color?: string;
    height?: number;
  }
  let { data = [], color = '#6366f1', height = 36 }: Props = $props();

  let w = $state(120);
  const pad = 3;

  const min = $derived(data.length ? Math.min(...data) : 0);
  const max = $derived(data.length ? Math.max(...data) : 1);

  function x(i: number): number {
    if (data.length <= 1) return w / 2;
    return pad + (i / (data.length - 1)) * (w - 2 * pad);
  }
  function y(v: number): number {
    if (max === min) return height / 2;
    return pad + (height - 2 * pad) - ((v - min) / (max - min)) * (height - 2 * pad);
  }
  const path = $derived(
    data.length ? data.map((v, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ') : ''
  );
</script>

<div bind:clientWidth={w} class="w-full" style="height: {height}px;">
  {#if data.length > 1}
    <svg width={w} {height} aria-hidden="true" class="block overflow-visible">
      <path d={path} fill="none" stroke={color} stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
      <circle cx={x(data.length - 1)} cy={y(data[data.length - 1])} r="2" fill={color} />
    </svg>
  {/if}
</div>
