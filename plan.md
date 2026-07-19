# Go POS — Implementation Plan

## Overview

A multi-shop Point of Sale web application built with **Go (Gin)**, **GORM + SQLite**, **html/template**, and **Bootstrap 5** frontend. The shopkeeper starts the app and uses it entirely through a web browser.

---

## Tech Stack

| Layer      | Technology                              |
|------------|-----------------------------------------|
| Language   | Go 1.26                                 |
| Web        | Gin + `html/template`                   |
| ORM / DB   | GORM + SQLite (`./data/pos.db`)         |
| Frontend   | Bootstrap 5 (CDN), Chart.js (CDN)       |
| Auth       | Session-based (cookie + gorilla/sessions or gin-contrib/sessions) |
| Image      | Uploaded to `./data/product/`           |

---

## Project Structure

```
go-pos/
├── main.go                  # entry point, init DB, register routes, run server
├── go.mod / go.sum
│
├── config/
│   └── config.go            # app config (port, session secret, db path)
│
├── models/                  # GORM models
│   ├── shop.go
│   ├── user.go
│   ├── product.go
│   ├── stock.go
│   ├── order.go
│   ├── order_item.go
│   └── tax.go               # shop tax settings
│
├── handlers/                 # Gin handler funcs grouped by feature
│   ├── auth.go               # login / logout
│   ├── dashboard.go          # home dashboard
│   ├── user.go               # admin user management
│   ├── product.go            # product CRUD + image upload
│   ├── stock.go              # stock add / view
│   ├── order.go              # create order, add items, pay
│   ├── receipt.go            # render receipt HTML
│   ├── tax.go                # tax settings
│   └── report.go             # report pages + data API for charts
│
├── middleware/
│   ├── auth.go               # require-login middleware
│   └── admin.go              # require-admin middleware
│
├── templates/                # Go html/template files
│   ├── layout/
│   │   ├── base.html         # base layout (navbar, sidebar, Bootstrap, JS)
│   │   └── navbar.html       # shared nav partial
│   ├── auth/
│   │   └── login.html
│   ├── dashboard/
│   │   └── index.html        # chart.js dashboard
│   ├── user/
│   │   ├── list.html
│   │   └── edit.html
│   ├── product/
│   │   ├── list.html
│   │   ├── form.html         # create / edit form (shared)
│   │   └── detail.html
│   ├── stock/
│   │   ├── list.html
│   │   └── add.html
│   ├── order/
│   │   ├── new.html          # POS selling screen
│   │   └── list.html
│   ├── receipt/
│   │   └── receipt.html      # standalone receipt (opens in new tab)
│   ├── tax/
│   │   └── settings.html
│   └── report/
│       ├── index.html        # dashboard with charts
│       └── report.html       # printable report (opens in new tab)
│
├── static/                   # served as /static/
│   ├── css/
│   │   └── app.css           # custom overrides
│   └── js/
│       └── app.js            # POS JS logic (add item, calc total, etc.)
│
└── data/                     # runtime data (gitignored)
    ├── pos.db                # SQLite database
    └── product/              # uploaded product images
```

---

## 1. Database Models

### 1.1 Shop

```go
type Shop struct {
    gorm.Model
    Name        string `gorm:"not null"`
    Address     string
    Phone       string
    // Tax settings
    TaxEnabled  bool    `gorm:"default:false"`
    TaxName     string  // registered business name
    TaxAddress  string  // registered address
    TaxID       string  // tax ID number
    TaxRate     float64 `gorm:"default:7.0"` // Thailand VAT 7%
}
```

### 1.2 User

```go
type User struct {
    gorm.Model
    ShopID       uint   `gorm:"not null;index"`
    Shop         Shop
    Email        string `gorm:"uniqueIndex;not null"`
    PasswordHash string `gorm:"not null"`
    Name         string `gorm:"not null"`
    Role         string `gorm:"default:'staff'"` // "admin" | "staff"
}
```

