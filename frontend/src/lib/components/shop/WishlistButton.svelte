<script lang="ts">
  import { wishlistStore } from '$lib/stores/wishlist.svelte';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    productID: string;
    /** "icon" — small heart for ProductCard; "full" — labeled button for PDP. */
    variant?: 'icon' | 'full';
    class?: string;
  }

  let { productID, variant = 'icon', class: extraClass = '' }: Props = $props();

  const active = $derived(wishlistStore.has(productID));
  let pending = $state(false);

  async function onClick(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (pending) return;
    pending = true;
    try {
      await wishlistStore.toggle(productID);
    } finally {
      pending = false;
    }
  }
</script>

{#if variant === 'icon'}
  <button type="button" onclick={onClick}
          aria-label={active ? m.wishlist_remove() : m.wishlist_add()}
          aria-pressed={active}
          disabled={pending}
          class="inline-flex items-center justify-center w-8 h-8 rounded-full
                 bg-white/90 backdrop-blur transition-colors
                 hover:bg-white shadow-sm disabled:opacity-60 {extraClass}">
    <svg class="w-4 h-4 transition-colors {active ? 'text-red-500 fill-current' : 'text-gray-500'}"
         aria-hidden="true"
         viewBox="0 0 24 24" fill={active ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8">
      <path stroke-linecap="round" stroke-linejoin="round"
            d="M21 8.25c0-2.485-2.099-4.5-4.688-4.5-1.935 0-3.597 1.126-4.312 2.733-.715-1.607-2.377-2.733-4.313-2.733C5.1 3.75 3 5.765 3 8.25c0 7.22 9 12 9 12s9-4.78 9-12Z" />
    </svg>
  </button>
{:else}
  <button type="button" onclick={onClick}
          aria-pressed={active}
          disabled={pending}
          class="inline-flex items-center justify-center gap-2 px-5 py-3 rounded-full
                 border text-sm font-medium transition-colors disabled:opacity-60
                 {active ? 'border-red-200 bg-red-50 text-red-600 hover:bg-red-100' : 'border-gray-200 text-gray-700 hover:border-gray-900'}
                 {extraClass}">
    <svg class="w-4 h-4 {active ? 'fill-current' : ''}"
         aria-hidden="true"
         viewBox="0 0 24 24" fill={active ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8">
      <path stroke-linecap="round" stroke-linejoin="round"
            d="M21 8.25c0-2.485-2.099-4.5-4.688-4.5-1.935 0-3.597 1.126-4.312 2.733-.715-1.607-2.377-2.733-4.313-2.733C5.1 3.75 3 5.765 3 8.25c0 7.22 9 12 9 12s9-4.78 9-12Z" />
    </svg>
    {active ? m.wishlist_added() : m.wishlist_add()}
  </button>
{/if}
