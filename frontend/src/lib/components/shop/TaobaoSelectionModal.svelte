<script lang="ts">
  /**
   * Taobao-layout PDP add-to-cart modal (desktop-1 / mobile-1).
   *
   * Surfaces the parent product's variants ("基本裝") and its curated promo
   * bundle products ("優惠套裝") in a single selection sheet. Clicking any
   * row updates the top summary (image, price, excerpt, CTA). Clicking the
   * top image opens the secondary image-browser modal.
   *
   * Only mounted when the taobao layout is active for the current product.
   */
  import type { Product, Variant, PromoBundle, ProductImage } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { trackAddToCart } from '$lib/tracker';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import { EMPTY_REFS, type ShortcodeRefs } from '$lib/shortcodes/types';

  let {
    open = $bindable(false),
    product,
    variants,
    images,
    promoBundles,
    cannotPurchaseLabel,
    onOpenImageBrowser,
    refs = EMPTY_REFS
  }: {
    open: boolean;
    product: Product;
    variants: Variant[];
    images: ProductImage[];
    promoBundles: PromoBundle[];
    cannotPurchaseLabel: string;
    onOpenImageBrowser: (index: number) => void;
    refs?: ShortcodeRefs;
  } = $props();

  type Selection = {
    kind: 'variant' | 'bundle';
    id: string;
    variantId: string;
    name: string;
    image: string | null;
    price: number;
    compareAtPrice: number | null;
    stockQty: number;
    excerpt: string | null;
    purchasable: boolean;
  };

  function primaryImageURL(): string | null {
    const sorted = [...images].sort((a, b) => {
      if (a.is_primary && !b.is_primary) return -1;
      if (!a.is_primary && b.is_primary) return 1;
      return a.sort_order - b.sort_order;
    });
    return sorted[0]?.url ?? product.primary_image_url ?? null;
  }

  function variantImage(v: Variant): string | null {
    return v.image_url ?? images.find((img) => img.variant_id === v.id)?.url ?? primaryImageURL();
  }

  // Variant display name: prefer the variant's own `name`, then its SKU
  // (matches the existing PDP's variant selector), then the parent product
  // name as a last-resort. The current Gyeon dataset has variants without
  // a `name` so SKU is what users actually see.
  function variantDisplayName(v: Variant): string {
    if (v.name && v.name.trim()) return v.name;
    if (v.sku && v.sku.trim()) return v.sku;
    return product.name;
  }

  function variantToSelection(v: Variant): Selection {
    return {
      kind: 'variant',
      id: v.id,
      variantId: v.id,
      name: variantDisplayName(v),
      image: variantImage(v),
      price: v.price,
      compareAtPrice: v.compare_at_price ?? null,
      stockQty: v.stock_qty,
      // Variants don't carry their own excerpt — fall back to product excerpt.
      excerpt: product.excerpt ?? null,
      // Variants inherit the parent product's role-purchase gate — every
      // variant of a blocked product is itself blocked.
      purchasable: product.purchasable !== false
    };
  }

  function bundleToSelection(b: PromoBundle): Selection {
    return {
      kind: 'bundle',
      id: b.bundle_product_id,
      variantId: b.variant_id,
      name: b.name,
      image: b.primary_image_url ?? null,
      price: b.price,
      compareAtPrice: b.compare_at_price ?? null,
      stockQty: b.stock_qty,
      excerpt: b.excerpt ?? product.excerpt ?? null,
      purchasable: b.purchasable !== false
    };
  }

  // Default selection = first IN-STOCK variant, then first in-stock promo
  // bundle. Falls back to the first row regardless of stock only when
  // everything is sold out (in which case the PDP CTA is already disabled
  // and the modal won't open anyway).
  const initial = $derived.by(() => {
    const firstInStockVariant = variants.find((v) => v.stock_qty > 0);
    if (firstInStockVariant) return variantToSelection(firstInStockVariant);
    const firstInStockBundle = promoBundles.find((b) => b.stock_qty > 0);
    if (firstInStockBundle) return bundleToSelection(firstInStockBundle);
    return variants[0]
      ? variantToSelection(variants[0])
      : promoBundles[0]
        ? bundleToSelection(promoBundles[0])
        : null;
  });
  let selected = $state<Selection | null>(initial);
  // Reset selection whenever the modal is freshly opened so re-entering the
  // flow always lands on the default first-variant state. Without this,
  // closing on slide N and re-opening would silently re-use slide N.
  $effect(() => {
    if (open && initial) selected = initial;
  });

  let qty = $state(1);
  let adding = $state(false);
  let added = $state(false);

  // Sorted "items" used by the image browser modal — same order as the
  // visible list (variants then promo bundles). Each entry maps 1:1 to a
  // slide in TaobaoImageBrowserModal.
  const items = $derived<Selection[]>([
    ...variants.map(variantToSelection),
    ...promoBundles.map(bundleToSelection)
  ]);

  function selectedIndex(): number {
    if (!selected) return 0;
    const i = items.findIndex(
      (it) => it.kind === selected!.kind && it.id === selected!.id
    );
    return i < 0 ? 0 : i;
  }

  function pick(s: Selection) {
    selected = s;
  }

  async function addToCart() {
    if (!selected || selected.stockQty <= 0 || !selected.purchasable || adding) return;
    adding = true;
    try {
      await cartStore.add(selected.variantId, qty);
      trackAddToCart({
        id: selected.kind === 'variant' ? product.id : selected.id,
        name: selected.name,
        price: selected.price,
        quantity: qty
      });
      added = true;
      setTimeout(() => {
        added = false;
        open = false;
      }, 800);
    } finally {
      adding = false;
    }
  }

  function close() { open = false; }

  // ESC to close. Body scroll lock is handled by toggling a class via $effect
  // (rather than touching document directly) so SSR stays safe.
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) close();
  }
  $effect(() => {
    if (typeof document === 'undefined') return;
    if (open) {
      const prev = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      return () => { document.body.style.overflow = prev; };
    }
  });

  function fmtMoney(n: number): string {
    return `HK$${Math.round(n) === n ? n : n.toFixed(2)}`;
  }