### 1.3 Product

```go
type Product struct {
    gorm.Model
    ShopID      uint    `gorm:"not null;index"`
    Shop        Shop
    Name        string  `gorm:"not null"`
    SKU         string  `gorm:"index"`
    Price       float64 `gorm:"not null"`
    Cost        float64 `gorm:"default:0"`     // cost price for profit calc
    ImagePath   string                          // relative path under ./data/product/
    Description string
    Stock       int     `gorm:"default:0"`     // current stock quantity
}
```

> **Design note:** Stock quantity is stored directly on `Product` for fast reads. A separate `StockHistory` table logs every stock change for auditing.

### 1.4 StockHistory

```go
type StockHistory struct {
    gorm.Model
    ProductID uint    `gorm:"not null;index"`
    Product   Product
    UserID    uint    `gorm:"not null"`
    User      User
    Quantity  int     `gorm:"not null"` // positive = add, negative = subtract
    Type      string  `gorm:"not null"` // "add" | "sale" | "adjustment"
    Note      string
}
```

### 1.5 Order

```go
type Order struct {
    gorm.Model
    ShopID     uint    `gorm:"not null;index"`
    Shop       Shop
    UserID     uint    `gorm:"not null"`          // cashier
    User       User
    Status     string  `gorm:"default:'pending'"` // "pending" | "paid"
    Subtotal   float64
    TaxAmount  float64
    Total      float64
    PaidAt     *time.Time
}
```

### 1.6 OrderItem

```go
type OrderItem struct {
    gorm.Model
    OrderID   uint    `gorm:"not null;index"`
    Order     Order
    ProductID uint    `gorm:"not null"`
    Product   Product
    Name      string  `gorm:"not null"` // snapshot product name
    Price     float64 `gorm:"not null"` // snapshot price at time of sale
    Cost      float64                   // snapshot cost at time of sale
    Quantity  int     `gorm:"not null"`
    Subtotal  float64 `gorm:"not null"` // price * quantity
}
```

---

## 2. Features — Routes & Logic

### 2.1 Auth (login / logout)

| Method | Route           | Handler              | Access   |
|--------|-----------------|----------------------|----------|
| GET    | `/login`        | `ShowLoginPage`      | public   |
| POST   | `/login`        | `Login`              | public   |
| GET    | `/logout`       | `Logout`             | logged-in|

- Password hashing: `golang.org/x/crypto/bcrypt`.
- Session: `gin-contrib/sessions` with cookie store.
- After login, redirect to dashboard.
- Session stores: `userID`, `shopID`, `role`.

### 2.2 Dashboard

| Method | Route | Handler     | Access    |
|--------|-------|-------------|-----------|
| GET    | `/`   | `Dashboard` | logged-in |

- Today's sales count + revenue.
- Quick-sell shortcut button.
- Low-stock alerts.

### 2.3 User Management (admin only)

| Method | Route                  | Handler          | Access |
|--------|------------------------|------------------|--------|
| GET    | `/users`               | `ListUsers`      | admin  |
| GET    | `/users/:id/edit`      | `EditUserForm`   | admin  |
| POST   | `/users/:id/edit`      | `UpdateUser`     | admin  |
| POST   | `/users/:id/password`  | `ChangePassword` | admin  |

- Admin can change any user's name, email, role, and password.
- No user creation in v1 — seed admin via CLI flag or first-run setup.
  - On first run, if no users exist, show a **setup page** to create the first admin account and shop.

### 2.4 Product CRUD

| Method | Route                    | Handler           | Access    |
|--------|--------------------------|--------------------|-----------|
| GET    | `/products`              | `ListProducts`     | logged-in |
| GET    | `/products/new`          | `NewProductForm`   | admin     |
| POST   | `/products`              | `CreateProduct`    | admin     |
| GET    | `/products/:id`          | `ShowProduct`      | logged-in |
| GET    | `/products/:id/edit`     | `EditProductForm`  | admin     |
| POST   | `/products/:id/edit`     | `UpdateProduct`    | admin     |
| POST   | `/products/:id/delete`   | `DeleteProduct`    | admin     |

