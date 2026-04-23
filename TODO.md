# Gyeon — TODO

## ✅ Completed

### Backend
- Auth (JWT, middleware, login handler) + multi-user admin auth with roles
- CMS: pages, posts, post categories, navigation
- eShop: products, product categories, variants (full CRUD + stock adjust), images (full CRUD)
- Orders: cart, checkout, order management, fulfillment, cancellation
- Customers: registration, login, profile, addresses, purchase history
- Admin: stats, user management (roles: super_admin/admin/editor)
- Site settings API (key-value store)
- Media library: file upload + management
- DB migrations: eshop, orders, cms, navigation, customer auth, settings/media, admin users
- SQL queries: products, categories, variants, product images, cms pages/posts

### Frontend (SvelteKit)
- Admin: dashboard, login/logout (email+password), products list+create/edit, orders list/detail
- Admin: product variants management (add/edit/delete/stock adjust), product images management
- Admin: customer management (list + detail with order history)
- Admin: site settings page, admin users (roles & permissions)
- Admin CMS: pages, posts, post categories, navigation
- Storefront: home, blog, blog post, products, product detail, cart, static pages
- Docker Compose setup

---

## 🔲 Backend

### eShop
- [ ] Pricing rules (discount campaigns, coupon codes) — `compare_at_price` already supported on variants

### Localization
- [ ] i18n support for content (CMS pages, products)

---

## 🔲 Frontend

### Admin
- [x] Product create/edit page
- [x] Product variants management UI (add/edit/delete/stock adjust)
- [x] Product images management UI (add/set primary/delete)
- [x] Customer management pages (list + detail with order history)
- [x] Admin settings page
- [x] User roles & permissions UI (create/edit/delete admin users with roles)

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
