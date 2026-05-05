import {
  adminListCampaigns,
  adminListCoupons,
  adminDeleteCampaign,
  adminDeleteCoupon,
  type Campaign,
  type Coupon
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [campaigns, coupons] = await Promise.all([
    adminListCampaigns(token).catch(() => [] as Campaign[]),
    adminListCoupons(token).catch(() => [] as Coupon[])
  ]);
  return { campaigns, coupons };
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
