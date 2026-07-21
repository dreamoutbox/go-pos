package main

import (
	"fmt"
	"html/template"
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

type CustomRender struct {
	templates map[string]*template.Template
}

func (r CustomRender) Instance(name string, data interface{}) render.Render {
	standalones := map[string]bool{
		"auth/login.html":      true,
		"receipt/receipt.html": true,
		"report/report.html":   true,
		"error/403.html":       true,
		"error/404.html":       true,
		"error/500.html":       true,
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
	}

	standalones := map[string]bool{
		"auth/login.html":      true,
		"receipt/receipt.html": true,
		"report/report.html":   true,
		"error/403.html":       true,
		"error/404.html":       true,
		"error/500.html":       true,
	}

	err := filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".html" {
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
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				_, err = t.Parse(string(content))
				if err != nil {
					panic(err)
				}
			} else {
				baseContent, err := os.ReadFile("templates/layout/base.html")
				if err != nil {
					panic(err)
				}
				_, err = t.New("base").Parse(string(baseContent))
				if err != nil {
					panic(err)
				}

				pageContent, err := os.ReadFile(path)
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
		&models.Shop{},
		&models.User{},
		&models.Product{},
		&models.StockHistory{},
		&models.Order{},
		&models.OrderItem{},
	)

	// Seed default data if database is empty
	seedDefaultData()

	r := gin.Default()

	// Load HTML custom templates
	r.HTMLRender = loadTemplates()

	// Static Assets
	r.Static("/static", "./static")
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