**Image upload:**
- Accept multipart form upload.
- Save to `./data/product/<productID>_<timestamp>.<ext>`.
- Store relative path in `Product.ImagePath`.
- Serve images via `r.Static("/uploads", "./data/product")`.

### 2.5 Stock Management

| Method | Route                      | Handler           | Access    |
|--------|----------------------------|--------------------|-----------|
| GET    | `/stock`                   | `StockList`        | logged-in |
| GET    | `/stock/:productID/add`    | `AddStockForm`     | admin     |
| POST   | `/stock/:productID/add`    | `AddStock`         | admin     |
| GET    | `/stock/history`           | `StockHistory`     | admin     |

**Logic:**
- `AddStock`: increment `Product.Stock` by N, insert `StockHistory` with `Type="add"`.
- On sale: decrement `Product.Stock` by quantity, insert `StockHistory` with `Type="sale"`.

### 2.6 Order / POS

| Method | Route                     | Handler           | Access    |
|--------|---------------------------|--------------------|-----------|
| GET    | `/orders/new`             | `NewOrderPage`     | logged-in |
| POST   | `/orders`                 | `CreateOrder`      | logged-in |
| GET    | `/orders`                 | `ListOrders`       | logged-in |
| GET    | `/orders/:id`             | `ShowOrder`        | logged-in |
| POST   | `/orders/:id/pay`         | `PayOrder`         | logged-in |

**POS screen (`/orders/new`):**
- Product search / barcode input.
- Add items to cart (JavaScript manages the cart in-browser).
- Display running subtotal, tax, total.
- Submit → POST `/orders` with JSON items array.

**CreateOrder logic:**
1. Validate all items have sufficient stock.
2. Create `Order` (status=pending) + `OrderItem` rows.
3. Deduct `Product.Stock` for each item.
4. Insert `StockHistory` records (type=sale).
5. If shop has `TaxEnabled`, calculate `TaxAmount = Subtotal * TaxRate / 100`.
6. `Total = Subtotal + TaxAmount`.

**PayOrder:** Set `Status = "paid"`, set `PaidAt`.

### 2.7 Receipt

| Method | Route                    | Handler          | Access    |
|--------|--------------------------|-------------------|-----------|
| GET    | `/orders/:id/receipt`    | `RenderReceipt`   | logged-in |

- Renders a standalone HTML page (no base layout — just receipt content).
- Includes: shop name/address, items, subtotal, tax, total, date, cashier.
- If tax enabled: shows tax name, address, tax ID, tax breakdown.
- Frontend: button on order detail page opens `/orders/:id/receipt` in a **new tab**.
- The receipt template includes a `window.print()` trigger or print button.

### 2.8 Tax Settings (admin)

| Method | Route            | Handler           | Access |
|--------|------------------|--------------------|--------|
| GET    | `/tax/settings`  | `TaxSettingsForm`  | admin  |
| POST   | `/tax/settings`  | `UpdateTaxSettings`| admin  |

- Toggle `TaxEnabled` on/off.
- Edit `TaxName`, `TaxAddress`, `TaxID`.
- Rate fixed at 7% (editable field with default).

### 2.9 Reports

| Method | Route                        | Handler            | Access    |
|--------|------------------------------|--------------------|-----------|
| GET    | `/reports`                   | `ReportDashboard`  | logged-in |
| GET    | `/reports/data`              | `ReportDataJSON`   | logged-in |
| GET    | `/reports/print`             | `PrintableReport`  | logged-in |

