import {
  adminGetCategories,
  adminGetCategoryRules,
  adminSaveCategoryRules
} from '$lib/api/admin';
import type { Category, CategoryRule, CustomerRole } from '$lib/types';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [categories, rulesRes] = await Promise.all([
    adminGetCategories(token).catch(() => [] as Category[]),
    adminGetCategoryRules(token).catch(() => ({ rules: [] as CategoryRule[] }))
  ]);
  return { categories, rules: rulesRes.rules };
};

const ROLES: CustomerRole[] = ['customer', 'installer'];

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    // The form posts one field per (role, category, dimension) checkbox using
    // a `${role}::${categoryID}::${dimension}` key. A missing key for either
    // dimension means "unchecked" — checkbox semantics, not "leave as-is".
    const raw = data.get('payload');
    if (typeof raw !== 'string') return fail(400, { error: 'missing payload' });
    let rules: CategoryRule[];
    try {
      rules = JSON.parse(raw) as CategoryRule[];
    } catch {
      return fail(400, { error: 'invalid payload' });
    }
    // Sanity: filter rule rows to known roles so a tampered form can't
    // sneak through enum values the backend would reject.
    rules = rules.filter((r) => ROLES.includes(r.role));
    try {
      await adminSaveCategoryRules(token, rules);
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to save category rules' });
    }
  }
};
