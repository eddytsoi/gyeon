<script lang="ts">
  // Account-scoped wishlist — a management-style list (one row per saved item)
  // that lives inside the My Account shell. The public /wishlist page keeps its
  // marketing grid; this view is for the logged-in customer to review, quick-add
  // to cart and remove. Auth is enforced by account/+layout.server.ts, so we can
  // assume an authenticated session here (no guest/login-prompt branch).
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { wishlistStore } from '$lib/stores/wishlist.svelte';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { getProductByID, getProductImages, getProductVariants } from '$lib/api';
  import type { Product, ProductImage, Variant } from '$lib/types';
  import { formatHKD } from '$lib/money';
  import { variantSuffix } from '$lib/variant';
  import { isVideo } from '$lib/media';
  import * as m from '$lib/paraglide/messages';

  // `variant` is the cheapest live variant (display default & single-variant
  // fast path); `variants` is ALL live variants, cheapest-first, for the inline
  // spec picker on multi-variant rows.
  type Card = {
    product: Product;
    image?: ProductImage;
    variant?: Variant;
    variants: Variant[];
    variantCount: number;
  };
  let cards = $state<Card[]>([]);
  let loading = $state(true);
  // Transient per-product add-to-cart state (keyed by product id).
  let adding = $state<Record<string, boolean>>({});
  let added = $state<Record<string, boolean>>({});
  // Multi-variant buy-in-place: which row's spec picker is expanded, and the
  // variant the user has selected within it. Both keyed by product id, mirroring
  // the adding/added records above.
  let picking = $state<Record<string, boolean>>({});
  let selectedId = $state<Record<string, string>>({});

  // Wishlist only stores product IDs, so hydrate each into product + primary
  // image + cheapest variant (mirrors the public /wishlist page). variantCount
  // lets us send multi-variant products to the PDP instead of silently adding
  // an arbitrary size to the cart.
  async function hydrate(productIDs: string[]) {
    if (productIDs.length === 0) {
      cards = [];
      loading = false;
      return;
    }
    const results = await Promise.all(
      productIDs.map(async (pid): Promise<Card | null> => {
        try {
          const [product, images, variants] = await Promise.all([
            getProductByID(pid),
            getProductImages(pid).catch(() => [] as ProductImage[]),
            getProductVariants(pid).catch(() => [] as Variant[])
          ]);
          const sorted = variants.slice().sort((a, b) => a.price - b.price);
          return {
            product,
            image: images.find((i) => i.is_primary) ?? images[0],
            variant: sorted[0],
            variants: sorted,
            variantCount: sorted.length
          } satisfies Card;
        } catch {
          return null;
        }
      })
    );
    cards = results.filter((x): x is Card => x !== null);
    loading = false;
  }

  onMount(async () => {
    await wishlistStore.init(true);
    await hydrate(wishlistStore.ids);
  });

  $effect(() => {
    // Re-hydrate when ids change (e.g. the customer removes a row here).
    hydrate(wishlistStore.ids);
  });

  function thumb(image?: ProductImage): string | null {
    if (!image) return null;
    // Videos only have a usable still if a thumbnail was generated.
    if (isVideo(image)) return image.thumbnail_url ?? null;
    return image.thumbnail_url ?? image.url;
  }

  async function addToCart(card: Card) {
    if (!card.variant) return;
    adding = { ...adding, [card.product.id]: true };
    try {
      await cartStore.add(card.variant.id);
      added = { ...added, [card.product.id]: true };
      setTimeout(() => {
        added = { ...added, [card.product.id]: false };
      }, 2000);
    } catch {
      // cartStore records the error; the storefront layout shows the toast.
    } finally {
      adding = { ...adding, [card.product.id]: false };
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
    const id = selectedId[card.product.id];
    return (id ? card.variants.find((v) => v.id === id) : undefined) ?? defaultVariant(card);
  }

  function togglePicker(card: Card) {
    const k = card.product.id;
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
    selectedId = { ...selectedId, [card.product.id]: v.id };
  }

  // Add the active (selected, or default) variant for a multi-variant row.
  // Mirrors addToCart's adding/added 2s feedback, reusing the same records.
  async function addSelected(card: Card) {
    const v = activeVariant(card);
    if (!v || v.stock_qty <= 0) return;
    const k = card.product.id;
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
  <title>{m.wishlist_title()}</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-bold text-gray-900">{m.wishlist_heading()}</h1>
    {#if cards.length > 0}
      <span class="text-sm text-gray-500">
        {cards.length === 1
          ? m.wishlist_count_one({ count: cards.length })
          : m.wishlist_count_many({ count: cards.length })}
      </span>
    {/if}
  </div>

  {#if loading}
    <div class="flex flex-col gap-3">
      {#each Array(3) as _}
        <div class="h-28 bg-gray-100 rounded-2xl animate-pulse"></div>
      {/each}
    </div>
  {:else if cards.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">{m.wishlist_empty()}</p>
      <a href="/products" class="mt-3 inline-block text-sm font-medium text-gray-900 hover:underline">
        {m.wishlist_empty_cta()}
      </a>
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each cards as card (card.product.id)}
        {@const k = card.product.id}
        {@const src = thumb(card.image)}
        {@const multi = card.variantCount > 1}
        {@const picker = picking[k] === true}
        {@const sel = activeVariant(card)}
        {@const soldOut = card.variants.length > 0 && card.variants.every((v) => v.stock_qty === 0)}
        {@const hasDiscount = sel?.compare_at_price != null && sel.compare_at_price > sel.price}
        <div class="bg-white rounded-2xl border border-gray-100 p-4 flex items-start gap-4">
          <!-- Thumbnail -->
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

          <!-- Name + price + actions -->
          <div class="flex-1 min-w-0">
            <a
              href="/products/{card.product.slug}"
              class="block font-medium text-gray-900 uppercase line-clamp-2 hover:underline"
            >
              {card.product.name}
            </a>

            {#if sel}
              <div class="mt-1 flex items-baseline gap-2">
                <span class="text-base font-bold text-gray-900">{formatHKD(sel.price)}</span>
                {#if hasDiscount}
                  <span class="text-sm text-gray-400 line-through">{formatHKD(sel.compare_at_price!)}</span>
                {/if}
              </div>
            {:else}
              <p class="mt-1 text-sm text-gray-400">{m.product_card_no_variants()}</p>
            {/if}

            <div class="mt-3 flex items-center gap-2">
              {#if soldOut}
                <span class="px-3 py-1.5 text-xs font-medium text-gray-500 bg-gray-100 rounded-lg">
                  {m.product_card_out_of_stock()}
                </span>
              {:else if multi}
                <button
                  type="button"
                  onclick={() => togglePicker(card)}
                  aria-expanded={picker}
                  aria-controls="wishlist-picker-{k}"
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
            {#if !soldOut && multi && picker}
              <div
                id="wishlist-picker-{k}"
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

          <!-- Remove -->
          <button
            type="button"
            onclick={() => wishlistStore.remove(card.product.id)}
            aria-label={m.wishlist_remove()}
            title={m.wishlist_remove()}
            class="shrink-0 text-gray-400 hover:text-red-600 transition-colors p-1 -m-1"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>
