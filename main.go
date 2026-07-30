package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/handlers"
	"github.com/dreamoutbox/go-pos/middleware"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

//go:embed templates static demo
var embeddedFS embed.FS

type CustomRender struct {
	templates map[string]*template.Template
}

func (r CustomRender) Instance(name string, data interface{}) render.Render {
	standalones := map[string]bool{
		"auth/login.html":        true,
		"receipt/receipt.html":   true,
		"report/report.html":     true,
		"credit_note/print.html": true,
		"debit_note/print.html":  true,
		"error/403.html":         true,
		"error/404.html":         true,
		"error/500.html":         true,
	}

	tmplName := "base"
	if standalones[name] {
		tmplName = name
	}

	return render.HTML{
		Template: r.templates[name],
		Name:     tmplName,
		Data:     data,
	}
}

func loadTemplates() CustomRender {
	r := CustomRender{templates: make(map[string]*template.Template)}

	funcMap := template.FuncMap{
		"formatMoney": func(val float64) string {
			return fmt.Sprintf("%.2f", val)
		},
		"formatDate": func(val interface{}) string {
			if val == nil {
				return "-"
			}
			switch t := val.(type) {
			case time.Time:
				return t.Format("02 Jan 2006 15:04")
			case *time.Time:
				if t == nil {
					return "-"
				}
				return t.Format("02 Jan 2006 15:04")
			default:
				return "-"
			}
		},
		"slice": func(s string, start, end int) string {
			if len(s) < end {
				return s
			}
			return s[start:end]
		},
		"not": func(v interface{}) bool {
			if v == nil {
				return true
			}
			switch b := v.(type) {
			case bool:
				return !b
			}
			return false
		},
	}

	standalones := map[string]bool{
		"auth/login.html":        true,
		"receipt/receipt.html":   true,
		"report/report.html":     true,
		"credit_note/print.html": true,
		"debit_note/print.html":  true,
		"error/403.html":         true,
		"error/404.html":         true,
		"error/500.html":         true,
	}

	err := fs.WalkDir(embeddedFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".html" {
			relPath, err := filepath.Rel("templates", path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			if strings.HasPrefix(relPath, "layout/") {
				return nil
			}

			t := template.New(relPath).Funcs(funcMap)

			if standalones[relPath] {
				content, err := fs.ReadFile(embeddedFS, path)
				if err != nil {
					return err
				}
				_, err = t.Parse(string(content))
				if err != nil {
					panic(err)
				}
			} else {
				baseContent, err := fs.ReadFile(embeddedFS, "templates/layout/base.html")
				if err != nil {
					panic(err)
				}
				_, err = t.New("base").Parse(string(baseContent))
				if err != nil {
					panic(err)
				}

				pageContent, err := fs.ReadFile(embeddedFS, path)
				if err != nil {
					panic(err)
				}
				_, err = t.New("content").Parse(string(pageContent))
				if err != nil {
					panic(err)
				}
			}

			r.templates[relPath] = t
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	return r
}

func main() {
	config.LoadConfig()
	config.InitDB()
	utils.InitValidator()

	// Migrate schemas
	config.DB.AutoMigrate(
		&models.Category{},
		&models.Shop{},
		&models.User{},
		&models.Product{},
		&models.StockHistory{},
		&models.Order{},
		&models.OrderItem{},
		&models.DocumentSequence{},
		&models.Refund{},
		&models.RefundItem{},
		&models.CreditNote{},
		&models.CreditNoteItem{},
		&models.DebitNote{},
		&models.DebitNoteItem{},
		&models.Session{},
	)

	// Backfill codes for legacy orders
	backfillOrderCodes()

	// Seed default data if database is empty
	seedDefaultData()
	seedCategories()
	seedMockData()

	r := gin.Default()

	// Load HTML custom templates from embedded FS
	r.HTMLRender = loadTemplates()

	// Static Assets from embedded FS
	staticFS, err := fs.Sub(embeddedFS, "static")
	if err == nil {
		r.StaticFS("/static", http.FS(staticFS))
	} else {
		r.Static("/static", "./static")
	}
	// Make sure upload directory exists and serve
	_ = os.MkdirAll(config.AppConfig.UploadDir, 0755)
	r.Static("/uploads", config.AppConfig.UploadDir)

	// Public Routes
	r.GET("/login", handlers.ShowLoginPage)
	r.POST("/login", handlers.Login)

	// Authenticated Routes
	auth := r.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		auth.POST("/logout", handlers.Logout)
		auth.GET("/", handlers.Dashboard)

		// POS Sales Orders
		auth.GET("/orders/new", handlers.NewOrderPage)
		auth.POST("/orders", handlers.CreateOrder)
		auth.GET("/orders", handlers.ListOrders)
		auth.GET("/orders/:id", handlers.ShowOrder)
		auth.PATCH("/orders/:id/pay", handlers.PayOrder)
		auth.GET("/orders/:id/receipt", handlers.RenderReceipt)

		// Product Catalog list & details
		auth.GET("/products", handlers.ListProducts)
		auth.GET("/products/:id", handlers.ShowProduct)

		// Stock Inventory view
		auth.GET("/stock", handlers.StockList)

		// Report & Analysis
		auth.GET("/reports", handlers.ReportDashboard)
		auth.GET("/reports/data", handlers.ReportDataJSON)
		auth.GET("/reports/print", handlers.PrintableReport)
		auth.GET("/reports/export", handlers.ExportExcelReport)

		// Refunds
		auth.GET("/refunds", handlers.ListRefunds)
		auth.GET("/orders/:id/refund", handlers.NewRefundForm)
		auth.POST("/orders/:id/refund", handlers.CreateRefund)
		auth.GET("/refunds/:id", handlers.ShowRefund)

		// Credit Notes
		auth.GET("/credit_notes", handlers.ListCreditNotes)
		auth.GET("/orders/:id/credit_note", handlers.NewCreditNoteForm)
		auth.POST("/orders/:id/credit_note", handlers.CreateCreditNote)
		auth.GET("/credit_notes/:id", handlers.ShowCreditNote)
		auth.GET("/credit_notes/:id/print", handlers.PrintCreditNote)

		// Debit Notes
		auth.GET("/debit_notes", handlers.ListDebitNotes)
		auth.GET("/orders/:id/debit_note", handlers.NewDebitNoteForm)
		auth.POST("/orders/:id/debit_note", handlers.CreateDebitNote)
		auth.GET("/debit_notes/:id", handlers.ShowDebitNote)
		auth.GET("/debit_notes/:id/print", handlers.PrintDebitNote)

		// Admin Panel Routes — accessible by shop_owner and superuser
		shopAdmin := auth.Group("/")
		shopAdmin.Use(middleware.ShopOwnerRequired())
		{
			// User management
			shopAdmin.GET("/users", handlers.ListUsers)
			shopAdmin.GET("/users/new", handlers.NewUserForm)
			shopAdmin.POST("/users", handlers.CreateUser)
			shopAdmin.GET("/users/:id/edit", handlers.EditUserForm)
			shopAdmin.PATCH("/users/:id", handlers.UpdateUser)
			shopAdmin.PATCH("/users/:id/password", handlers.ChangePassword)

			// Product creation & editing
			shopAdmin.GET("/products/new", handlers.NewProductForm)
			shopAdmin.POST("/products", handlers.CreateProduct)
			shopAdmin.GET("/products/:id/edit", handlers.EditProductForm)
			shopAdmin.PATCH("/products/:id", handlers.UpdateProduct)
			shopAdmin.DELETE("/products/:id", handlers.DeleteProduct)
			shopAdmin.POST("/products/:id/restore", handlers.RestoreProduct)
			shopAdmin.PATCH("/products/:id/restore", handlers.RestoreProduct)

			// Stock updates & audit history
			shopAdmin.GET("/stock/:productID/add", handlers.AddStockForm)
			shopAdmin.POST("/stock/:productID/add", handlers.AddStock)
			shopAdmin.PATCH("/stock/:productID", handlers.EditStock)
			shopAdmin.GET("/stock/history", handlers.StockHistory)

			// Tax settings
			shopAdmin.GET("/tax/settings", handlers.TaxSettingsForm)
			shopAdmin.PATCH("/tax/settings", handlers.UpdateTaxSettings)

			// My Shop self-service settings (shop_owner edits their own shop)
			shopAdmin.GET("/shop/settings", handlers.MyShopSettingsForm)
			shopAdmin.PATCH("/shop/settings", handlers.UpdateMyShopSettings)
		}

		// Superuser-only Routes — global shop management
		superAdmin := auth.Group("/")
		superAdmin.Use(middleware.SuperuserRequired())
		{
			superAdmin.GET("/shops", handlers.ListShops)
			superAdmin.GET("/shops/new", handlers.NewShopForm)
			superAdmin.POST("/shops", handlers.CreateShop)
			superAdmin.GET("/shops/:id/edit", handlers.EditShopForm)
			superAdmin.PATCH("/shops/:id", handlers.UpdateShop)
			superAdmin.POST("/shops/:id/switch", handlers.SwitchShop)
		}
	}

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error/404.html", nil)
	})

	fmt.Printf("Starting Go POS server on http://localhost:%s\n", config.AppConfig.Port)
	server := &http.Server{
		Addr:    ":" + config.AppConfig.Port,
		Handler: middleware.MethodOverride(r),
	}
	_ = server.ListenAndServe()
}

