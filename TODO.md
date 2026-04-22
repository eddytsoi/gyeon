# Gyeon — TODO

## ✅ Completed

### Backend
- Auth (JWT, middleware, login handler)
- CMS: pages, posts, post categories, navigation
- eShop: products, product categories
- Orders: cart, order management
- Admin stats handler
- DB migrations: eshop, orders, cms, navigation
- SQL queries: products, categories, variants, product images, cms pages/posts

### Frontend (SvelteKit)
- Admin: dashboard, login/logout, products list, orders list/detail
- Admin CMS: pages, posts, post categories, navigation
- Storefront: home, blog, blog post, products, product detail, cart, static pages
- Docker Compose setup

---

## 🔲 Backend

### eShop
- [ ] Product variants handler (`/backend/internal/shop/`) — queries already exist
- [ ] Product images handler — queries already exist
- [ ] Inventory management (stock tracking, low stock alerts)
- [ ] Pricing rules (discounts, sale price)

### Orders
- [ ] Checkout handler (place order from cart)
- [ ] Order fulfillment (status transitions: pending → processing → shipped → delivered)
- [ ] Order cancellation

### Customers
- [ ] Customer registration & login
- [ ] Customer profile (addresses, account info)
- [ ] Purchase history API

### Admin
- [ ] User roles & permissions system
- [ ] Site settings API

### Media
- [ ] File/image upload endpoint
- [ ] Media library management

### Localization
- [ ] i18n support for content (CMS pages, products)

---

## 🔲 Frontend

### Admin
- [ ] Product create/edit page (only list page exists)
- [ ] Product variants management UI
- [ ] Product images management UI
- [ ] Customer management pages
- [ ] Admin settings page
- [ ] User roles & permissions UI

### Storefront
- [ ] Checkout page & flow
- [ ] Customer account pages (register, login, profile, order history)
- [ ] Localization / language switcher

---

## 🔲 Infrastructure & Docs

- [ ] Document environment variables in `CLAUDE.md`
- [ ] Getting Started / Running Locally guide
- [ ] Testing setup (backend: Go tests, frontend: Playwright or Vitest)
- [ ] CI/CD pipeline
- [ ] Architecture documentation (define module boundaries)
