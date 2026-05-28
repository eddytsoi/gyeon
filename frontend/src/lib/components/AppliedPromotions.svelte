<script lang="ts">
  // Renders the per-promotion block that sits beneath the discount line on
  // the storefront checkout / success / account-order summaries. Each row
  // shows the promotion name (always, when the discount applied) and the
  // admin-authored description if one was set, so a shopper sees why they
  // got the discount before they pay — and again on the receipt.
  //
  // Description is optional in the admin form; we no longer hide the entire
  // row when it's blank, otherwise an applied discount would be unexplained.
  // Empty array (e.g. imported / pre-migration orders) still renders nothing.
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
    (promotions ?? []).filter((p) => (p.name ?? p.code ?? '').trim() !== '')
  );
</script>

{#if visible.length > 0}
  <ul class="border-t border-gray-100 pt-2 flex flex-col gap-1">
    {#each visible as p}
      <li class="text-xs text-gray-500 leading-snug">
        <span class="text-gray-700">{p.name ?? p.code ?? ''}</span>
        {#if (p.description ?? '').trim() !== ''}
          <span class="text-gray-400"> — </span>
          <span>{p.description}</span>
        {/if}
      </li>
    {/each}
  </ul>
{/if}
