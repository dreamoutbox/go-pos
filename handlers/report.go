package handlers

import (
	"net/http"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
)

type TopProduct struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Revenue  float64 `json:"revenue"`
}

type DailyStat struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Profit  float64 `json:"profit"`
}

type SummaryStats struct {
	Revenue   float64 `json:"revenue"`
	Profit    float64 `json:"profit"`
	Orders    int64   `json:"orders"`
	ItemsSold int64   `json:"items_sold"`
}

func ReportDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "report/index.html", gin.H{
		"user": c.MustGet("user"),
		"shop": c.MustGet("shop"),
	})
}

func ReportDataJSON(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	dateRange := c.DefaultQuery("range", "today")

	var startDate time.Time
	now := time.Now()

	switch dateRange {
	case "today":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, 0, -30)
	default:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// 1. Fetch Orders for calculations
	var orders []models.Order
	err := config.DB.Where("shop_id = ? AND status = ? AND created_at >= ?", shopID, "paid", startDate).
		Preload("OrderItems").
		Find(&orders).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. Aggregate Summaries
	var summary SummaryStats
	summary.Orders = int64(len(orders))

	dailyMap := make(map[string]*DailyStat)

	for _, order := range orders {
		summary.Revenue += order.Total

		// Calculate profit at order level
		var orderProfit float64
		for _, item := range order.OrderItems {
			summary.ItemsSold += int64(item.Quantity)
			// Profit = item revenue - item cost
			itemProfit := item.Subtotal - (item.Cost * float64(item.Quantity))
			orderProfit += itemProfit
		}
		summary.Profit += orderProfit

		// Daily aggregation
		dateStr := order.CreatedAt.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyStat{Date: dateStr}
		}
		dailyMap[dateStr].Revenue += order.Total
		dailyMap[dateStr].Profit += orderProfit
	}

	// Format daily list
	var dailyStats []DailyStat
	tempDate := startDate
	for !tempDate.After(now) {
		dateStr := tempDate.Format("2006-01-02")
		stat := DailyStat{Date: dateStr}
		if val, exists := dailyMap[dateStr]; exists {
			stat.Revenue = val.Revenue
			stat.Profit = val.Profit
		}
		dailyStats = append(dailyStats, stat)
		tempDate = tempDate.AddDate(0, 0, 1)
	}

	// 3. Top Products
	var topProducts []TopProduct
	config.DB.Table("order_items").
		Select("order_items.name as name, sum(order_items.quantity) as quantity, sum(order_items.subtotal) as revenue").
		Joins("JOIN orders ON order_items.order_id = orders.id").
		Where("orders.shop_id = ? AND orders.status = ? AND orders.created_at >= ?", shopID, "paid", startDate).
		Group("order_items.name").
		Order("quantity DESC").
		Limit(5).
		Scan(&topProducts)

	// 4. Low stock products
	var lowStock []models.Product
	config.DB.Where("shop_id = ? AND stock < ?", shopID, 10).Find(&lowStock)

	c.JSON(http.StatusOK, gin.H{
		"summary":      summary,
		"daily_stats":  dailyStats,
		"top_products": topProducts,
		"low_stock":    lowStock,
	})
}

func PrintableReport(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	dateRange := c.DefaultQuery("range", "today")

	var startDate time.Time
	now := time.Now()

	switch dateRange {
	case "today":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, 0, -30)
	default:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	var orders []models.Order
	err := config.DB.Where("shop_id = ? AND status = ? AND created_at >= ?", shopID, "paid", startDate).
		Preload("OrderItems").
		Find(&orders).Error
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	var summary SummaryStats
	summary.Orders = int64(len(orders))
	for _, order := range orders {
		summary.Revenue += order.Total
		for _, item := range order.OrderItems {
			summary.ItemsSold += int64(item.Quantity)
			summary.Profit += item.Subtotal - (item.Cost * float64(item.Quantity))
		}
	}

	var topProducts []TopProduct
	config.DB.Table("order_items").
		Select("order_items.name as name, sum(order_items.quantity) as quantity, sum(order_items.subtotal) as revenue").
		Joins("JOIN orders ON order_items.order_id = orders.id").
		Where("orders.shop_id = ? AND orders.status = ? AND orders.created_at >= ?", shopID, "paid", startDate).
		Group("order_items.name").
		Order("quantity DESC").
		Limit(10).
		Scan(&topProducts)

	var lowStock []models.Product
	config.DB.Where("shop_id = ? AND stock < ?", shopID, 10).Find(&lowStock)

	c.HTML(http.StatusOK, "report/report.html", gin.H{
		"range":        dateRange,
		"startDate":    startDate.Format("02 Jan 2006"),
		"endDate":      now.Format("02 Jan 2006"),
		"summary":      summary,
		"top_products": topProducts,
		"low_stock":    lowStock,
		"user":         c.MustGet("user"),
		"shop":         c.MustGet("shop"),
	})
}
