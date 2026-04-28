import {
  getCategories,
  getProductByID,
  getProductImages,
  getProductVariants,
  getProducts,
  validateCoupon
} from '$lib/api';
import { cartStore } from '$lib/stores/cart.svelte';

type ToolAnnotations = {
  readOnlyHint?: boolean;
  destructiveHint?: boolean;
  idempotentHint?: boolean;
  untrustedContentHint?: boolean;
};

type ToolDefinition = {
  name: string;
  title: string;
  description: string;
  inputSchema: Record<string, unknown>;
  annotations?: ToolAnnotations;
  execute: (input: any) => Promise<unknown>;
};

type ModelContext = {
  registerTool: (tool: ToolDefinition) => Promise<unknown> | unknown;
};

function getModelContext(): ModelContext | null {
  if (typeof navigator === 'undefined') return null;
  const ctx = (navigator as unknown as { modelContext?: ModelContext }).modelContext;
  return ctx && typeof ctx.registerTool === 'function' ? ctx : null;
}

const tools: ToolDefinition[] = [
  {
    name: 'browse_products',
    title: 'Browse products',
    description: 'List products in the catalog with pagination.',
    inputSchema: {
      type: 'object',
      properties: {
        limit: { type: 'integer', minimum: 1, maximum: 100, default: 20 },
        offset: { type: 'integer', minimum: 0, default: 0 }
      }
    },
    annotations: { readOnlyHint: true },
    execute: ({ limit, offset } = {}) => getProducts(limit ?? 20, offset ?? 0)
  },
  {
    name: 'get_product_detail',
    title: 'Get product detail',
    description: 'Fetch a product with its variants and images by product ID.',
    inputSchema: {
      type: 'object',
      properties: { productID: { type: 'string' } },
      required: ['productID']
    },
    annotations: { readOnlyHint: true },
    execute: async ({ productID }) => {
      const [product, variants, images] = await Promise.all([
        getProductByID(productID),
        getProductVariants(productID),
        getProductImages(productID)
      ]);
      return { product, variants, images };
    }
  },
  {
    name: 'list_categories',
    title: 'List categories',
    description: 'Return all storefront categories.',
    inputSchema: { type: 'object', properties: {} },
    annotations: { readOnlyHint: true },
    execute: () => getCategories()
  },
  {
    name: 'view_cart',
    title: 'View cart',
    description: 'Read the current shopping cart for this browser session.',
    inputSchema: { type: 'object', properties: {} },
    annotations: { readOnlyHint: true },
    execute: () => Promise.resolve(cartStore.cart)
  },
  {
    name: 'add_to_cart',
    title: 'Add item to cart',
    description: 'Add a product variant to the current shopping cart.',
    inputSchema: {
      type: 'object',
      properties: {
        variantID: { type: 'string' },
        quantity: { type: 'integer', minimum: 1, default: 1 }
      },
      required: ['variantID']
    },
    execute: async ({ variantID, quantity }) => {
      await cartStore.add(variantID, quantity ?? 1);
      return cartStore.cart;
    }
  },
  {
    name: 'update_cart_item',
    title: 'Update cart item quantity',
    description: 'Change the quantity of an item already in the cart.',
    inputSchema: {
      type: 'object',
      properties: {
        itemID: { type: 'string' },
        quantity: { type: 'integer', minimum: 0 }
      },
      required: ['itemID', 'quantity']
    },
    execute: async ({ itemID, quantity }) => {
      await cartStore.update(itemID, quantity);
      return cartStore.cart;
    }
  },
  {
    name: 'remove_from_cart',
    title: 'Remove item from cart',
    description: 'Remove an item from the current shopping cart.',
    inputSchema: {
      type: 'object',
      properties: { itemID: { type: 'string' } },
      required: ['itemID']
    },
    execute: async ({ itemID }) => {
      await cartStore.remove(itemID);
      return cartStore.cart;
    }
  },
  {
    name: 'validate_coupon',
    title: 'Validate coupon',
    description: 'Check whether a coupon code is valid for a given cart subtotal.',
    inputSchema: {
      type: 'object',
      properties: {
        code: { type: 'string' },
        subtotal: { type: 'number', minimum: 0 }
      },
      required: ['code', 'subtotal']
    },
    annotations: { readOnlyHint: true },
    execute: ({ code, subtotal }) => validateCoupon(code, subtotal)
  }
];

let registered = false;

export async function registerStorefrontTools(enabled: boolean): Promise<void> {
  if (!enabled || registered) return;
  const ctx = getModelContext();
  if (!ctx) return;
  for (const tool of tools) {
    try {
      await ctx.registerTool(tool);
    } catch (err) {
      console.warn(`[webmcp] failed to register tool "${tool.name}":`, err);
    }
  }
  registered = true;
}
