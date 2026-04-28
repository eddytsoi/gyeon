import type { Address, Cart, CartItem, Category, CheckoutResult, CmsPage, CmsPost, Customer, CustomerInfoInput, NavMenu, Order, PaymentConfig, Product, ProductImage, SavedPaymentMethod, ShippingAddressInput, Variant } from '$lib/types';

const base = () =>
  typeof window === 'undefined'
    ? (process.env.API_BASE ?? 'http://localhost:8080/api/v1')
    : '/api/v1';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base()}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init
  });
  if (!res.ok) throw new Error(`API ${res.status}: ${path}`);
  return res.json() as Promise<T>;
}

export const getCategories = () => request<Category[]>('/categories');
export const getCategoryBySlug = (slug: string) => request<Category>(`/categories/${slug}`);

export const getProducts = (limit = 20, offset = 0) =>
  request<Product[]>(`/products?limit=${limit}&offset=${offset}`);
export const getProductByID = (id: string) => request<Product>(`/products/${id}`);
export const getProductVariants = (id: string) => request<Variant[]>(`/products/${id}/variants`);
export const getVariantByID = (id: string) => request<Variant>(`/products/variants/${id}`);
export const getProductImages = (id: string) => request<ProductImage[]>(`/products/${id}/images`);

export const getOrCreateCart = (sessionToken: string, customerID?: string) =>
  request<Cart>('/cart', {
    method: 'POST',
    body: JSON.stringify({ session_token: sessionToken, customer_id: customerID ?? null })
  });
export const getCart = (id: string) => request<Cart>(`/cart/${id}`);
export const addToCart = (cartID: string, variantID: string, quantity: number) =>
  request<CartItem>(`/cart/${cartID}/items`, {
    method: 'POST',
    body: JSON.stringify({ variant_id: variantID, quantity })
  });
export const updateCartItem = (cartID: string, itemID: string, quantity: number) =>
  request(`/cart/${cartID}/items/${itemID}`, {
    method: 'PUT',
    body: JSON.stringify({ quantity })
  });
export const removeCartItem = (cartID: string, itemID: string) =>
  fetch(`${base()}/cart/${cartID}/items/${itemID}`, { method: 'DELETE' });

export const checkout = (
  cartID: string,
  options: {
    customerID?: string;
    customerInfo?: CustomerInfoInput;
    shippingAddressID?: string;
    shippingAddress?: ShippingAddressInput;
    saveAddress?: boolean;
    shippingFee?: number;
    couponCode?: string;
    notes?: string;
    saveCard?: boolean;
    savedPaymentMethodId?: string;
  } = {}
) =>
  request<CheckoutResult>('/orders/checkout', {
    method: 'POST',
    body: JSON.stringify({
      cart_id: cartID,
      customer_id: options.customerID ?? null,
      customer_info: options.customerInfo ?? null,
      shipping_address_id: options.shippingAddressID ?? null,
      shipping_address: options.shippingAddress ?? null,
      save_address: options.saveAddress ?? false,
      shipping_fee: options.shippingFee ?? 0,
      coupon_code: options.couponCode ?? null,
      notes: options.notes ?? null,
      save_card: options.saveCard ?? false,
      saved_payment_method_id: options.savedPaymentMethodId ?? null
    })
  });

export const validateCoupon = (code: string, subtotal: number) =>
  request<{ valid: boolean; discount_type?: string; discount_value?: number; discount_amount?: number; message?: string }>(
    '/pricing/validate-coupon',
    { method: 'POST', body: JSON.stringify({ code, subtotal }) }
  );

export const getPaymentConfig = () => request<PaymentConfig>('/payments/config');

export type PublicSetting = { key: string; value: string; description?: string; updated_at: string };
export const getPublicSettings = () => request<PublicSetting[]>('/settings/');

export const setupPassword = (token: string, password: string) =>
  fetch(`${base()}/customers/setup-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, password })
  });

export const requestPasswordReset = (email: string) =>
  fetch(`${base()}/customers/forgot-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email })
  });

// CMS public API
export const getBlogPosts = (limit = 20, offset = 0) =>
  request<CmsPost[]>(`/cms/posts?limit=${limit}&offset=${offset}`);

export const getBlogPostBySlug = (slug: string) =>
  request<CmsPost>(`/cms/posts/by-slug/${slug}`);

export const getCmsPageBySlug = (slug: string) =>
  request<CmsPage>(`/cms/pages/by-slug/${slug}`);

export const getNavMenu = (handle: string) =>
  request<NavMenu>(`/cms/nav/by-handle/${handle}`);

export const getOrderByID = (id: string) => request<Order>(`/orders/${id}`);

export type OrderPaymentInfo = {
  order: Order;
  client_secret: string;
  publishable_key: string;
  mode: string;
  currency: string;
};

export const getOrderPaymentInfo = (id: string, clientSecret: string) =>
  request<OrderPaymentInfo>(`/orders/${id}/payment-info?cs=${encodeURIComponent(clientSecret)}`);

export type OrderSetupTokenResult = {
  token?: string;
  url?: string;
  already_set: boolean;
};

export const createOrderSetupToken = (orderID: string, paymentIntent: string) =>
  request<OrderSetupTokenResult>(`/orders/${orderID}/setup-token`, {
    method: 'POST',
    body: JSON.stringify({ payment_intent: paymentIntent })
  });

// --- Customer auth & account ---

function authed(token: string): RequestInit {
  return { headers: { Authorization: `Bearer ${token}` } };
}

export const registerCustomer = (
  email: string,
  password: string,
  firstName: string,
  lastName: string,
  phone?: string
) =>
  request<{ customer: Customer; token: string }>('/customers/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, first_name: firstName, last_name: lastName, phone })
  });

export const loginCustomer = (email: string, password: string) =>
  request<{ customer: Customer; token: string }>('/customers/login', {
    method: 'POST',
    body: JSON.stringify({ email, password })
  });

export const getMyProfile = (token: string) =>
  request<Customer>('/customers/me', authed(token));

export const updateMyProfile = (
  token: string,
  data: { first_name: string; last_name: string; phone?: string }
) =>
  request<Customer>('/customers/me', {
    method: 'PUT',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const getMyAddresses = (token: string) =>
  request<Address[]>('/customers/me/addresses', authed(token));

export const createMyAddress = (token: string, data: object) =>
  request<Address>('/customers/me/addresses', {
    method: 'POST',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const updateMyAddress = (token: string, id: string, data: object) =>
  request<Address>(`/customers/me/addresses/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const deleteMyAddress = (token: string, id: string) =>
  fetch(`${base()}/customers/me/addresses/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  });

export const getMyOrders = (token: string, limit = 20, offset = 0) =>
  request<Order[]>(`/customers/me/orders?limit=${limit}&offset=${offset}`, authed(token));

// --- Saved payment methods ---

export const getMySavedCards = (token: string) =>
  request<SavedPaymentMethod[]>('/payments/saved-cards', authed(token));

export const deleteMySavedCard = (token: string, id: string) =>
  fetch(`${base()}/payments/saved-cards/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  });

export const setDefaultCard = (token: string, id: string) =>
  fetch(`${base()}/payments/saved-cards/${id}/default`, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${token}` }
  });
