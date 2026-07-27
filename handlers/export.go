package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func ExportExcelReport(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	shop := c.MustGet("shop").(models.Shop)
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

	// 1. Fetch data
	var orders []models.Order
	config.DB.Where("shop_id = ? AND status IN (?, ?, ?) AND created_at >= ?", shopID, "paid", "partially_refunded", "refunded", startDate).
		Preload("OrderItems").
		Preload("User").
		Find(&orders)

	var refunds []models.Refund
	config.DB.Where("shop_id = ? AND created_at >= ?", shopID, startDate).
		Preload("RefundItems").
		Preload("Order").
		Preload("User").
		Find(&refunds)

	var creditNotes []models.CreditNote
	config.DB.Where("shop_id = ? AND created_at >= ?", shopID, startDate).
		Preload("CreditNoteItems").
		Preload("Order").
		Preload("User").
		Find(&creditNotes)

	var debitNotes []models.DebitNote
	config.DB.Where("shop_id = ? AND created_at >= ?", shopID, startDate).
		Preload("DebitNoteItems").
		Preload("Order").
		Preload("User").
		Find(&debitNotes)

	var products []models.Product
	config.DB.Where("shop_id = ?", shopID).
		Preload("Category").
		Find(&products)

	var stockHistories []models.StockHistory
	config.DB.Joins("JOIN products ON stock_histories.product_id = products.id").
		Where("products.shop_id = ? AND stock_histories.created_at >= ?", shopID, startDate).
		Preload("Product").
		Preload("User").
		Order("stock_histories.created_at DESC").
		Find(&stockHistories)

	// Create excel file
	f := excelize.NewFile()

	// Default sheet is Sheet1, rename it to "Orders"
	f.SetSheetName("Sheet1", "Orders")

	// Create all required sheets (11 sheets in total)
	sheets := []string{
		"Order Items",
		"Refunds",
		"Refund Items",
		"Credit Notes",
		"Credit Note Items",
		"Debit Notes",
		"Debit Note Items",
		"Products",
		"Stock History",
		"VAT Summary",
	}
	for _, sheet := range sheets {
		f.NewSheet(sheet)
	}

	// --- 1. Sheet: Orders ---
	f.SetCellValue("Orders", "A1", "Order Code")
	f.SetCellValue("Orders", "B1", "Date / Time")
	f.SetCellValue("Orders", "C1", "Cashier")
	f.SetCellValue("Orders", "D1", "Status")
	f.SetCellValue("Orders", "E1", "Subtotal (Net)")
	f.SetCellValue("Orders", "F1", "VAT Amount")
	f.SetCellValue("Orders", "G1", "Total (Gross)")

	for i, o := range orders {
		row := i + 2
		code := o.Code
		if code == "" {
			code = fmt.Sprintf("#%d", o.ID)
		}
		f.SetCellValue("Orders", fmt.Sprintf("A%d", row), code)
		f.SetCellValue("Orders", fmt.Sprintf("B%d", row), o.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("Orders", fmt.Sprintf("C%d", row), o.User.Name)
		f.SetCellValue("Orders", fmt.Sprintf("D%d", row), o.Status)
		f.SetCellValue("Orders", fmt.Sprintf("E%d", row), o.Subtotal)
		f.SetCellValue("Orders", fmt.Sprintf("F%d", row), o.TaxAmount)
		f.SetCellValue("Orders", fmt.Sprintf("G%d", row), o.Total)
	}

	// --- 2. Sheet: Order Items ---
	f.SetCellValue("Order Items", "A1", "Order Code")
	f.SetCellValue("Order Items", "B1", "Product Name")
	f.SetCellValue("Order Items", "C1", "Unit Price")
	f.SetCellValue("Order Items", "D1", "Cost")
	f.SetCellValue("Order Items", "E1", "Quantity")
	f.SetCellValue("Order Items", "F1", "Subtotal")

	itemRow := 2
	for _, o := range orders {
		code := o.Code
		if code == "" {
			code = fmt.Sprintf("#%d", o.ID)
		}
		for _, item := range o.OrderItems {
			f.SetCellValue("Order Items", fmt.Sprintf("A%d", itemRow), code)
			f.SetCellValue("Order Items", fmt.Sprintf("B%d", itemRow), item.Name)
			f.SetCellValue("Order Items", fmt.Sprintf("C%d", itemRow), item.Price)
			f.SetCellValue("Order Items", fmt.Sprintf("D%d", itemRow), item.Cost)
			f.SetCellValue("Order Items", fmt.Sprintf("E%d", itemRow), item.Quantity)
			f.SetCellValue("Order Items", fmt.Sprintf("F%d", itemRow), item.Subtotal)
			itemRow++
		}
	}

	// --- 3. Sheet: Refunds ---
	f.SetCellValue("Refunds", "A1", "Refund Code")
	f.SetCellValue("Refunds", "B1", "Order Ref")
	f.SetCellValue("Refunds", "C1", "Date / Time")
	f.SetCellValue("Refunds", "D1", "Staff")
	f.SetCellValue("Refunds", "E1", "Reason")
	f.SetCellValue("Refunds", "F1", "Subtotal")
	f.SetCellValue("Refunds", "G1", "VAT Amount")
	f.SetCellValue("Refunds", "H1", "Total")

	for i, r := range refunds {
		row := i + 2
		orderCode := r.Order.Code
		if orderCode == "" && r.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", r.OrderID)
		}
		f.SetCellValue("Refunds", fmt.Sprintf("A%d", row), r.Code)
		f.SetCellValue("Refunds", fmt.Sprintf("B%d", row), orderCode)
		f.SetCellValue("Refunds", fmt.Sprintf("C%d", row), r.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("Refunds", fmt.Sprintf("D%d", row), r.User.Name)
		f.SetCellValue("Refunds", fmt.Sprintf("E%d", row), r.Reason)
		f.SetCellValue("Refunds", fmt.Sprintf("F%d", row), r.Subtotal)
		f.SetCellValue("Refunds", fmt.Sprintf("G%d", row), r.TaxAmount)
		f.SetCellValue("Refunds", fmt.Sprintf("H%d", row), r.Total)
	}

	// --- 4. Sheet: Refund Items (NEW) ---
	f.SetCellValue("Refund Items", "A1", "Refund Code")
	f.SetCellValue("Refund Items", "B1", "Order Ref")
	f.SetCellValue("Refund Items", "C1", "Product Name")
	f.SetCellValue("Refund Items", "D1", "Unit Price")
	f.SetCellValue("Refund Items", "E1", "Quantity")
	f.SetCellValue("Refund Items", "F1", "Subtotal")
	f.SetCellValue("Refund Items", "G1", "Return To Stock")

	rItemRow := 2
	for _, r := range refunds {
		orderCode := r.Order.Code
		if orderCode == "" && r.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", r.OrderID)
		}
		for _, item := range r.RefundItems {
			f.SetCellValue("Refund Items", fmt.Sprintf("A%d", rItemRow), r.Code)
			f.SetCellValue("Refund Items", fmt.Sprintf("B%d", rItemRow), orderCode)
			f.SetCellValue("Refund Items", fmt.Sprintf("C%d", rItemRow), item.Name)
			f.SetCellValue("Refund Items", fmt.Sprintf("D%d", rItemRow), item.Price)
			f.SetCellValue("Refund Items", fmt.Sprintf("E%d", rItemRow), item.Quantity)
			f.SetCellValue("Refund Items", fmt.Sprintf("F%d", rItemRow), item.Subtotal)
			f.SetCellValue("Refund Items", fmt.Sprintf("G%d", rItemRow), item.ReturnToStock)
			rItemRow++
		}
	}

	// --- 5. Sheet: Credit Notes ---
	f.SetCellValue("Credit Notes", "A1", "Credit Note Code")
	f.SetCellValue("Credit Notes", "B1", "Order Ref")
	f.SetCellValue("Credit Notes", "C1", "Date / Time")
	f.SetCellValue("Credit Notes", "D1", "Staff")
	f.SetCellValue("Credit Notes", "E1", "Reason")
	f.SetCellValue("Credit Notes", "F1", "Subtotal")
	f.SetCellValue("Credit Notes", "G1", "VAT Amount")
	f.SetCellValue("Credit Notes", "H1", "Total")

	for i, cn := range creditNotes {
		row := i + 2
		orderCode := cn.Order.Code
		if orderCode == "" && cn.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", cn.OrderID)
		}
		f.SetCellValue("Credit Notes", fmt.Sprintf("A%d", row), cn.Code)
		f.SetCellValue("Credit Notes", fmt.Sprintf("B%d", row), orderCode)
		f.SetCellValue("Credit Notes", fmt.Sprintf("C%d", row), cn.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("Credit Notes", fmt.Sprintf("D%d", row), cn.User.Name)
		f.SetCellValue("Credit Notes", fmt.Sprintf("E%d", row), cn.Reason)
		f.SetCellValue("Credit Notes", fmt.Sprintf("F%d", row), cn.Subtotal)
		f.SetCellValue("Credit Notes", fmt.Sprintf("G%d", row), cn.TaxAmount)
		f.SetCellValue("Credit Notes", fmt.Sprintf("H%d", row), cn.Total)
	}

	// --- 6. Sheet: Credit Note Items (NEW) ---
	f.SetCellValue("Credit Note Items", "A1", "Credit Note Code")
	f.SetCellValue("Credit Note Items", "B1", "Order Ref")
	f.SetCellValue("Credit Note Items", "C1", "Product Name")
	f.SetCellValue("Credit Note Items", "D1", "Unit Price")
	f.SetCellValue("Credit Note Items", "E1", "Quantity")
	f.SetCellValue("Credit Note Items", "F1", "Subtotal")
	f.SetCellValue("Credit Note Items", "G1", "Return To Stock")

	cnItemRow := 2
	for _, cn := range creditNotes {
		orderCode := cn.Order.Code
		if orderCode == "" && cn.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", cn.OrderID)
		}
		for _, item := range cn.CreditNoteItems {
			f.SetCellValue("Credit Note Items", fmt.Sprintf("A%d", cnItemRow), cn.Code)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("B%d", cnItemRow), orderCode)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("C%d", cnItemRow), item.Name)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("D%d", cnItemRow), item.Price)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("E%d", cnItemRow), item.Quantity)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("F%d", cnItemRow), item.Subtotal)
			f.SetCellValue("Credit Note Items", fmt.Sprintf("G%d", cnItemRow), item.ReturnToStock)
			cnItemRow++
		}
	}

	// --- 7. Sheet: Debit Notes ---
	f.SetCellValue("Debit Notes", "A1", "Debit Note Code")
	f.SetCellValue("Debit Notes", "B1", "Order Ref")
	f.SetCellValue("Debit Notes", "C1", "Date / Time")
	f.SetCellValue("Debit Notes", "D1", "Staff")
	f.SetCellValue("Debit Notes", "E1", "Reason")
	f.SetCellValue("Debit Notes", "F1", "Subtotal")
	f.SetCellValue("Debit Notes", "G1", "VAT Amount")
	f.SetCellValue("Debit Notes", "H1", "Total")

	for i, dbn := range debitNotes {
		row := i + 2
		orderCode := dbn.Order.Code
		if orderCode == "" && dbn.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", dbn.OrderID)
		}
		f.SetCellValue("Debit Notes", fmt.Sprintf("A%d", row), dbn.Code)
		f.SetCellValue("Debit Notes", fmt.Sprintf("B%d", row), orderCode)
		f.SetCellValue("Debit Notes", fmt.Sprintf("C%d", row), dbn.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("Debit Notes", fmt.Sprintf("D%d", row), dbn.User.Name)
		f.SetCellValue("Debit Notes", fmt.Sprintf("E%d", row), dbn.Reason)
		f.SetCellValue("Debit Notes", fmt.Sprintf("F%d", row), dbn.Subtotal)
		f.SetCellValue("Debit Notes", fmt.Sprintf("G%d", row), dbn.TaxAmount)
		f.SetCellValue("Debit Notes", fmt.Sprintf("H%d", row), dbn.Total)
	}

	// --- 8. Sheet: Debit Note Items (NEW) ---
	f.SetCellValue("Debit Note Items", "A1", "Debit Note Code")
	f.SetCellValue("Debit Note Items", "B1", "Order Ref")
	f.SetCellValue("Debit Note Items", "C1", "Item / Charge Name")
	f.SetCellValue("Debit Note Items", "D1", "Unit Price")
	f.SetCellValue("Debit Note Items", "E1", "Quantity")
	f.SetCellValue("Debit Note Items", "F1", "Subtotal")
	f.SetCellValue("Debit Note Items", "G1", "Deduct Stock")

	dbnItemRow := 2
	for _, dbn := range debitNotes {
		orderCode := dbn.Order.Code
		if orderCode == "" && dbn.OrderID > 0 {
			orderCode = fmt.Sprintf("#%d", dbn.OrderID)
		}
		for _, item := range dbn.DebitNoteItems {
			f.SetCellValue("Debit Note Items", fmt.Sprintf("A%d", dbnItemRow), dbn.Code)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("B%d", dbnItemRow), orderCode)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("C%d", dbnItemRow), item.Name)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("D%d", dbnItemRow), item.Price)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("E%d", dbnItemRow), item.Quantity)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("F%d", dbnItemRow), item.Subtotal)
			f.SetCellValue("Debit Note Items", fmt.Sprintf("G%d", dbnItemRow), item.DeductStock)
			dbnItemRow++
		}
	}

	// --- 9. Sheet: Products ---
	f.SetCellValue("Products", "A1", "ID")
	f.SetCellValue("Products", "B1", "Name")
	f.SetCellValue("Products", "C1", "SKU")
	f.SetCellValue("Products", "D1", "Category")
	f.SetCellValue("Products", "E1", "Cost Price")
	f.SetCellValue("Products", "F1", "Selling Price")
	f.SetCellValue("Products", "G1", "VAT Rate (%)")
	f.SetCellValue("Products", "H1", "VAT Exempt")
	f.SetCellValue("Products", "I1", "Current Stock")

	for i, p := range products {
		row := i + 2
		categoryName := "Uncategorized"
		if p.Category != nil {
			categoryName = p.Category.Name
		}
		f.SetCellValue("Products", fmt.Sprintf("A%d", row), p.ID)
		f.SetCellValue("Products", fmt.Sprintf("B%d", row), p.Name)
		f.SetCellValue("Products", fmt.Sprintf("C%d", row), p.SKU)
		f.SetCellValue("Products", fmt.Sprintf("D%d", row), categoryName)
		f.SetCellValue("Products", fmt.Sprintf("E%d", row), p.Cost)
		f.SetCellValue("Products", fmt.Sprintf("F%d", row), p.Price)
		f.SetCellValue("Products", fmt.Sprintf("G%d", row), p.VatRate)
		f.SetCellValue("Products", fmt.Sprintf("H%d", row), p.VatExempt)
		f.SetCellValue("Products", fmt.Sprintf("I%d", row), p.Stock)
	}

	// --- 10. Sheet: Stock History ---
	f.SetCellValue("Stock History", "A1", "Date / Time")
	f.SetCellValue("Stock History", "B1", "Product Name")
	f.SetCellValue("Stock History", "C1", "SKU")
	f.SetCellValue("Stock History", "D1", "Type")
	f.SetCellValue("Stock History", "E1", "Quantity")
	f.SetCellValue("Stock History", "F1", "Staff")
	f.SetCellValue("Stock History", "G1", "Note")

	for i, sh := range stockHistories {
		row := i + 2
		f.SetCellValue("Stock History", fmt.Sprintf("A%d", row), sh.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("Stock History", fmt.Sprintf("B%d", row), sh.Product.Name)
		f.SetCellValue("Stock History", fmt.Sprintf("C%d", row), sh.Product.SKU)
		f.SetCellValue("Stock History", fmt.Sprintf("D%d", row), sh.Type)
		f.SetCellValue("Stock History", fmt.Sprintf("E%d", row), sh.Quantity)
		f.SetCellValue("Stock History", fmt.Sprintf("F%d", row), sh.User.Name)
		f.SetCellValue("Stock History", fmt.Sprintf("G%d", row), sh.Note)
	}

	// --- 11. Sheet: VAT Summary (For Tax Submission) ---
	summary := computeReportSummary(orders, refunds, creditNotes, debitNotes, shop)

	f.SetCellValue("VAT Summary", "A1", "Metric")
	f.SetCellValue("VAT Summary", "B1", "Amount (THB)")

	vatMetrics := []struct {
		metric string
		amount float64
	}{
		{"Gross Sales (Incl. VAT)", summary.GrossSales},
		{"Net Sales (Excl. VAT)", summary.NetSales},
		{"Output VAT", summary.VATPayable},
		{"Total Refunds (RFD)", summary.TotalRefunds},
		{"Total Credit Notes (CDN)", summary.TotalCreditNotes},
		{"Total Debit Notes (DBN)", summary.TotalDebitNotes},
		{"Net VAT Payable", summary.VATPayable},
	}

	for i, m := range vatMetrics {
		row := i + 2
		f.SetCellValue("VAT Summary", fmt.Sprintf("A%d", row), m.metric)
		f.SetCellValue("VAT Summary", fmt.Sprintf("B%d", row), m.amount)
	}

	// Add Per-Order VAT Detail Table below summary table
	startRow := len(vatMetrics) + 4
	f.SetCellValue("VAT Summary", fmt.Sprintf("A%d", startRow), "Order VAT Breakdown Detail")

	headerRow := startRow + 1
	f.SetCellValue("VAT Summary", fmt.Sprintf("A%d", headerRow), "Order Code")
	f.SetCellValue("VAT Summary", fmt.Sprintf("B%d", headerRow), "Date / Time")
	f.SetCellValue("VAT Summary", fmt.Sprintf("C%d", headerRow), "Net Sales (Excl. VAT)")
	f.SetCellValue("VAT Summary", fmt.Sprintf("D%d", headerRow), "VAT Amount")
	f.SetCellValue("VAT Summary", fmt.Sprintf("E%d", headerRow), "Gross Amount (Incl. VAT)")

	detailRow := headerRow + 1
	for _, o := range orders {
		code := o.Code
		if code == "" {
			code = fmt.Sprintf("#%d", o.ID)
		}
		f.SetCellValue("VAT Summary", fmt.Sprintf("A%d", detailRow), code)
		f.SetCellValue("VAT Summary", fmt.Sprintf("B%d", detailRow), o.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue("VAT Summary", fmt.Sprintf("C%d", detailRow), o.Subtotal)
		f.SetCellValue("VAT Summary", fmt.Sprintf("D%d", detailRow), o.TaxAmount)
		f.SetCellValue("VAT Summary", fmt.Sprintf("E%d", detailRow), o.Total)
		detailRow++
	}

	// Prepare file response
	filename := fmt.Sprintf("report_%s_%s_%s.xlsx", shop.Name, dateRange, now.Format("20060102"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("File-Name", filename)
	c.Header("Cache-Control", "no-cache")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file: " + err.Error()})
	}
}
