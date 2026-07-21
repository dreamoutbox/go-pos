package utils

import (
	"fmt"
	"time"

	"github.com/dreamoutbox/go-pos/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GenerateDocumentCode creates a running number scoped by shop, prefix, and YYYYMM.
// Format: PREFIX-YYYYMM-00001 (e.g., ORD-202607-00001, RFD-202607-00012)
// Must be called within a database transaction (tx).
func GenerateDocumentCode(tx *gorm.DB, shopID uint, prefix string, t time.Time) (string, error) {
	yearMonth := t.Format("200601")

	var seq models.DocumentSequence

	// Lock the sequence row or create it if missing
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("shop_id = ? AND prefix = ? AND year_month = ?", shopID, prefix, yearMonth).
		First(&seq).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			seq = models.DocumentSequence{
				ShopID:    shopID,
				Prefix:    prefix,
				YearMonth: yearMonth,
				LastValue: 1,
			}
			if createErr := tx.Create(&seq).Error; createErr != nil {
				return "", createErr
			}
		} else {
			return "", err
		}
	} else {
		seq.LastValue++
		if saveErr := tx.Save(&seq).Error; saveErr != nil {
			return "", saveErr
		}
	}

	code := fmt.Sprintf("%s-%s-%05d", prefix, yearMonth, seq.LastValue)

	return code, nil
}
