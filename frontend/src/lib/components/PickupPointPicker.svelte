<script lang="ts">
  import { onMount } from 'svelte';
  import { listShipanyPickupPoints, type ShipanyPickupPoint } from '$lib/api';
  import { HK_DISTRICTS } from '$lib/data/hk-districts';
  import * as m from '$lib/paraglide/messages';

  type Props = {
    carrier: string;
    carrierName: string;
    initialDistrict?: string;
    onSelect: (point: ShipanyPickupPoint) => void;
    onClose: () => void;
  };
  let { carrier, carrierName, initialDistrict = '', onSelect, onClose }: Props = $props();

  let district = $state(initialDistrict);
  let search = $state('');
  let points = $state<ShipanyPickupPoint[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function load() {
    loading = true;
    error = '';
    try {
      points = await listShipanyPickupPoints(carrier);
    } catch (e) {
      error = e instanceof Error ? e.message : m.pickup_load_failed();
      points = [];
    } finally {
      loading = false;
    }
  }

  onMount(load);

  const filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return points.filter((p) => {
      if (district && p.district && p.district !== district) return false;
      if (q && !`${p.name} ${p.address}`.toLowerCase().includes(q)) return false;
      return true;
    });
  });
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
  <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
       onclick={onClose}
       role="button" tabindex="-1" aria-label={m.pickup_aria_close()}></div>
  <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-lg max-h-[80vh] flex flex-col">
    <div class="flex items-start justify-between gap-4 mb-3">
      <div>
        <h3 class="font-semibold text-gray-900">{m.pickup_heading()}</h3>
        <p class="text-xs text-gray-400 mt-0.5">{carrierName}</p>
      </div>
      <button type="button" onclick={onClose}
              class="text-gray-400 hover:text-gray-700 transition-colors text-sm">{m.pickup_close_button()}</button>
    </div>

    <div class="grid grid-cols-2 gap-2 mb-3">
      <select bind:value={district}
              class="border border-gray-200 rounded-xl px-3 py-2 text-sm bg-white
                     focus:outline-none focus:ring-2 focus:ring-gray-900">
        <option value="">{m.pickup_district_all()}</option>
        {#each HK_DISTRICTS as d}
          <option value={d}>{d}</option>
        {/each}
      </select>
      <input type="search" bind:value={search} placeholder={m.pickup_search_placeholder()}
             class="border border-gray-200 rounded-xl px-3 py-2 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900" />
    </div>

    <div class="flex-1 overflow-y-auto -mx-2 px-2">
      {#if loading}
        <div class="text-center py-12 text-sm text-gray-400">{m.pickup_loading()}</div>
      {:else if error}
        <div class="text-center py-12 text-sm text-red-500">{error}</div>
      {:else if filtered.length === 0}
        <div class="text-center py-12 text-sm text-gray-400">{m.pickup_no_results()}</div>
      {:else}
        <ul class="flex flex-col divide-y divide-gray-100">
          {#each filtered as p}
            <li>
              <button type="button"
                      onclick={() => onSelect(p)}
                      class="w-full text-left py-3 px-2 hover:bg-gray-50 transition-colors rounded-lg">
                <p class="text-sm font-medium text-gray-900">{p.name}</p>
                <p class="text-xs text-gray-500 mt-0.5 leading-relaxed">{p.address}</p>
                {#if p.district}
                  <p class="text-[11px] text-gray-400 mt-0.5">{p.district}</p>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
</div>