</script>

<svelte:window onkeydown={onKey} />

{#if open && selected}
  <!-- Backdrop. Click closes the modal so it behaves like the platform
       standard. inert prevents scroll/tab leaking behind. -->
  <div class="fixed inset-0 z-[200] bg-black/40 flex items-end sm:items-center justify-center"
       role="dialog" aria-modal="true" aria-label="Add to cart"
       onclick={close}
       onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') close(); }}
       tabindex="-1">
    <!-- Inner sheet — stops click propagation so chrome on the panel
         doesn't dismiss. Mobile: bottom sheet, full width, capped height.
         Desktop: centered card. -->
    <div class="relative bg-white w-full sm:max-w-3xl sm:rounded-2xl rounded-t-2xl shadow-xl
                max-h-[92vh] flex flex-col overflow-hidden"
         role="document"
         onclick={(e) => e.stopPropagation()}
         onkeydown={(e) => e.stopPropagation()}
         tabindex="-1">
      <!-- Close button (top-right) -->
      <button type="button" aria-label="Close" onclick={close}
              class="absolute top-3 right-3 p-2 text-paper0 hover:text-ink-900 z-10">
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6l-12 12"/></svg>
      </button>

      <!-- ── Top summary ─────────────────────────────────────────── -->
      <div class="flex items-start gap-4 p-5 sm:p-6 border-b border-ink-300/40">
        <button type="button" aria-label="View image"
                onclick={() => onOpenImageBrowser(selectedIndex())}
                class="shrink-0 w-24 h-24 sm:w-28 sm:h-28 rounded overflow-hidden bg-paper">
          {#if selected.image}
            <ResponsiveImage src={selected.image} alt={selected.name}
                             widths={[160, 240, 320]} sizes="120px"
                             class="w-full h-full object-cover" />
          {/if}
        </button>
        <div class="flex-1 min-w-0">
          <div class="text-2xl sm:text-3xl font-display font-bold text-navy-500 mb-2">
            {fmtMoney(selected.price)}
          </div>
          {#if selected.excerpt}
            <div class="text-sm text-ink-900 leading-relaxed prose prose-sm max-w-none">
              <MarkdownContent content={selected.excerpt} {refs} />
            </div>
          {/if}
        </div>
      </div>

      <!-- Qty stepper -->
      <div class="px-5 sm:px-6 py-3 border-b border-ink-300/40">
        <div class="inline-flex items-center border border-ink-300 rounded">
          <button type="button" aria-label="Decrease quantity"
                  onclick={() => (qty = Math.max(1, qty - 1))}
                  class="px-4 py-2 text-lg text-ink-900 hover:bg-paper">−</button>
          <input type="number" min="1" bind:value={qty}
                 class="w-14 text-center bg-transparent border-0 focus:outline-none focus:ring-0 text-sm" />
          <button type="button" aria-label="Increase quantity"
                  onclick={() => (qty += 1)}
                  class="px-4 py-2 text-lg text-ink-900 hover:bg-paper">+</button>
        </div>
      </div>

      <!-- ── Lists ────────────────────────────────────────────────── -->
      <div class="flex-1 overflow-y-auto px-5 sm:px-6 py-4 space-y-5">
        {#if variants.length > 0}
          <section>
            <h3 class="text-sm font-semibold text-ink-900 mb-2">基本裝</h3>
            <ul class="space-y-2">
              {#each variants as v (v.id)}
                {@const isSel = selected!.kind === 'variant' && selected!.id === v.id}
                {@const rowPurchasable = product.purchasable !== false}
                <li>
                  <button type="button" onclick={() => pick(variantToSelection(v))}
                          class="w-full flex items-center gap-3 px-3 py-2.5 rounded border text-left transition-colors
                                 {isSel ? 'border-navy-500 bg-navy-500/5' : 'border-ink-300/40 hover:border-ink-300'}
                                 {rowPurchasable ? '' : 'opacity-50'}">
                    <span class="w-10 h-10 shrink-0 bg-paper rounded overflow-hidden">
                      {#if variantImage(v)}
                        <ResponsiveImage src={variantImage(v)!} alt=""
                                         widths={[80, 160]} sizes="40px"
                                         class="w-full h-full object-cover" />
                      {/if}
                    </span>
                    <span class="flex-1 text-sm text-ink-900 truncate">{variantDisplayName(v)}</span>
                    <span class="text-sm font-semibold text-navy-500 whitespace-nowrap">{fmtMoney(v.price)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if promoBundles.length > 0}
          <section>
            <h3 class="text-sm font-semibold text-ink-900 mb-2">優惠套裝</h3>
            <ul class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {#each promoBundles as pb (pb.id)}
                {@const isSel = selected!.kind === 'bundle' && selected!.id === pb.bundle_product_id}
                {@const rowPurchasable = pb.purchasable !== false}
                <li>
                  <button type="button" onclick={() => pick(bundleToSelection(pb))}
                          class="w-full flex items-center gap-3 px-3 py-2.5 rounded border text-left transition-colors
                                 {isSel ? 'border-navy-500 bg-navy-500/5' : 'border-ink-300/40 hover:border-ink-300'}
                                 {rowPurchasable ? '' : 'opacity-50'}">
                    <span class="w-10 h-10 shrink-0 bg-paper rounded overflow-hidden">
                      {#if pb.primary_image_url}
                        <ResponsiveImage src={pb.primary_image_url} alt=""
                                         widths={[80, 160]} sizes="40px"
                                         class="w-full h-full object-cover" />
                      {/if}
                    </span>
                    <span class="flex-1 text-xs sm:text-sm text-ink-900 line-clamp-2">{pb.name}</span>
                    <span class="text-sm font-semibold text-navy-500 whitespace-nowrap">{fmtMoney(pb.price)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      </div>

      <!-- Sticky CTA -->
      <div class="border-t border-ink-300/40 p-4 sm:p-5">
        <button type="button" onclick={addToCart}
                disabled={selected.stockQty <= 0 || !selected.purchasable || adding}
                class="w-full py-3.5 bg-navy-500 text-white text-sm font-semibold rounded
                       hover:bg-navy-700 transition-colors
                       disabled:opacity-50 disabled:cursor-not-allowed">
          {#if added}
            ✓ 已加入購物車
          {:else if adding}
            加入中…
          {:else if !selected.purchasable}
            {cannotPurchaseLabel}
          {:else if selected.stockQty <= 0}
            缺貨
          {:else}
            加入購物車
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
