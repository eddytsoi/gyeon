<script lang="ts">
  // Renders the per-promotion description block that sits beneath the
  // discount line on the storefront checkout / success / account-order
  // summaries. Each row shows the promotion name and (when set) the
  // admin-authored description in small grey text, so a shopper sees why
  // they got the discount before they pay — and again on the receipt.
  //
  // Empty array / all-blank descriptions → renders nothing (keeps the
  // visual unchanged for orders without active promotions, including
  // imported / pre-migration orders).
  type Promotion = {
    kind?: string;
    id?: string;
    name?: string;
    code?: string;
    description?: string | null;
    amount?: number;
  };

  let { promotions = [] }: { promotions?: Promotion[] } = $props();

  const visible = $derived(
    (promotions ?? []).filter((p) => (p.description ?? '').trim() !== '')
  );
</script>

{#if visible.length > 0}
  <ul class="border-t border-gray-100 pt-2 flex flex-col gap-1">
    {#each visible as p}
      <li class="text-xs text-gray-500 leading-snug">
        <span class="text-gray-700">{p.name ?? p.code ?? ''}</span>
        <span class="text-gray-400"> — </span>
        <span>{p.description}</span>
      </li>
    {/each}
  </ul>
{/if}
