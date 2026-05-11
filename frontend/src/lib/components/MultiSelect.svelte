<script lang="ts">
  interface Option {
    value: string;
    label: string;
  }

  interface Props {
    options: Option[];
    selected: string[];
    placeholder?: string;
    onChange: (values: string[]) => void;
  }

  let { options, selected, placeholder = 'Select…', onChange }: Props = $props();

  let open = $state(false);
  let query = $state('');
  let containerEl: HTMLDivElement | undefined = $state();

  // Defensive dedup: SvelteKit's update({ reset: false }) flow during form
  // saves can briefly hand the component a transient `options` snapshot with
  // duplicate values, which would crash the keyed each-blocks below with
  // each_key_duplicate. Same for `selected` after rapid toggle clicks.
  const dedupedOptions = $derived.by(() => {
    const seen = new Set<string>();
    return options.filter((o) => (seen.has(o.value) ? false : (seen.add(o.value), true)));
  });
  const dedupedSelected = $derived.by(() => {
    const seen = new Set<string>();
    return selected.filter((v) => (seen.has(v) ? false : (seen.add(v), true)));
  });

  const labelOf = $derived((v: string) => dedupedOptions.find((o) => o.value === v)?.label ?? v);

  const filtered = $derived(
    query.trim() === ''
      ? dedupedOptions
      : dedupedOptions.filter((o) => {
          const q = query.toLowerCase();
          return o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q);
        })
  );

  function toggle(value: string) {
    const next = dedupedSelected.includes(value)
      ? dedupedSelected.filter((v) => v !== value)
      : [...dedupedSelected, value];
    onChange(next);
  }

  function remove(value: string) {
    onChange(dedupedSelected.filter((v) => v !== value));
  }

  function handleClickOutside(e: MouseEvent) {
    if (containerEl && !containerEl.contains(e.target as Node)) {
      open = false;
      query = '';
    }
  }

  $effect(() => {
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  });
</script>

<div class="relative" bind:this={containerEl}>
  <button
    type="button"
    onclick={() => (open = !open)}
    class="w-full min-h-[44px] border border-gray-200 rounded-xl px-3 py-2 text-sm text-left
           focus:outline-none focus:ring-2 focus:ring-gray-900 flex flex-wrap gap-1.5 items-center
           hover:border-gray-300 transition-colors"
  >
    {#if dedupedSelected.length === 0}
      <span class="text-gray-400">{placeholder}</span>
    {:else}
      {#each dedupedSelected as value (value)}
        <span class="inline-flex items-center gap-1 bg-gray-100 text-gray-800 text-xs rounded-md pl-2 pr-1 py-0.5">
          {labelOf(value)}
          <button
            type="button"
            onclick={(e) => { e.stopPropagation(); remove(value); }}
            class="text-gray-400 hover:text-gray-700 leading-none px-1"
            aria-label="Remove {labelOf(value)}"
          >×</button>
        </span>
      {/each}
    {/if}
  </button>

  {#if open}
    <div class="absolute z-20 mt-1 w-full bg-white border border-gray-200 rounded-xl shadow-lg max-h-80 overflow-hidden flex flex-col">
      <div class="p-2 border-b border-gray-100">
        <input
          type="text"
          bind:value={query}
          placeholder="Search…"
          class="w-full px-3 py-1.5 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-gray-900"
        />
      </div>
      <ul class="overflow-y-auto py-1 flex-1">
        {#each filtered as opt (opt.value)}
          {@const isSelected = selected.includes(opt.value)}
          <li>
            <button
              type="button"
              onclick={() => toggle(opt.value)}
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 hover:bg-gray-50 transition-colors
                     {isSelected ? 'bg-gray-50' : ''}"
            >
              <span class="w-4 h-4 inline-flex items-center justify-center rounded border
                           {isSelected ? 'bg-gray-900 border-gray-900' : 'border-gray-300'}">
                {#if isSelected}
                  <svg viewBox="0 0 16 16" class="w-3 h-3 text-white" fill="none" stroke="currentColor" stroke-width="2.5">
                    <path d="M3 8l3 3 7-7" />
                  </svg>
                {/if}
              </span>
              <span class="flex-1 text-gray-800">{opt.label}</span>
            </button>
          </li>
        {:else}
          <li class="px-3 py-2 text-sm text-gray-400">No results</li>
        {/each}
      </ul>
    </div>
  {/if}
</div>
