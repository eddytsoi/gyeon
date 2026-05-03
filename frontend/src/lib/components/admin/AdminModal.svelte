<script lang="ts">
  import type { Snippet } from 'svelte';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    open: boolean;
    onClose: () => void;
    size?: 'sm' | 'md';
    ariaLabel?: string;
    children: Snippet;
  }

  let { open, onClose, size = 'sm', ariaLabel, children }: Props = $props();

  const maxWidth = $derived(size === 'md' ? 'max-w-md' : 'max-w-sm');
  const resolvedLabel = $derived(ariaLabel ?? m.admin_modal_close());

  function onKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') onClose();
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={onClose} role="button" tabindex="-1" aria-label={resolvedLabel}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full {maxWidth}">
      {@render children()}
    </div>
  </div>
{/if}
