<script lang="ts">
  import { cartStore } from '$lib/stores/cart.svelte';
  import { encodeSharedCart } from '$lib/cartShare';
  import * as m from '$lib/paraglide/messages';

  // Builds a shareable URL of the current cart and either invokes the native
  // share sheet (mobile) or copies to clipboard (desktop), flashing an inline
  // "copied" confirmation. Clipboard usage mirrors admin/media +
  // admin/PasswordInput, which already use navigator.clipboard.writeText.
  let copied = $state(false);
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  function buildUrl(): string {
    const items = (cartStore.cart?.items ?? []).map((i) => ({
      variantId: i.variant_id,
      quantity: i.quantity
    }));
    return `${window.location.origin}/cart/shared?c=${encodeSharedCart(items)}`;
  }

  function flashCopied() {
    copied = true;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => (copied = false), 2000);
  }

  async function share() {
    const url = buildUrl();
    // Web Share API (mobile) lets the user pick WhatsApp etc directly.
    if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
      try {
        await navigator.share({ title: m.cart_share_native_title(), url });
        return;
      } catch {
        // user dismissed the sheet, or share failed — fall through to clipboard
      }
    }
    try {
      await navigator.clipboard.writeText(url);
      flashCopied();
    } catch {
      // clipboard blocked (insecure context / denied permission) — last resort
      window.prompt(m.cart_share_button(), url);
    }
  }
</script>

{#if (cartStore.cart?.items?.length ?? 0) > 0}
  <button
    type="button"
    onclick={share}
    class="w-full py-3 inline-flex items-center justify-center gap-2
           border border-gray-300 text-gray-700 font-medium rounded-xl
           hover:bg-gray-50 transition-colors text-sm">
    {#if copied}
      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
           viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
      </svg>
      {m.cart_share_copied()}
    {:else}
      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
           viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round"
              d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
      </svg>
      {m.cart_share_button()}
    {/if}
  </button>
{/if}
