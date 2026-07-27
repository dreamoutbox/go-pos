# Implementation Plan — Excel Report Export with Item Breakdown Sheets

Add an **Export Excel** feature to the Business Reports & Analytics page that downloads a multi-sheet `.xlsx` workbook containing all business summary data, document history, and item breakdown details for the selected date range.

---

## User Review Required

> [!IMPORTANT]
> The Excel report workbook will be expanded from 8 sheets to **11 sheets** to include granular item-level breakdown details for Refunds, Credit Notes, and Debit Notes.

---

## Workbook Structure (11 Sheets)

### 1. Orders
- Columns: `Order Code`, `Date / Time`, `Cashier`, `Status`, `Subtotal (Net)`, `VAT Amount`, `Total (Gross)`

### 2. Order Items (Breakdown Detail for Orders)
- Columns: `Order Code`, `Product Name`, `Unit Price`, `Cost`, `Quantity`, `Subtotal`

### 3. Refunds
- Columns: `Refund Code`, `Order Ref`, `Date / Time`, `Staff`, `Reason`, `Subtotal`, `VAT Amount`, `Total`

### 4. Refund Items (NEW - Breakdown Detail for Refunds)
- Columns: `Refund Code`, `Order Ref`, `Product Name`, `Unit Price`, `Quantity`, `Subtotal`, `Return To Stock`

### 5. Credit Notes
- Columns: `Credit Note Code`, `Order Ref`, `Date / Time`, `Staff`, `Reason`, `Subtotal`, `VAT Amount`, `Total`

### 6. Credit Note Items (NEW - Breakdown Detail for Credit Notes)
- Columns: `Credit Note Code`, `Order Ref`, `Product Name`, `Unit Price`, `Quantity`, `Subtotal`, `Return To Stock`

### 7. Debit Notes
- Columns: `Debit Note Code`, `Order Ref`, `Date / Time`, `Staff`, `Reason`, `Subtotal`, `VAT Amount`, `Total`

### 8. Debit Note Items (NEW - Breakdown Detail for Debit Notes)
- Columns: `Debit Note Code`, `Order Ref`, `Item / Charge Name`, `Unit Price`, `Quantity`, `Subtotal`, `Deduct Stock`

### 9. Products
- Columns: `ID`, `Name`, `SKU`, `Category`, `Cost Price`, `Selling Price`, `VAT Rate (%)`, `VAT Exempt`, `Current Stock`

### 10. Stock History
- Columns: `Date / Time`, `Product Name`, `SKU`, `Type`, `Quantity`, `Staff`, `Note`

### 11. VAT Summary (For Tax Submission)
- Summary Table: `Gross Sales (Incl. VAT)`, `Net Sales (Excl. VAT)`, `Output VAT`, `Total Refunds`, `Total Credit Notes`, `Total Debit Notes`, `Net VAT Payable`
- Per-Order VAT Detail Table: `Order Code`, `Date / Time`, `Net Sales (Excl. VAT)`, `VAT Amount`, `Gross Amount (Incl. VAT)`

---

## Proposed Changes

### Handlers Component

#### [MODIFY] [export.go](file:///home/z/git/go-pos/handlers/export.go)
- Add preloads for `RefundItems`, `CreditNoteItems`, and `DebitNoteItems`.
- Create and populate the 3 new sheets: `"Refund Items"`, `"Credit Note Items"`, and `"Debit Note Items"`.

---

## Verification Plan

### Automated Tests
- Run `go build ./...` to verify clean compilation.

### Manual Verification
- Access `http://localhost:8080/reports/export?range=today` (and `week`/`month`).
- Verify all 11 sheets are populated with correct headers and data rows.
