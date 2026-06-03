import {
  adminGetNavMenus,
  adminGetNavMenu,
  adminAddNavItem,
  adminUpdateNavItem,
  adminDeleteNavItem,
  type NavMenu,
  type NavItem
} from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const menus = await adminGetNavMenus(token).catch(() => [] as NavMenu[]);

  const sortedMenus = menus.sort((a, b) => {
    if (a.handle === 'header') return -1;
    if (b.handle === 'header') return 1;
    return a.name.localeCompare(b.name);
  });

  // Default to the header menu (sortedMenus[0] is header when present),
  // otherwise the first available menu. URL ?menu=… still wins.
  const headerMenu = sortedMenus.find((m) => m.handle === 'header');
  const selectedID =
    url.searchParams.get('menu') ?? headerMenu?.id ?? sortedMenus[0]?.id ?? '';
  const selected = selectedID
    ? await adminGetNavMenu(token, selectedID).catch(() => null)
    : null;

  return { menus: sortedMenus, selected, selectedID };
};

function extractHiddenRoles(data: FormData): string[] {
  const roles: string[] = [];
  if (data.get('hide_customer')) roles.push('customer');
  if (data.get('hide_installer')) roles.push('installer');
  if (data.get('hide_installer_v2')) roles.push('installer_v2');
  return roles;
}

export const actions: Actions = {
  addItem: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const menuID = data.get('menu_id') as string;
    try {
      await adminAddNavItem(token, menuID, {
        label: (data.get('label') as string).trim(),
        url: (data.get('url') as string).trim(),
        target: (data.get('target') as string) || '_self',
        sort_order: parseInt(data.get('sort_order') as string) || 0,
        parent_id: (data.get('parent_id') as string) || undefined,
        hidden_for_roles: extractHiddenRoles(data)
      });
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to add item' });
    }
  },

  updateItem: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const menuID = data.get('menu_id') as string;
    const itemID = data.get('item_id') as string;
    try {
      await adminUpdateNavItem(token, menuID, itemID, {
        label: (data.get('label') as string).trim(),
        url: (data.get('url') as string).trim(),
        target: (data.get('target') as string) || '_self',
        sort_order: parseInt(data.get('sort_order') as string) || 0,
        parent_id: (data.get('parent_id') as string) || undefined,
        hidden_for_roles: extractHiddenRoles(data)
      });
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to update item' });
    }
  },

  deleteItem: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const menuID = data.get('menu_id') as string;
    const itemID = data.get('item_id') as string;
    try {
      await adminDeleteNavItem(token, menuID, itemID);
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to delete item' });
    }
  }
};
