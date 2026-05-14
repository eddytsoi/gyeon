import { error } from '@sveltejs/kit';
import { getOrderByPaymentIntent, createOrderSetupToken } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
  const orderID = url.searchParams.get('order');
  const paymentIntent = url.searchParams.get('payment_intent');
  if (!orderID || !paymentIntent) throw error(404, 'Order not found');

  let order;
  try {
    order = await getOrderByPaymentIntent(orderID, paymentIntent);
  } catch {
    throw error(404, 'Order not found');
  }

  let setupURL: string | null = null;
  try {
    const result = await createOrderSetupToken(orderID, paymentIntent);
    if (!result.already_set && result.url) {
      setupURL = result.url;
    }
  } catch {
    // 401 (pi mismatch) or other failure — silently hide the CTA.
  }

  return { order, setupURL };
};
