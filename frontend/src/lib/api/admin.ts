import type { Category, Order, Product, Variant } from '$lib/types';

const base = () =>
  typeof window === 'undefined' ? 'http://localhost:8080/api/v1' : '/api/v1';

async function request<T>(path: string, token: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base()}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...init?.headers
    },
    ...init
  });
  if (!res.ok) throw new Error(`API ${res.status}: ${path}`);
  return res.json() as Promise<T>;
}

export interface AdminStats {
  total_products: number;
  total_orders: number;
  total_revenue: number;
  pending_orders: number;
}

export const adminLogin = async (password: string): Promise<string> => {
  const res = await fetch(`${base()}/admin/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password })
  });
  if (!res.ok) throw new Error('Invalid password');
  const data = await res.json();
  return data.token;
};

export const getStats = (token: string) =>
  request<AdminStats>('/admin/stats', token);

// Products (admin uses same endpoints, protected by token for mutations)
export const adminGetProducts = (token: string, limit = 50, offset = 0) =>
  request<Product[]>(`/products?limit=${limit}&offset=${offset}`, token);

export const adminCreateProduct = (token: string, body: Partial<Product>) =>
  request<Product>('/products', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateProduct = (token: string, id: string, body: Partial<Product> & { is_active: boolean }) =>
  request<Product>(`/products/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteProduct = (token: string, id: string) =>
  request(`/products/${id}`, token, { method: 'DELETE' });

export const adminGetVariants = (token: string, productID: string) =>
  request<Variant[]>(`/products/${productID}/variants`, token);

export const adminCreateVariant = (token: string, productID: string, body: Partial<Variant>) =>
  request<Variant>(`/products/${productID}/variants`, token, { method: 'POST', body: JSON.stringify(body) });

// Categories
export const adminGetCategories = (token: string) =>
  request<Category[]>('/categories', token);

export const adminCreateCategory = (token: string, body: Partial<Category>) =>
  request<Category>('/categories', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateCategory = (token: string, id: string, body: Partial<Category> & { is_active: boolean }) =>
  request<Category>(`/categories/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

// Orders
export const adminGetOrders = (token: string, limit = 50, offset = 0) =>
  request<Order[]>(`/orders?limit=${limit}&offset=${offset}`, token);

export const adminGetOrder = (token: string, id: string) =>
  request<Order>(`/orders/${id}`, token);

export const adminUpdateOrderStatus = (token: string, id: string, status: string, note?: string) =>
  request<Order>(`/orders/${id}/status`, token, {
    method: 'POST',
    body: JSON.stringify({ status, note })
  });

// ── CMS ──────────────────────────────────────────────────────────────────────

export interface CmsPage {
  id: string;
  slug: string;
  title: string;
  content: string;
  meta_title?: string;
  meta_desc?: string;
  is_published: boolean;
  created_at: string;
  updated_at: string;
}

export interface CmsPost {
  id: string;
  category_id?: string;
  slug: string;
  title: string;
  excerpt?: string;
  content: string;
  cover_image_url?: string;
  is_published: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

// Pages
export const adminGetPages = (token: string) =>
  request<CmsPage[]>('/admin/cms/pages', token);

export const adminGetPage = (token: string, id: string) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token);

export const adminCreatePage = (token: string, body: Partial<CmsPage>) =>
  request<CmsPage>('/admin/cms/pages', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePage = (token: string, id: string, body: Partial<CmsPage> & { is_published: boolean }) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePage = (token: string, id: string) =>
  request(`/admin/cms/pages/${id}`, token, { method: 'DELETE' });

// Posts
export const adminGetPosts = (token: string, limit = 50, offset = 0) =>
  request<CmsPost[]>(`/admin/cms/posts?limit=${limit}&offset=${offset}`, token);

export const adminGetPost = (token: string, id: string) =>
  request<CmsPost>(`/admin/cms/posts/${id}`, token);

export const adminCreatePost = (token: string, body: Partial<CmsPost>) =>
  request<CmsPost>('/admin/cms/posts', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePost = (token: string, id: string, body: Partial<CmsPost> & { is_published: boolean }) =>
  request<CmsPost>(`/admin/cms/posts/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePost = (token: string, id: string) =>
  request(`/admin/cms/posts/${id}`, token, { method: 'DELETE' });

// Post Categories
export interface PostCategory {
  id: string;
  slug: string;
  name: string;
  sort_order: number;
}

export const adminGetPostCategories = (token: string) =>
  request<PostCategory[]>('/admin/cms/post-categories', token);

export const adminCreatePostCategory = (token: string, body: Omit<PostCategory, 'id'>) =>
  request<PostCategory>('/admin/cms/post-categories', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePostCategory = (token: string, id: string, body: Omit<PostCategory, 'id'>) =>
  request<PostCategory>(`/admin/cms/post-categories/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePostCategory = (token: string, id: string) =>
  request(`/admin/cms/post-categories/${id}`, token, { method: 'DELETE' });

// Navigation
export interface NavItem {
  id: string;
  menu_id: string;
  parent_id?: string;
  label: string;
  url: string;
  target: string;
  sort_order: number;
  children: NavItem[];
}

export interface NavMenu {
  id: string;
  handle: string;
  name: string;
  items: NavItem[];
  created_at: string;
  updated_at: string;
}

export const adminGetNavMenus = (token: string) =>
  request<NavMenu[]>('/admin/cms/nav', token);

export const adminGetNavMenu = (token: string, id: string) =>
  request<NavMenu>(`/admin/cms/nav/${id}`, token);

export const adminAddNavItem = (token: string, menuID: string, body: Partial<NavItem>) =>
  request<NavItem>(`/admin/cms/nav/${menuID}/items`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateNavItem = (token: string, menuID: string, itemID: string, body: Partial<NavItem>) =>
  request<NavItem>(`/admin/cms/nav/${menuID}/items/${itemID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteNavItem = (token: string, menuID: string, itemID: string) =>
  request(`/admin/cms/nav/${menuID}/items/${itemID}`, token, { method: 'DELETE' });

export const adminReplaceNavItems = (token: string, menuID: string, items: Partial<NavItem>[]) =>
  request<NavItem[]>(`/admin/cms/nav/${menuID}/items`, token, { method: 'PUT', body: JSON.stringify(items) });
