import {
  adminListCampaigns,
  adminListCoupons,
  adminDeleteCampaign,
  adminDeleteCoupon
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';

  // Independent page cursors per tab so navigating one doesn't reset the other.
  const campaignsPage = Math.max(1, parseInt(url.searchParams.get('cp') ?? '1', 10) || 1);
  const couponsPage = Math.max(1, parseInt(url.searchParams.get('up') ?? '1', 10) || 1);

  const [campaignsRes, couponsRes] = await Promise.all([
    adminListCampaigns(token, PAGE_SIZE, (campaignsPage - 1) * PAGE_SIZE).catch(() => ({ items: [], total: 0 })),
    adminListCoupons(token, PAGE_SIZE, (couponsPage - 1) * PAGE_SIZE).catch(() => ({ items: [], total: 0 }))
  ]);
  return {
    campaigns: campaignsRes.items,
    campaignsTotal: campaignsRes.total,
    campaignsPage,
    coupons: couponsRes.items,
    couponsTotal: couponsRes.total,
    couponsPage,
    pageSize: PAGE_SIZE
  };
};

export const actions: Actions = {
  deleteCampaign: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const id = (await request.formData()).get('id') as string;
    try {
      await adminDeleteCampaign(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete campaign' });
    }
    redirect(303, '/admin/discounts');
  },
  deleteCoupon: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const id = (await request.formData()).get('id') as string;
    try {
      await adminDeleteCoupon(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete coupon' });
    }
    redirect(303, '/admin/discounts');
  }
};