func seedDefaultData() {
	var userCount int64
	config.DB.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		// Create default shop
		shop := models.Shop{
			Name:        "My POS Shop",
			TaxEnabled:  false,
			TaxIncluded: true,
			TaxRate:     7.0,
		}
		if err := config.DB.Create(&shop).Error; err != nil {
			panic("failed to seed default shop: " + err.Error())
		}

		// Create default superuser
		admin := models.User{
			ShopID: shop.ID,
			Email:  "admin@pos.local",
			Name:   "Administrator",
			Role:   "superuser",
		}
		if err := admin.SetPassword("admin"); err != nil {
			panic("failed to set admin password: " + err.Error())
		}
		if err := config.DB.Create(&admin).Error; err != nil {
			panic("failed to seed default admin: " + err.Error())
		}

		fmt.Println("==================================================")
		fmt.Println("FIRST RUN DETECTED: Default shop & superuser created.")
		fmt.Println("Email   : admin@pos.local")
		fmt.Println("Password: admin")
		fmt.Println("Role    : superuser")
		fmt.Println("==========================================")
	}
}

func backfillOrderCodes() {
	var orders []models.Order
	if err := config.DB.Where("code IS NULL OR code = ''").Order("id ASC").Find(&orders).Error; err == nil {
		for _, order := range orders {
			tx := config.DB.Begin()
			code, err := utils.GenerateDocumentCode(tx, order.ShopID, "ORD", order.CreatedAt)
			if err == nil {
				tx.Model(&order).Update("code", code)
				tx.Commit()
			} else {
				tx.Rollback()
			}
		}
	}
}

