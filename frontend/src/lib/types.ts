export interface Category {
  id: string;
  parent_id?: string;
  slug: string;
  name: string;
  description?: string;
  media_file_id?: string;
  image_url?: string;
  sort_order: number;
  is_active: boolean;
}

export interface Product {
  id: string;
  number: number;
  category_id?: string;
  slug: string;
  name: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Variant {
  id: string;
  product_id: string;
  sku: string;
  price: number;
  compare_at_price?: number;
  stock_qty: number;
  is_active: boolean;
  product_name?: string;
  image_url?: string;
}

export interface ProductImage {
  id: string;
  product_id: string;
  variant_id?: string;
  media_file_id?: string;
  url: string;
  alt_text?: string;
  sort_order: number;
  is_primary: boolean;
}

export interface CartItem {
  id: string;
  cart_id: string;
  variant_id: string;
  quantity: number;
  added_at: string;
  product_name: string;
  sku: string;
  price: number;
  image_url?: string;
}

export interface Cart {
  id: string;
  customer_id?: string;
  session_token?: string;
  items: CartItem[];
}

export interface OrderItem {
  id: string;
  order_id: string;
  variant_id?: string;
  product_name: string;
  variant_sku: string;
  variant_attrs?: Record<string, string>;
  unit_price: number;
  quantity: number;
  line_total: number;
}

export interface ShippingAddress {
  first_name: string;
  last_name: string;
  phone?: string;
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  country: string;
}

export interface Order {
  id: string;
  number: number;
  customer_id?: string;
  status: string;
  shipping_address_id?: string;
  shipping_address?: ShippingAddress;
  subtotal: number;
  shipping_fee: number;
  discount_amount: number;
  total: number;
  notes?: string;
  customer_email?: string;
  customer_phone?: string;
  customer_name?: string;
  payment_intent_id?: string;
  payment_status?: string;
  payment_method?: string;
  paid_at?: string;
  items: OrderItem[];
  created_at: string;
}

export interface PaymentConfig {
  publishable_key: string;
  mode: 'test' | 'live';
}

export interface CheckoutResult {
  order: Order;
  client_secret: string;
  publishable_key: string;
  mode: 'test' | 'live';
  setup_client_secret?: string;
}

export interface SavedPaymentMethod {
  id: string;
  customer_id: string;
  stripe_pm_id: string;
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  is_default: boolean;
  created_at: string;
}

export interface CustomerInfoInput {
  first_name: string;
  last_name?: string;
  email: string;
  phone: string;
}

export interface ShippingAddressInput {
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  country: string;
}

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

export interface Customer {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Address {
  id: string;
  customer_id?: string;
  first_name: string;
  last_name: string;
  phone?: string;
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  country: string;
  is_default: boolean;
  created_at: string;
}

export interface CmsPost {
  id: string;
  category_id?: string;
  slug: string;
  title: string;
  excerpt?: string;
  content: string;
  cover_media_file_id?: string;
  cover_image_url?: string;
  is_published: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}
