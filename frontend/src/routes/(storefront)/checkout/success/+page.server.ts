import { error } from '@sveltejs/kit';
import { getOrderByPaymentIntent, getMyOrderByID, createOrderSetupToken } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url, cookies }) => {
  const orderID = url.searchParams.get('order');
  const paymentIntent = url.searchParams.get('payment_intent');
  const method = url.searchParams.get('method');
  const loggedIn = !!cookies.get('customer_token');

  // Bank-transfer (installer) orders have no Stripe PaymentIntent. They are
  // authorized by the customer's own token (installers are always logged in)
  // and show the transfer instructions + on-hold status instead of a paid
  // confirmation.
  if (method === 'bank_transfer') {
    const token = cookies.get('customer_token') ?? null;
    if (!orderID || !token) throw error(404, 'Order not found');
    let order;
    try {
      order = await getMyOrderByID(token, orderID);
    } catch {
      throw error(404, 'Order not found');
    }
    return { order, setupURL: null, bankTransfer: true, loggedIn };
  }

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

  return { order, setupURL, bankTransfer: false, loggedIn };
};