func seedCategories() {
	defaultCategories := []string{
		"Beverages",
		"Snacks",
		"Dairy",
		"Bakery",
		"Personal Care",
		"Household",
		"Alcohol & Tobacco",
		"Health",
		"Stationery",
		"Other",
	}

	for _, name := range defaultCategories {
		var cat models.Category
		if err := config.DB.Where("name = ?", name).First(&cat).Error; err != nil {
			config.DB.Create(&models.Category{Name: name})
		}
	}
}

type MockProductItem struct {
	Name        string   `json:"name"`
	SKU         string   `json:"sku"`
	Category    string   `json:"category"`
	Cost        float64  `json:"cost"`
	Price       float64  `json:"price"`
	VatRate     *float64 `json:"vat_rate"`
	VatExempt   bool     `json:"vat_exempt"`
	Description string   `json:"description"`
	Stock       int      `json:"stock"`
	Image       string   `json:"image"`
}

func seedMockData() {
	mockEnv := os.Getenv("MOCK_DATA")
	if strings.TrimSpace(mockEnv) == "" {
		return
	}

	mockFilePath := "demo/products.mock.json"
	data, err := fs.ReadFile(embeddedFS, mockFilePath)
	if err != nil {
		data, err = os.ReadFile(mockFilePath)
		if err != nil {
			fmt.Printf("Warning: failed to read mock data file %s: %v\n", mockFilePath, err)
			return
		}
	}

	var mockItems []MockProductItem
	if err := json.Unmarshal(data, &mockItems); err != nil {
		fmt.Printf("Warning: failed to parse mock json data: %v\n", err)
		return
	}

	var shop models.Shop
	if err := config.DB.First(&shop).Error; err != nil {
		fmt.Println("Warning: no shop found for mock data seeding")
		return
	}

	var adminUser models.User
	_ = config.DB.First(&adminUser)

	_ = os.MkdirAll(config.AppConfig.UploadDir, 0755)

	seededCount := 0
	for _, item := range mockItems {
		var count int64
		config.DB.Model(&models.Product{}).Where("shop_id = ? AND sku = ?", shop.ID, item.SKU).Count(&count)
		if count > 0 {
			continue
		}

		var catID *uint
		if item.Category != "" {
			var cat models.Category
			if err := config.DB.Where("name = ?", item.Category).First(&cat).Error; err == nil {
				catID = &cat.ID
			}
		}

		var imagePath string
		if item.Image != "" {
			srcImgPath := "demo/products/" + item.Image
			srcData, err := fs.ReadFile(embeddedFS, srcImgPath)
			if err != nil {
				srcData, err = os.ReadFile(filepath.Join("demo", "products", item.Image))
			}
			if err == nil {
				destFilename := fmt.Sprintf("mock_%s_%s", item.SKU, item.Image)
				destImgPath := filepath.Join(config.AppConfig.UploadDir, destFilename)
				if err := os.WriteFile(destImgPath, srcData, 0644); err == nil {
					imagePath = "/uploads/" + destFilename
				}
			}
		}

		vatRate := 7.0
		if item.VatRate != nil {
			vatRate = *item.VatRate
		}

		product := models.Product{
			ShopID:      shop.ID,
			CategoryID:  catID,
			Name:        item.Name,
			SKU:         item.SKU,
			Cost:        item.Cost,
			Price:       item.Price,
			VatRate:     vatRate,
			VatExempt:   item.VatExempt,
			ImagePath:   imagePath,
			Description: item.Description,
			Stock:       item.Stock,
		}

		if err := config.DB.Create(&product).Error; err == nil {
			seededCount++
			if item.Stock > 0 && adminUser.ID > 0 {
				history := models.StockHistory{
					ProductID: product.ID,
					UserID:    adminUser.ID,
					Quantity:  item.Stock,
					Type:      "add",
					Note:      "Mock Data Initial Stock",
				}
				_ = config.DB.Create(&history)
			}
		}
	}

	if seededCount > 0 {
		fmt.Printf("MOCK_DATA ENABLED: Seeded %d mock products into shop '%s'.\n", seededCount, shop.Name)
	}
}
