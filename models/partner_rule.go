package models

import (
	"gorm.io/gorm"
)

const (
	RuleCategoryAllowed   = "allowed"   // boleh dilanggar
	RuleCategoryForbidden = "forbidden" // tidak boleh dilanggar
	RuleCategoryTolerated = "tolerated" // dalam toleransi

	RuleStatusActive   = "active"
	RuleStatusInactive = "inactive"
)

// PartnerRule adalah SOP/aturan bersama satu relasi partner (PartnerId). Kedua
// anggota relasi aktif bisa menambah, melihat, mengubah, dan menghapusnya; semua
// query selalu dibatasi oleh partner_id dari relasi aktif milik user yang login.
type PartnerRule struct {
	gorm.Model
	PartnerId   uint   `json:"partner_id" gorm:"not null;index"`
	CreatedBy   uint   `json:"created_by" gorm:"not null"`
	Title       string `json:"title" gorm:"type:varchar(150);not null"`
	Description string `json:"description" gorm:"type:text"`
	Category    string `json:"category" gorm:"type:varchar(10);not null;index"`
	// ActionPlan berisi langkah yang dilakukan jika aturan dilanggar, misalnya
	// apa yang harus dilakukan jika salah satu mendiamkan yang lain.
	ActionPlan string `json:"action_plan" gorm:"type:text"`
	Status     string `json:"status" gorm:"type:varchar(10);not null;default:active;index"`
}

func CreatePartnerRule(db *gorm.DB, userId uint, rule *PartnerRule) error {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return err
	}

	rule.PartnerId = partnerId
	rule.CreatedBy = userId
	return db.Create(rule).Error
}

// GetPartnerRules mengambil aturan milik relasi aktif userId. category dan status
// opsional; kosong berarti tidak difilter.
func GetPartnerRules(db *gorm.DB, userId uint, category, status string) ([]PartnerRule, error) {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return nil, err
	}

	rules := []PartnerRule{}
	query := db.Where("partner_id = ?", partnerId)
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err = query.Order("id DESC").Find(&rules).Error
	return rules, err
}

// FindPartnerRuleByID hanya mengembalikan aturan milik relasi aktif userId.
// Aturan pasangan lain dikembalikan sebagai gorm.ErrRecordNotFound.
func FindPartnerRuleByID(db *gorm.DB, id, userId uint) (PartnerRule, error) {
	var rule PartnerRule

	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return rule, err
	}

	err = db.Where("id = ? AND partner_id = ?", id, partnerId).First(&rule).Error
	return rule, err
}

func UpdatePartnerRule(db *gorm.DB, id, userId uint, title, description, category, actionPlan string) (PartnerRule, error) {
	rule, err := FindPartnerRuleByID(db, id, userId)
	if err != nil {
		return rule, err
	}

	rule.Title = title
	rule.Description = description
	rule.Category = category
	rule.ActionPlan = actionPlan
	err = db.Save(&rule).Error
	return rule, err
}

func SetPartnerRuleStatus(db *gorm.DB, id, userId uint, status string) (PartnerRule, error) {
	rule, err := FindPartnerRuleByID(db, id, userId)
	if err != nil {
		return rule, err
	}

	rule.Status = status
	err = db.Save(&rule).Error
	return rule, err
}

func DeletePartnerRule(db *gorm.DB, id, userId uint) error {
	rule, err := FindPartnerRuleByID(db, id, userId)
	if err != nil {
		return err
	}
	return db.Delete(&rule).Error
}

func IsValidRuleCategory(category string) bool {
	return category == RuleCategoryAllowed || category == RuleCategoryForbidden || category == RuleCategoryTolerated
}

func IsValidRuleStatus(status string) bool {
	return status == RuleStatusActive || status == RuleStatusInactive
}
