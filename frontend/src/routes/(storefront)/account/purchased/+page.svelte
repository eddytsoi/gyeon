<script lang="ts">
  // 曾經購買 — a product-level view of everything the customer has ever bought,
  // most-recently-purchased first. Unlike the Order History page (which is
  // transaction-level), this de-duplicates across orders so consumables can be
  // re-ordered in one tap. Each row expands to its provenance: which order(s)
  // and — for bundle components — which bundle it came in.
  //
  // The provenance list arrives from the server load (authenticated endpoint);
  // current price / stock / image are hydrated here via the public product
  // APIs (same approach as the Wishlist page) so buy-again reflects live data.
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { getProductByID, getProductImages, getProductVariants } from '$lib/api';
  import type { Product, ProductImage, Variant, PurchasedProduct } from '$lib/types';
  import { formatHKD } from '$lib/money';
  import { variantSuffix } from '$lib/variant';
  import { formatOrderDate } from '$lib/datetime';
  import { isVideo } from '$lib/media';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // A hydrated row. `product` is undefined for history-only entries — products
  // (or their variants) deleted since purchase, which we still surface for the
  // record but can't re-order or link.
  type Card = {
    purchased: PurchasedProduct;
    product?: Product;
    image?: ProductImage;
    variant?: Variant; // cheapest live variant — display default & single-variant fast path
    variants: Variant[]; // ALL live variants, cheapest-first; [] for unavailable rows
    variantCount: number;
  };

  let cards = $state<Card[]>([]);
  let loading = $state(true);
  let adding = $state<Record<string, boolean>>({});
  let added = $state<Record<string, boolean>>({});
  let open = $state<Record<string, boolean>>({});
  // Multi-variant buy-again: which row's spec picker is expanded, and the
  // variant the user has selected within it. Both keyed by keyOf, mirroring
  // the adding/added/open records above.
  let picking = $state<Record<string, boolean>>({});
  let selectedId = $state<Record<string, string>>({});

  // Stable per-row key: product id when known, else the snapshot name bucket
  // (mirrors the backend grouping key).
  const keyOf = (p: PurchasedProduct) => p.product_id ?? `name:${p.product_name.toLowerCase()}`;

  const orderCount = (p: PurchasedProduct) => new Set(p.sources.map((s) => s.order_id)).size;

  async function hydrate(list: PurchasedProduct[]) {
    if (list.length === 0) {
      cards = [];
      loading = false;
      return;
    }
    cards = await Promise.all(
      list.map(async (purchased): Promise<Card> => {
        if (!purchased.product_id) {
          return { purchased, variants: [], variantCount: 0 };
        }
        try {
          const [product, images, variants] = await Promise.all([
            getProductByID(purchased.product_id),
            getProductImages(purchased.product_id).catch(() => [] as ProductImage[]),
            getProductVariants(purchased.product_id).catch(() => [] as Variant[])
          ]);
          const sorted = variants.slice().sort((a, b) => a.price - b.price);
          return {
            purchased,
            product,
            image: images.find((i) => i.is_primary) ?? images[0],
            variant: sorted[0],
            variants: sorted,
            variantCount: sorted.length
          };
        } catch {
          // Product gone since purchase — fall back to history-only.
          return { purchased, variants: [], variantCount: 0 };
        }
      })
    );
    loading = false;
  }

  onMount(() => {
    hydrate(data.purchased);
  });

  function thumb(image?: ProductImage): string | null {
    if (!image) return null;
    if (isVideo(image)) return image.thumbnail_url ?? null;
    return image.thumbnail_url ?? image.url;
  }

  async function addToCart(card: Card) {
    if (!card.variant) return;
    const k = keyOf(card.purchased);
    adding = { ...adding, [k]: true };
    try {
      await cartStore.add(card.variant.id);
      added = { ...added, [k]: true };
      setTimeout(() => {
        added = { ...added, [k]: false };
      }, 2000);
    } catch {
      // cartStore records the error; the storefront layout shows the toast.
    } finally {
      adding = { ...adding, [k]: false };
    }
  }

  // ── Multi-variant inline spec picker ──────────────────────────────
  // Cheapest in-stock variant, else cheapest overall. `variants` is already
  // sorted cheapest-first, so the first in-stock entry is the cheapest one.
  function defaultVariant(card: Card): Variant | undefined {
    return card.variants.find((v) => v.stock_qty > 0) ?? card.variants[0];
  }

  // The variant the row should price and that "add" will use: the user's pick
  // if any (and still resolvable), otherwise the row default.
  function activeVariant(card: Card): Variant | undefined {
    const id = selectedId[keyOf(card.purchased)];
    return (id ? card.variants.find((v) => v.id === id) : undefined) ?? defaultVariant(card);
  }

  function togglePicker(card: Card) {
    const k = keyOf(card.purchased);
    const next = !picking[k];
    // Seed the default selection on first open so a price shows and "add"
    // works without an extra click.
    if (next && !selectedId[k]) {
      const def = defaultVariant(card);
      if (def) selectedId = { ...selectedId, [k]: def.id };
    }
    picking = { ...picking, [k]: next };
  }

  function selectVariant(card: Card, v: Variant) {
    if (v.stock_qty <= 0) return; // chips are disabled, but guard anyway
    selectedId = { ...selectedId, [keyOf(card.purchased)]: v.id };
  }

  // Add the active (selected, or default) variant for a multi-variant row.
  // Mirrors addToCart's adding/added 2s feedback, reusing the same records.
  async function addSelected(card: Card) {
    const v = activeVariant(card);
    if (!v || v.stock_qty <= 0) return;
    const k = keyOf(card.purchased);
    adding = { ...adding, [k]: true };
    try {
      await cartStore.add(v.id);
      added = { ...added, [k]: true };
      setTimeout(() => {
        added = { ...added, [k]: false };
      }, 2000);
    } catch {
      // cartStore records the error; the storefront layout shows the toast.
    } finally {
      adding = { ...adding, [k]: false };
    }
  }
