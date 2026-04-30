<script lang="ts">
  interface Props {
    value: string;
    placeholder?: string;
    delay?: number;
    onChange: (q: string) => void;
  }

  let { value, placeholder = 'Search…', delay = 250, onChange }: Props = $props();

  // Local mirror so typing stays responsive while the debounce decides
  // whether to fire onChange. Re-syncs whenever the parent's value changes
  // (e.g. browser back/forward updating ?q=).
  let local = $state(value);
  $effect(() => {
    local = value;
  });

  let timer: ReturnType<typeof setTimeout> | undefined;

  function fire(next: string) {
    if (timer) clearTimeout(timer);
    if (next === value) return;
    timer = setTimeout(() => onChange(next), delay);
  }

  function onInput(e: Event) {
    local = (e.currentTarget as HTMLInputElement).value;
    fire(local);
  }

  function clear() {
    if (timer) clearTimeout(timer);
    local = '';
    if (value !== '') onChange('');
  }
</script>

<div class="relative w-full sm:max-w-xs">
  <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-400">
    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="m21 21-4.3-4.3M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z" />
    </svg>
  </span>
  <input
    type="search"
    value={local}
    {placeholder}
    oninput={onInput}
    autocomplete="off"
    spellcheck="false"
    class="w-full pl-9 pr-9 py-2 text-sm rounded-xl border border-gray-200 bg-white
           focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900
           placeholder:text-gray-400" />
  {#if local}
    <button
      type="button"
      onclick={clear}
      aria-label="Clear search"
      class="absolute inset-y-0 right-2 flex items-center justify-center w-6 h-full text-gray-400 hover:text-gray-700">
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
      </svg>
    </button>
  {/if}
</div>

<style>
  input[type="search"]::-webkit-search-cancel-button,
  input[type="search"]::-webkit-search-decoration {
    -webkit-appearance: none;
    appearance: none;
    display: none;
  }
</style>
