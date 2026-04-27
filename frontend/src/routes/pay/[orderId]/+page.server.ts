import { error } from '@sveltejs/kit';
import { getOrderPaymentInfo } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, url }) => {
  const cs = url.searchParams.get('cs');
  if (!cs) {
    throw error(400, '缺少付款驗證碼');
  }
  try {
    const info = await getOrderPaymentInfo(params.orderId, cs);
    return { info };
  } catch (e) {
    const msg = e instanceof Error ? e.message : '';
    if (msg.includes('410')) {
      throw error(410, '此訂單已完成付款或已不可付款');
    }
    if (msg.includes('401')) {
      throw error(401, '付款連結無效');
    }
    if (msg.includes('404')) {
      throw error(404, '找不到此訂單');
    }
    throw error(500, '無法載入付款資料');
  }
};
