<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import type { CategoryRule, CustomerRole } from '$lib/types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const ROLES: CustomerRole[] = ['customer', 'installer', 'installer_v2'];

  // Resolves the enum value to its localized label. Centralized so the role
  // header cell and any future per-role copy stay in sync.
  function roleLabel(role: CustomerRole): string {
    if (role === 'installer') return m.admin_role_installer();
    if (role === 'installer_v2') return m.admin_role_installer_v2();
    return m.admin_role_customer();
  }

  // Build a local state map keyed by `${role}::${category_id}` so each cell
  // can flip its can_view / is_listed / can_purchase flags independently.
  // Missing rows default to fully allowed — the backend stores only the
  // negative cases, so a fresh load returns just the restrictions. Defaulting
  // is_listed/can_purchase to true here matches the DB default and what
  // SaveBulk will drop from the row set when round-tripping a fully-allowed
  // category.
  type Cell = { can_view: boolean; is_listed: boolean; can_purchase: boolean };
  const ALL_ALLOWED: Cell = { can_view: true, is_listed: true, can_purchase: true };

  function buildState(rules: CategoryRule[]): Record<string, Cell> {
    const out: Record<string, Cell> = {};
    for (const r of rules) {
      out[`${r.role}::${r.category_id}`] = {
        can_view: r.can_view,
        is_listed: r.is_listed,
        can_purchase: r.can_purchase
      };
    }
    return out;
  }
  let state = $state<Record<string, Cell>>(buildState(data.rules));
  let saving = $state(false);

  function cellFor(role: CustomerRole, categoryID: string): Cell {
    return state[`${role}::${categoryID}`] ?? { ...ALL_ALLOWED };
  }

  function setCell(role: CustomerRole, categoryID: string, patch: Partial<Cell>) {
    const key = `${role}::${categoryID}`;
    const current = state[key] ?? { ...ALL_ALLOWED };
    state[key] = { ...current, ...patch };
  }

  // Serialise for submit: only include rows where at least one dimension is
  // restricted. Default-allowed rows are encoded as "no row" — matches the
  // backend's storage model (SaveBulk drops these too).
  const payload = $derived.by<string>(() => {
    const rules: CategoryRule[] = [];
    for (const role of ROLES) {
      for (const c of data.categories) {
        const cell = cellFor(role, c.id);
        if (!cell.can_view || !cell.is_listed || !cell.can_purchase) {
          rules.push({
            role,
            category_id: c.id,
            can_view: cell.can_view,
            is_listed: cell.is_listed,
            can_purchase: cell.can_purchase
          });
        }
      }
    }
    return JSON.stringify(rules);
  });

  // Checkbox interactions. The matrix surfaces the implication chain
  // (!can_view → !is_listed → ... folded into a "fully blocked" state)
  // through enable/disable on dependent checkboxes — the underlying state
  // still stores the three flags independently so re-checking restores
  // each dimension explicitly.
  //
  // Re-toggling **View** on restores BOTH is_listed and can_purchase to
  // true. This avoids the previous silent-trap where re-checking View
  // would re-show the row in listings but leave can_purchase=false, leaving
  // installer "visible but unpurchasable" with no obvious UI cue.
  function onViewToggle(role: CustomerRole, categoryID: string, checked: boolean) {
    if (!checked) {
      setCell(role, categoryID, { can_view: false, is_listed: false, can_purchase: false });
    } else {
      setCell(role, categoryID, { can_view: true, is_listed: true, can_purchase: true });
    }
  }

  // Untoggling List doesn't auto-disable Buy — a category may be unlisted
  // (private link) yet still purchasable for someone who reaches the PDP
  // directly. Re-toggling List doesn't touch Buy either.
  function onListToggle(role: CustomerRole, categoryID: string, checked: boolean) {
    setCell(role, categoryID, { is_listed: checked });
  }

  function onPurchaseToggle(role: CustomerRole, categoryID: string, checked: boolean) {
    setCell(role, categoryID, { can_purchase: checked });
  }
</script>

<svelte:head>
  <title>{m.admin_category_roles_title()}</title>
</svelte:head>