**Report Dashboard (`/reports`):**
- Uses **Chart.js** (CDN) to render charts.
- JavaScript fetches `/reports/data?range=today|week|month` and draws:
  - Sales revenue bar chart (daily).
  - Profit/loss line chart.
  - Top products pie chart.
- Summary cards: total revenue, total profit, total orders, total items sold.
- Low-stock product table (stock < configurable threshold, default 10).

**ReportDataJSON:** Returns JSON for Chart.js:
```json
{
  "labels": ["Mon", "Tue", ...],
  "revenue": [1200, 3400, ...],
  "profit": [300, 800, ...],
  "top_products": [{"name": "...", "qty": 50}, ...],
  "low_stock": [{"name": "...", "stock": 3}, ...],
  "summary": { "revenue": 15000, "profit": 4500, "orders": 42, "items": 128 }
}
```

**PrintableReport (`/reports/print?range=...`):**
- Standalone HTML page (opens in new tab).
- Shows the same data in a print-friendly table layout.
- Includes `window.print()` button.

---

## 3. Middleware

### 3.1 AuthRequired
- Check session for `userID`. If missing → redirect `/login`.
- Load user from DB, inject into `gin.Context` (`c.Set("user", user)`).

### 3.2 AdminRequired
- After AuthRequired, check `user.Role == "admin"`. If not → 403 page.

### 3.3 ShopContext
- Inject `shop` into context from session `shopID` for all handlers.

---

## 4. Template Rendering

- Use `template.ParseGlob` or manual loading to parse all templates.
- Base layout (`base.html`) uses `{{template "content" .}}` pattern.
- Each page defines `{{define "content"}} ... {{end}}`.
- Template functions: `formatMoney`, `formatDate`, `add`, `mul`.

---

## 5. First-Run Setup

On startup:
1. Auto-migrate all models.
2. If `users` table is empty → redirect all routes to `/setup`.
3. `/setup` page: create first shop + admin account.

---

## 6. Dependencies to Add

```
go get github.com/gin-gonic/gin
go get github.com/gin-contrib/sessions
go get github.com/gin-contrib/sessions/cookie
go get golang.org/x/crypto/bcrypt
```

Current deps (keep): `gorm.io/gorm`, `gorm.io/driver/sqlite`.

---

## 7. Implementation Order

| Phase | Task                                    | Files                                    |
|-------|-----------------------------------------|------------------------------------------|
| 1     | Project structure + config + models     | `config/`, `models/`, `main.go`          |
| 2     | Template system + base layout           | `templates/layout/`, static assets       |
| 3     | Auth (login/logout/session/middleware)   | `handlers/auth.go`, `middleware/`         |
| 4     | First-run setup page                    | `handlers/setup.go`, `templates/setup/`  |
| 5     | Dashboard (basic)                       | `handlers/dashboard.go`                  |
| 6     | Product CRUD + image upload             | `handlers/product.go`                    |
| 7     | Stock management                        | `handlers/stock.go`                      |
| 8     | Order / POS screen                      | `handlers/order.go`, `static/js/app.js`  |
| 9     | Receipt template                        | `handlers/receipt.go`                    |
| 10    | Tax settings                            | `handlers/tax.go`                        |
| 11    | User management (admin)                 | `handlers/user.go`                       |
| 12    | Reports + Chart.js dashboard            | `handlers/report.go`                     |

---

## 8. Key Design Decisions

1. **Multi-shop isolation**: All queries filter by `shop_id` from session. Users belong to one shop.
2. **Stock on Product**: Denormalized `Stock` field on `Product` for fast POS reads; `StockHistory` for audit trail.
3. **Price snapshots in OrderItem**: Order items store the price/cost at time of sale, so product price changes don't affect past orders.
4. **Receipt & Report as standalone HTML**: No layout wrapper — just clean HTML that opens in a new tab for printing.
5. **No user registration**: Admin creates users. First-run creates admin.
6. **Thailand VAT**: Default 7%, configurable per shop. Applied at order level.
