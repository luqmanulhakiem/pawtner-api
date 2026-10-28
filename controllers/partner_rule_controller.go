package controllers

import (
	"errors"
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"pawtner-api/models"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PartnerRuleInput struct {
	Title       string `json:"title" binding:"required,max=150"`
	Description string `json:"description"`
	Category    string `json:"category" binding:"required"` // allowed | forbidden | tolerated
	ActionPlan  string `json:"action_plan"`
	Status      string `json:"status"` // active | inactive, hanya dipakai saat membuat (default active)
}

type PartnerRuleStatusInput struct {
	Status string `json:"status" binding:"required"`
}

const invalidRuleCategoryMessage = "Invalid category, use 'allowed', 'forbidden' or 'tolerated'"

// partnerRuleError memetakan error business logic aturan partner ke respons HTTP.
func partnerRuleError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, models.ErrPartnerNotActive):
		helpers.Error(c, http.StatusForbidden, "You don't have an active partner")
	case errors.Is(err, gorm.ErrRecordNotFound):
		helpers.Error(c, http.StatusNotFound, "Rule not found")
	default:
		helpers.Error(c, http.StatusInternalServerError, fallback)
	}
}

// partnerRuleIDParam membaca :id dari path dan menulis respons error jika tidak valid.
func partnerRuleIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid rule id")
		return 0, false
	}
	return uint(id), true
}

func CreatePartnerRule(c *gin.Context) {
	var input PartnerRuleInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidRuleCategory(input.Category) {
		helpers.Error(c, http.StatusBadRequest, invalidRuleCategoryMessage)
		return
	}

	status := input.Status
	if status == "" {
		status = models.RuleStatusActive
	}
	if !models.IsValidRuleStatus(status) {
		helpers.Error(c, http.StatusBadRequest, "Invalid status, use 'active' or 'inactive'")
		return
	}

	rule := models.PartnerRule{
		Title:       strings.TrimSpace(input.Title),
		Description: input.Description,
		Category:    input.Category,
		ActionPlan:  input.ActionPlan,
		Status:      status,
	}
	if rule.Title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}

	if err := models.CreatePartnerRule(config.DB.WithContext(c.Request.Context()), userId, &rule); err != nil {
		partnerRuleError(c, err, "Failed to save rule")
		return
	}

	helpers.Success(c, http.StatusCreated, "Rule added successfully", rule)
}

// GetPartnerRules menampilkan aturan pasangan aktif: GET /rules?category=allowed|forbidden|tolerated&status=active|inactive
func GetPartnerRules(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	category := c.Query("category")
	if category != "" && !models.IsValidRuleCategory(category) {
		helpers.Error(c, http.StatusBadRequest, invalidRuleCategoryMessage)
		return
	}
	status := c.Query("status")
	if status != "" && !models.IsValidRuleStatus(status) {
		helpers.Error(c, http.StatusBadRequest, "Invalid status, use 'active' or 'inactive'")
		return
	}

	rules, err := models.GetPartnerRules(config.DB.WithContext(c.Request.Context()), userId, category, status)
	if err != nil {
		partnerRuleError(c, err, "Failed to fetch rules")
		return
	}

	helpers.Success(c, http.StatusOK, "Rules fetched successfully", rules)
}

func GetPartnerRule(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := partnerRuleIDParam(c)
	if !ok {
		return
	}

	rule, err := models.FindPartnerRuleByID(config.DB.WithContext(c.Request.Context()), id, userId)
	if err != nil {
		partnerRuleError(c, err, "Failed to fetch rule")
		return
	}

	helpers.Success(c, http.StatusOK, "Rule fetched successfully", rule)
}

func UpdatePartnerRule(c *gin.Context) {
	var input PartnerRuleInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := partnerRuleIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidRuleCategory(input.Category) {
		helpers.Error(c, http.StatusBadRequest, invalidRuleCategoryMessage)
		return
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}

	rule, err := models.UpdatePartnerRule(config.DB.WithContext(c.Request.Context()), id, userId, title, input.Description, input.Category, input.ActionPlan)
	if err != nil {
		partnerRuleError(c, err, "Failed to update rule")
		return
	}

	helpers.Success(c, http.StatusOK, "Rule updated successfully", rule)
}

// SetPartnerRuleStatus mengaktifkan/menonaktifkan aturan: PUT /rules/:id/status
func SetPartnerRuleStatus(c *gin.Context) {
	var input PartnerRuleStatusInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := partnerRuleIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidRuleStatus(input.Status) {
		helpers.Error(c, http.StatusBadRequest, "Invalid status, use 'active' or 'inactive'")
		return
	}

	rule, err := models.SetPartnerRuleStatus(config.DB.WithContext(c.Request.Context()), id, userId, input.Status)
	if err != nil {
		partnerRuleError(c, err, "Failed to update rule status")
		return
	}

	helpers.Success(c, http.StatusOK, "Rule status updated successfully", rule)
}

func DeletePartnerRule(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := partnerRuleIDParam(c)
	if !ok {
		return
	}

	if err := models.DeletePartnerRule(config.DB.WithContext(c.Request.Context()), id, userId); err != nil {
		partnerRuleError(c, err, "Failed to delete rule")
		return
	}

	helpers.Success(c, http.StatusOK, "Rule deleted successfully", nil)
}