<div class="max-w-6xl">
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/products/categories" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">{m.admin_category_roles_back()}</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">{m.admin_category_roles_heading()}</h1>
  </div>

  <p class="text-sm text-gray-500 mb-6 max-w-2xl">
    {m.admin_category_roles_description()}
  </p>

  {#if form?.success}
    <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700">{m.admin_category_roles_saved()}</div>
  {/if}
  {#if form?.error}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">{form.error}</div>
  {/if}

  <form
    method="POST"
    action="?/save"
    use:enhance={() => {
      saving = true;
      return async ({ update }) => {
        // reset:false → don't call form.reset() after the action. The default
        // would blank every checkbox in the form to its initial DOM state,
        // and Svelte's one-way checked={...} binding wouldn't re-sync because
        // the underlying state values didn't change.
        await update({ reset: false });
        saving = false;
      };
    }}
  >
    <input type="hidden" name="payload" value={payload} />

    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-100">
          <tr>
            <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_category_roles_col_category()}</th>
            {#each ROLES as role (role)}
              <th colspan="3" class="text-center px-3 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide border-l border-gray-100">
                {roleLabel(role)}
              </th>
            {/each}
          </tr>
          <tr class="bg-gray-50 border-b border-gray-100">
            <th></th>
            {#each ROLES as role (role)}
              <th class="text-center px-3 py-2 text-[10px] font-medium text-gray-400 uppercase tracking-wider border-l border-gray-100">{m.admin_category_roles_col_view()}</th>
              <th class="text-center px-3 py-2 text-[10px] font-medium text-gray-400 uppercase tracking-wider">{m.admin_category_roles_col_list()}</th>
              <th class="text-center px-3 py-2 text-[10px] font-medium text-gray-400 uppercase tracking-wider">{m.admin_category_roles_col_purchase()}</th>
            {/each}
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          {#each data.categories as c (c.id)}
            <tr class="hover:bg-gray-50/50">
              <td class="px-5 py-3">
                <div class="text-sm font-medium text-gray-900">{c.name}</div>
                <div class="text-xs text-gray-400 font-mono">{c.slug}</div>
              </td>
              {#each ROLES as role (role)}
                {@const cell = cellFor(role, c.id)}
                <td class="text-center px-3 py-3 border-l border-gray-100">
                  <input
                    type="checkbox"
                    checked={cell.can_view}
                    onchange={(e) => onViewToggle(role, c.id, (e.currentTarget as HTMLInputElement).checked)}
                    class="h-4 w-4 rounded border-gray-300"
                  />
                </td>
                <td class="text-center px-3 py-3">
                  <input
                    type="checkbox"
                    checked={cell.is_listed}
                    disabled={!cell.can_view}
                    onchange={(e) => onListToggle(role, c.id, (e.currentTarget as HTMLInputElement).checked)}
                    class="h-4 w-4 rounded border-gray-300 disabled:opacity-30"
                  />
                </td>
                <td class="text-center px-3 py-3">
                  <input
                    type="checkbox"
                    checked={cell.can_purchase}
                    disabled={!cell.can_view}
                    onchange={(e) => onPurchaseToggle(role, c.id, (e.currentTarget as HTMLInputElement).checked)}
                    class="h-4 w-4 rounded border-gray-300 disabled:opacity-30"
                  />
                </td>
              {/each}
            </tr>
          {/each}
          {#if data.categories.length === 0}
            <tr>
              <td colspan={1 + ROLES.length * 3} class="px-5 py-8 text-center text-gray-400 text-sm">
                {m.admin_category_roles_empty_pre()}<a class="underline" href="/admin/products/categories">{m.admin_category_roles_back()}</a>{m.admin_category_roles_empty_post()}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <div class="mt-6 flex justify-end">
      <SaveButton
        loading={saving}
        class="inline-flex items-center justify-center gap-1.5 px-5 py-2 text-sm font-semibold text-white bg-gray-900 rounded-lg hover:bg-gray-700 disabled:opacity-50"
      >
        {m.admin_category_roles_save()}
      </SaveButton>
    </div>
  </form>
</div>
