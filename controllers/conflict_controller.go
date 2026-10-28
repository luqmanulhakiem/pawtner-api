package controllers

import (
	"errors"
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"pawtner-api/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ConflictInput struct {
	Title        string `json:"title" binding:"required,max=150"`
	Description  string `json:"description"`
	ConflictDate string `json:"conflict_date" binding:"required"` // YYYY-MM-DD
}

type ConflictResolveInput struct {
	Resolution string `json:"resolution"`
}

type ConflictNoteInput struct {
	Note string `json:"note" binding:"required"`
}

// conflictError memetakan error business logic conflict ke respons HTTP.
func conflictError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, models.ErrPartnerNotActive):
		helpers.Error(c, http.StatusForbidden, "You don't have an active partner")
	case errors.Is(err, models.ErrConflictResolved):
		helpers.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		helpers.Error(c, http.StatusNotFound, "Conflict not found")
	default:
		helpers.Error(c, http.StatusInternalServerError, fallback)
	}
}

// conflictIDParam membaca :id dari path dan menulis respons error jika tidak valid.
func conflictIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid conflict id")
		return 0, false
	}
	return uint(id), true
}

func CreateConflict(c *gin.Context) {
	var input ConflictInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := time.Parse(moodDateLayout, input.ConflictDate); err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	conflict := models.Conflict{
		Title:        strings.TrimSpace(input.Title),
		Description:  input.Description,
		ConflictDate: input.ConflictDate,
	}
	if conflict.Title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}

	if err := models.CreateConflict(config.DB.WithContext(c.Request.Context()), userId, &conflict); err != nil {
		conflictError(c, err, "Failed to save conflict")
		return
	}

	helpers.Success(c, http.StatusCreated, "Conflict recorded successfully", conflict)
}

// GetConflicts menampilkan conflict milik pasangan aktif: GET /conflicts?status=ongoing|resolved&from=YYYY-MM-DD&to=YYYY-MM-DD
func GetConflicts(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	status := c.Query("status")
	if status != "" && !models.IsValidConflictStatus(status) {
		helpers.Error(c, http.StatusBadRequest, "Invalid status, use 'ongoing' or 'resolved'")
		return
	}

	from, to := c.Query("from"), c.Query("to")
	for _, date := range []string{from, to} {
		if date == "" {
			continue
		}
		if _, err := time.Parse(moodDateLayout, date); err != nil {
			helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
			return
		}
	}
	if from != "" && to != "" && from > to {
		helpers.Error(c, http.StatusBadRequest, "Query 'from' must not be after 'to'")
		return
	}

	conflicts, err := models.GetConflicts(config.DB.WithContext(c.Request.Context()), userId, status, from, to)
	if err != nil {
		conflictError(c, err, "Failed to fetch conflicts")
		return
	}

	helpers.Success(c, http.StatusOK, "Conflicts fetched successfully", conflicts)
}

// GetConflict menampilkan satu conflict beserta timeline catatannya.
func GetConflict(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := conflictIDParam(c)
	if !ok {
		return
	}

	detail, err := models.GetConflictDetail(config.DB.WithContext(c.Request.Context()), id, userId)
	if err != nil {
		conflictError(c, err, "Failed to fetch conflict")
		return
	}

	helpers.Success(c, http.StatusOK, "Conflict fetched successfully", detail)
}

func UpdateConflict(c *gin.Context) {
	var input ConflictInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := conflictIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := time.Parse(moodDateLayout, input.ConflictDate); err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}

	conflict, err := models.UpdateConflict(config.DB.WithContext(c.Request.Context()), id, userId, title, input.Description, input.ConflictDate)
	if err != nil {
		conflictError(c, err, "Failed to update conflict")
		return
	}

	helpers.Success(c, http.StatusOK, "Conflict updated successfully", conflict)
}

func ResolveConflict(c *gin.Context) {
	var input ConflictResolveInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := conflictIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	conflict, err := models.ResolveConflict(config.DB.WithContext(c.Request.Context()), id, userId, input.Resolution)
	if err != nil {
		conflictError(c, err, "Failed to resolve conflict")
		return
	}

	helpers.Success(c, http.StatusOK, "Conflict resolved successfully", conflict)
}

func AddConflictNote(c *gin.Context) {
	var input ConflictNoteInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := conflictIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	note := strings.TrimSpace(input.Note)
	if note == "" {
		helpers.Error(c, http.StatusBadRequest, "Note is required")
		return
	}

	conflictNote, err := models.AddConflictNote(config.DB.WithContext(c.Request.Context()), id, userId, note)
	if err != nil {
		conflictError(c, err, "Failed to save conflict note")
		return
	}

	helpers.Success(c, http.StatusCreated, "Conflict note added successfully", conflictNote)
}
