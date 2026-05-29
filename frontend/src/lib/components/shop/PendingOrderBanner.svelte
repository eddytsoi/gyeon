<script lang="ts">
  import { getCartPendingOrder, type CartPendingOrder } from '$lib/api';
  import * as m from '$lib/paraglide/messages';

  // Shows an amber "you have an unpaid order" banner when the current cart
  // already has an outstanding pending order, linking to /pay/{id} so the
  // shopper resumes the same order instead of creating a duplicate.
  let { cartId }: { cartId: string | null | undefined } = $props();

  let pending = $state<CartPendingOrder | null>(null);

  $effect(() => {
    const id = cartId;
    if (!id) {
      pending = null;
      return;
    }
    let cancelled = false;
    getCartPendingOrder(id).then((res) => {
      if (!cancelled) pending = res;
    });
    return () => {
      cancelled = true;
    };
  });
</script>

{#if pending}
  <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 mb-6">
    <p class="text-sm text-amber-800">
      {m.cart_pending_order_banner({ orderNumber: pending.order_number })}
    </p>
    <a
      href={`/pay/${pending.order_id}?cs=${encodeURIComponent(pending.client_secret)}`}
      class="inline-block mt-2 text-sm font-medium text-amber-900 underline">
      {m.cart_pending_order_continue()}
    </a>
  </div>
{/if}
