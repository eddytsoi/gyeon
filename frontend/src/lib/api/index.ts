import type { Address, Cart, CartItem, Category, CmsPage, CmsPost, Customer, NavMenu, Order, Product, ProductImage, Variant } from '$lib/types';

const base = () =>
  typeof window === 'undefined' ? 'http://localhost:8080/api/v1' : '/api/v1';

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

export const checkout = (cartID: string, shippingAddressID?: string, notes?: string) =>
  request<Order>('/orders/checkout', {
    method: 'POST',
    body: JSON.stringify({ cart_id: cartID, shipping_address_id: shippingAddressID, notes })
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
