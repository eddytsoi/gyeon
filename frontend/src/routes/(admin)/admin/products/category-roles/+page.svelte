<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import type { CategoryRule, CustomerRole } from '$lib/types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  const ROLES: CustomerRole[] = ['customer', 'installer'];

  // Resolves the enum value to its localized label. Centralized so the role
  // header cell and any future per-role copy stay in sync.
  function roleLabel(role: CustomerRole): string {
    return role === 'installer' ? m.admin_role_installer() : m.admin_role_customer();
  }

  // Build a local state map keyed by `${role}::${category_id}` so each cell
  // can flip its can_view / can_purchase flags independently. Missing rows
  // default to allowed — the backend stores only the negative cases, so a
  // fresh load returns just the restrictions.
  type Cell = { can_view: boolean; can_purchase: boolean };
  function buildState(rules: CategoryRule[]): Record<string, Cell> {
    const out: Record<string, Cell> = {};
    for (const r of rules) {
      out[`${r.role}::${r.category_id}`] = {
        can_view: r.can_view,
        can_purchase: r.can_purchase
      };
    }
    return out;
  }
  let state = $state<Record<string, Cell>>(buildState(data.rules));
  let saving = $state(false);

  function cellFor(role: CustomerRole, categoryID: string): Cell {
    return state[`${role}::${categoryID}`] ?? { can_view: true, can_purchase: true };
  }

  function setCell(role: CustomerRole, categoryID: string, patch: Partial<Cell>) {
    const key = `${role}::${categoryID}`;
    const current = state[key] ?? { can_view: true, can_purchase: true };
    state[key] = { ...current, ...patch };
  }

  // Serialise for submit: only include rows where at least one dimension is
  // restricted. Default-allowed rows are encoded as "no row" — matches the
  // backend's storage model.
  const payload = $derived.by<string>(() => {
    const rules: CategoryRule[] = [];
    for (const role of ROLES) {
      for (const c of data.categories) {
        const cell = cellFor(role, c.id);
        if (!cell.can_view || !cell.can_purchase) {
          rules.push({ role, category_id: c.id, can_view: cell.can_view, can_purchase: cell.can_purchase });
        }
      }
    }
    return JSON.stringify(rules);
  });

  // Helper: toggling "can view" off should also force "can purchase" off,
  // since you can't buy what you can't see. The reverse is allowed — a role
  // may see a category but be blocked from buying from it.
  function onViewToggle(role: CustomerRole, categoryID: string, checked: boolean) {
    if (!checked) {
      setCell(role, categoryID, { can_view: false, can_purchase: false });
    } else {
      setCell(role, categoryID, { can_view: true });
    }
  }

  function onPurchaseToggle(role: CustomerRole, categoryID: string, checked: boolean) {
    setCell(role, categoryID, { can_purchase: checked });
  }
</script>

<svelte:head>
  <title>{m.admin_category_roles_title()}</title>
</svelte:head>

<div class="max-w-5xl">
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
        await update();
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
              <th colspan="2" class="text-center px-3 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide border-l border-gray-100">
                {roleLabel(role)}
              </th>
            {/each}
          </tr>
          <tr class="bg-gray-50 border-b border-gray-100">
            <th></th>
            {#each ROLES as role (role)}
              <th class="text-center px-3 py-2 text-[10px] font-medium text-gray-400 uppercase tracking-wider border-l border-gray-100">{m.admin_category_roles_col_view()}</th>
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
              <td colspan={1 + ROLES.length * 2} class="px-5 py-8 text-center text-gray-400 text-sm">
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