</script>

<svelte:head>
  <title>{m.purchased_title()}</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <div class="flex items-center justify-between gap-3">
    <h1 class="text-xl font-bold text-gray-900">{m.purchased_heading()}</h1>
    {#if cards.length > 0}
      <span class="text-sm text-gray-500 shrink-0">
        {cards.length === 1
          ? m.purchased_count_one({ count: cards.length })
          : m.purchased_count_many({ count: cards.length })}
      </span>
    {/if}
  </div>

  {#if loading}
    <div class="flex flex-col gap-3">
      {#each Array(3) as _}
        <div class="h-32 bg-gray-100 rounded-2xl animate-pulse"></div>
      {/each}
    </div>
  {:else if cards.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">{m.purchased_empty()}</p>
      <a href="/products" class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline">
        {m.purchased_empty_cta()}
      </a>
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each cards as card (keyOf(card.purchased))}
        {@const p = card.purchased}
        {@const k = keyOf(p)}
        {@const src = thumb(card.image)}
        {@const multi = card.variantCount > 1}
        {@const picker = picking[k] === true}
        {@const sel = activeVariant(card)}
        {@const soldOut = card.variants.length > 0 && card.variants.every((v) => v.stock_qty === 0)}
        {@const hasDiscount = sel?.compare_at_price != null && sel.compare_at_price > sel.price}
        {@const expanded = open[k] === true}
        <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div class="p-4 flex items-start gap-4">
            <!-- Thumbnail -->
            {#if card.product}
              <a href="/products/{card.product.slug}" class="shrink-0">
                {#if src}
                  <img
                    {src}
                    alt={card.image?.alt_text ?? card.product.name}
                    loading="lazy"
                    class="w-20 h-20 sm:w-24 sm:h-24 object-cover rounded-xl bg-gray-50"
                  />
                {:else}
                  <div class="w-20 h-20 sm:w-24 sm:h-24 rounded-xl bg-gray-100 flex items-center justify-center text-gray-300">
                    <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1" aria-hidden="true">
                      <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909M3.75 21h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v13.5A1.5 1.5 0 0 0 3.75 21Z" />
                    </svg>
                  </div>
                {/if}
              </a>
            {:else}
              <div class="w-20 h-20 sm:w-24 sm:h-24 rounded-xl bg-gray-100 flex items-center justify-center text-gray-300 shrink-0">
                <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909M3.75 21h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v13.5A1.5 1.5 0 0 0 3.75 21Z" />
                </svg>
              </div>
            {/if}

            <!-- Name + meta + price + actions -->
            <div class="flex-1 min-w-0">
              {#if card.product}
                <a
                  href="/products/{card.product.slug}"
                  class="block font-medium text-gray-900 uppercase line-clamp-2 hover:underline"
                >
                  {card.product.name}
                </a>
              {:else}
                <p class="font-medium text-gray-900 uppercase line-clamp-2">{p.product_name}</p>
              {/if}

              <!-- Provenance summary -->
              <p class="mt-1 text-xs text-gray-400">
                {m.purchased_meta_last({ date: formatOrderDate(p.last_purchased_at) })}
                · {p.total_quantity === 1
                  ? m.purchased_meta_qty_one({ count: p.total_quantity })
                  : m.purchased_meta_qty_many({ count: p.total_quantity })}
                · {orderCount(p) === 1
                  ? m.purchased_meta_orders_one({ count: orderCount(p) })
                  : m.purchased_meta_orders_many({ count: orderCount(p) })}
              </p>

              {#if card.product && sel}
                <div class="mt-1.5 flex items-baseline gap-2">
                  <span class="text-base font-bold text-gray-900">{formatHKD(sel.price)}</span>
                  {#if hasDiscount}
                    <span class="text-sm text-gray-400 line-through">{formatHKD(sel.compare_at_price!)}</span>
                  {/if}
                </div>
              {/if}

              <div class="mt-3 flex items-center gap-2">
                {#if !card.product}
                  <span class="px-3 py-1.5 text-xs font-medium text-gray-500 bg-gray-100 rounded-lg">
                    {m.purchased_unavailable()}
                  </span>
                {:else if soldOut}
                  <span class="px-3 py-1.5 text-xs font-medium text-gray-500 bg-gray-100 rounded-lg">
                    {m.product_card_out_of_stock()}
                  </span>
                {:else if multi}
                  <button
                    type="button"
                    onclick={() => togglePicker(card)}
                    aria-expanded={picker}
                    aria-controls="purchased-picker-{k}"
                    class="px-3 py-1.5 text-sm font-medium text-gray-900 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
                  >
                    {m.wishlist_select_options()}
                  </button>
                  {#if picker}
                    <button
                      type="button"
                      disabled={adding[k] || !sel || sel.stock_qty <= 0}
                      onclick={() => addSelected(card)}
                      class="px-3 py-1.5 text-sm font-medium text-white bg-gray-900 rounded-lg hover:bg-gray-700 transition-colors disabled:opacity-60"
                    >
                      {added[k] ? m.product_detail_added() : m.product_detail_add_to_cart()}
                    </button>
                  {/if}
                {:else if card.variant}
                  <button
                    type="button"
                    disabled={adding[k]}
                    onclick={() => addToCart(card)}
                    class="px-3 py-1.5 text-sm font-medium text-white bg-gray-900 rounded-lg hover:bg-gray-700 transition-colors disabled:opacity-60"
                  >
                    {added[k] ? m.product_detail_added() : m.product_detail_add_to_cart()}
                  </button>
                {/if}
              </div>

              <!-- Inline spec picker (multi-variant rows) -->
              {#if card.product && !soldOut && multi && picker}
                <div
                  id="purchased-picker-{k}"
                  transition:slide={{ duration: 200, easing: cubicOut }}
                  class="mt-2.5 flex flex-wrap gap-2"
                >
                  {#each card.variants as v (v.id)}
                    {@const isSelected = sel?.id === v.id}
                    {@const isAvailable = v.stock_qty > 0}
                    <button
                      type="button"
                      onclick={() => selectVariant(card, v)}
                      disabled={!isAvailable}
                      aria-pressed={isSelected}
                      class="px-3 py-1.5 rounded-lg text-sm font-medium border transition-colors
                             {isSelected
                               ? 'bg-gray-900 border-gray-900 text-white'
                               : isAvailable
                                 ? 'border-gray-300 text-gray-900 hover:bg-gray-50'
                                 : 'border-gray-200 text-gray-400 line-through cursor-not-allowed opacity-60'}"
                    >
                      {variantSuffix(v.name) || v.sku}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <!-- Provenance expand toggle -->
          <button
            type="button"
            onclick={() => (open = { ...open, [k]: !expanded })}
            aria-expanded={expanded}
            aria-controls="purchased-src-{k}"
            class="w-full flex items-center justify-between gap-3 px-4 py-2.5 border-t border-gray-100 text-left text-xs font-medium text-gray-500 hover:text-gray-900 hover:bg-gray-50 transition-colors"
          >
            <span>{m.purchased_view_sources()}</span>
            <svg
              class="w-4 h-4 transition-transform duration-200 {expanded ? 'rotate-180' : ''}"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="m6 9 6 6 6-6" />
            </svg>
          </button>

          {#if expanded}
            <div id="purchased-src-{k}" transition:slide={{ duration: 220, easing: cubicOut }}
                 class="px-4 pb-4 pt-1 flex flex-col gap-2">
              {#each p.sources as s}
                <div class="flex items-center justify-between gap-3 text-sm">
                  <div class="min-w-0">
                    <a href="/account/orders/{s.order_id}" class="font-medium text-gray-700 hover:underline">
                      {m.purchased_source_order({ number: s.order_number })}
                    </a>
                    <span class="text-gray-400"> · {formatOrderDate(s.purchased_at)}</span>
                    {#if s.bundle_name}
                      <span class="mt-0.5 block text-xs text-navy-500">
                        {m.purchased_from_bundle({ name: s.bundle_name })}
                      </span>
                    {/if}
                  </div>
                  <span class="shrink-0 text-gray-500 tabular-nums">{m.purchased_source_qty({ count: s.quantity })}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>
