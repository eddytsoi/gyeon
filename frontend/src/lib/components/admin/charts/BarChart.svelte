<script lang="ts">
  // Pure-Tailwind horizontal bar chart. No SVG needed since horizontal bars
  // map cleanly to flex rows + percentage widths.
  interface Bar { label: string; value: number }
  interface Props {
    data: Bar[];
    formatValue?: (n: number) => string;
  }
  let { data, formatValue = (n: number) => String(n) }: Props = $props();
  const max = $derived(Math.max(1, ...data.map((d) => d.value)));
</script>

{#if data.length === 0}
  <p class="text-sm text-gray-400 py-6 text-center">No data</p>
{:else}
  <div class="space-y-2">
    {#each data as bar}
      {@const pct = (bar.value / max) * 100}
      <div>
        <div class="flex items-baseline justify-between text-xs mb-0.5">
          <span class="text-gray-700 truncate">{bar.label}</span>
          <span class="text-gray-500 font-mono ml-2">{formatValue(bar.value)}</span>
        </div>
        <div class="h-2 bg-gray-100 rounded-full overflow-hidden">
          <div class="h-full bg-gray-900 rounded-full transition-[width] duration-300"
               style="width: {pct}%;"></div>
        </div>
      </div>
    {/each}
  </div>
{/if}
