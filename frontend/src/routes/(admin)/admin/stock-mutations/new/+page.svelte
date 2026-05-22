<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    adminCreateStockMutation,
    adminExecuteStockMutation,
    StockMutationInsufficientStockError,
    type StockMutationType,
    type StockMutationConflict
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import ProductPicker, { type ProductPickerAddPayload } from '$lib/components/admin/ProductPicker.svelte';
  import MutationItemsTable, { type MutationItemRow } from '$lib/components/admin/MutationItemsTable.svelte';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  const token = data.token!;

  let type = $state<StockMutationType>('in');
  let note = $state('');
  let items = $state<MutationItemRow[]>([]);
  let saving = $state(false);
  let conflicts = $state<StockMutationConflict[] | null>(null);

  let nextKey = 0;
  function addItem(payload: ProductPickerAddPayload) {
    const variant = payload.variant;
    const isBundle = payload.productKind === 'bundle';
    // Reject duplicate top-level variant — the backend's partial unique
    // index (mutation_id, variant_id) WHERE parent_item_id IS NULL enforces
    // this for both kinds. A bundle's parent variant counts as a top-level
    // variant, so picking the same bundle twice is also blocked here.
    // (Components inside a bundle are still free to repeat across bundles —
    // the partial index ignores child rows.)
    if (items.some((it) => it.variantId === variant.id)) {
      notify.error(
        m.admin_stock_mutations_duplicate_variant_title(),
        m.admin_stock_mutations_duplicate_variant_body()
      );
      return;
    }
    nextKey += 1;
    items = [
      ...items,
      {
        key: `${variant.id}-${nextKey}`,
        variantId: variant.id,
        productName: payload.productName,
        sku: variant.sku,
        variantName: variant.name,
        primaryImageUrl: payload.primaryImageUrl ?? null,
        quantity: payload.quantity,
        currentStock: isBundle ? null : (variant.stock_qty ?? null),
        kind: isBundle ? 'bundle' : 'simple',
        components: isBundle
          ? payload.bundleItems.map((bi) => ({
              variantId: bi.component_variant_id,
              productName: bi.display_name_override || bi.component_product_name || '',
              sku: bi.component_sku || '',
              variantName: bi.component_variant_name ?? null,
              primaryImageUrl: bi.component_primary_image_url ?? null,
              perParentQuantity: bi.quantity,
              currentStock: bi.component_stock_qty ?? null
            }))
          : undefined
      }
    ];
  }

  function changeQty(key: string, qty: number) {
    items = items.map((it) => (it.key === key ? { ...it, quantity: qty } : it));
  }
  function removeRow(key: string) {
    items = items.filter((it) => it.key !== key);
  }

  function setType(t: StockMutationType) {
    if (items.length > 0 && t !== type) {
      if (!confirm(m.admin_stock_mutations_confirm_change_type())) return;
      items = [];
    }
    type = t;
  }

  async function save({ thenExecute }: { thenExecute: boolean }) {
    if (items.length === 0) {
      notify.error(
        m.admin_stock_mutations_no_items_title(),
        m.admin_stock_mutations_no_items_body()
      );
      return;
    }
    saving = true;
    conflicts = null;
    try {
      const created = await adminCreateStockMutation(token, {
        type,
        note: note.trim() || null,
        items: items.map((it) => ({ variant_id: it.variantId, quantity: it.quantity }))
      });
      if (!thenExecute) {
        notify.success(m.admin_stock_mutations_saved_success({ id: created.mutation_number }));
        goto(`/admin/stock-mutations/${created.id}`);
        return;
      }
      try {
        await adminExecuteStockMutation(token, created.id);
        notify.success(m.admin_stock_mutations_saved_and_executed({ id: created.mutation_number }));
        goto(`/admin/stock-mutations/${created.id}`);
      } catch (e) {
        if (e instanceof StockMutationInsufficientStockError) {
          conflicts = e.conflicts;
          notify.error(
            m.admin_stock_mutations_insufficient_stock_not_executed(),
            m.admin_stock_mutations_insufficient_stock_saved_as_draft()
          );
          goto(`/admin/stock-mutations/${created.id}`);
          return;
        }
        throw e;
      }
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_save_failure(),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head><title>{m.admin_stock_mutations_new()} · {m.admin_stock_mutations_heading()}</title></svelte:head>

<div class="space-y-4 max-w-4xl">
  <div class="flex items-center gap-3">
    <a href="/admin/stock-mutations" class="text-sm text-gray-500 hover:text-gray-700">← {m.admin_stock_mutations_back()}</a>
    <h1 class="text-2xl font-semibold text-gray-900">{m.admin_stock_mutations_new()}</h1>
  </div>

  <!-- Type selector -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
    <h2 class="text-sm font-medium text-gray-900">{m.admin_stock_mutations_section_direction()}</h2>
    <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-sm">
      <button class="px-4 py-2 {type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}"
              onclick={() => setType('in')} type="button">{m.admin_stock_mutations_button_in()}</button>
      <button class="px-4 py-2 border-l border-gray-200 {type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}"
              onclick={() => setType('out')} type="button">{m.admin_stock_mutations_button_out()}</button>
    </div>
    <p class="text-xs text-gray-500">
      {#if type === 'in'}
        {@html m.admin_stock_mutations_direction_hint_in()}
      {:else}
        {@html m.admin_stock_mutations_direction_hint_out()}
      {/if}
    </p>
  </section>

  <!-- Items -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-4">
    <h2 class="text-sm font-medium text-gray-900">{m.admin_stock_mutations_section_items()}</h2>
    <ProductPicker {token} mode="variant-only" onAdd={addItem} />
    <MutationItemsTable {items} {type} onChangeQty={changeQty} onRemove={removeRow} />
  </section>

  <!-- Note -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-2">
    <label for="note" class="text-sm font-medium text-gray-900">{m.admin_stock_mutations_section_note_optional()}</label>
    <textarea id="note" bind:value={note} rows="3"
              class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900"
              placeholder={m.admin_stock_mutations_note_placeholder()}></textarea>
  </section>

  <!-- Actions -->
  <div class="flex flex-wrap justify-end gap-2">
    <a href="/admin/stock-mutations" class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50">{m.admin_stock_mutations_action_discard()}</a>
    <button onclick={() => save({ thenExecute: false })} disabled={saving}
            class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50">
      {saving ? '…' : m.admin_stock_mutations_action_save_draft()}
    </button>
    <button onclick={() => save({ thenExecute: true })} disabled={saving}
            class="px-4 py-2 text-sm rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50">
      {saving ? '…' : m.admin_stock_mutations_action_save_execute()}
    </button>
  </div>

  {#if conflicts && conflicts.length > 0}
    <section class="bg-red-50 border border-red-200 rounded-xl p-4 space-y-2">
      <h3 class="text-sm font-medium text-red-800">{m.admin_stock_mutations_conflict_heading()}</h3>
      <ul class="text-xs text-red-700 space-y-1">
        {#each conflicts as c}
          <li>
            • <strong>{c.product_name ?? c.variant_id}</strong>
            ({c.variant_sku ?? '—'}) — {m.admin_stock_mutations_conflict_line({ requested: String(c.requested), available: String(c.available) })}
          </li>
        {/each}
      </ul>
    </section>
  {/if}
</div>
