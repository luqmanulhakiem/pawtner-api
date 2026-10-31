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

type MemoryInput struct {
	Title       string    `json:"title" binding:"required,max=150"`
	Description string    `json:"description"`
	Location    string    `json:"location" binding:"max=150"`
	EventAt     time.Time `json:"event_at" binding:"required"` // RFC3339, mis. 2026-10-31T09:00:00+07:00
	LinkURL     string    `json:"link_url" binding:"max=500"`  // link dokumentasi (gdrive, dll), opsional
}

const (
	memoryDateLayout        = "2006-01-02"
	invalidMemoryLinkMsg    = "Invalid link_url, use an absolute http(s) URL"
	invalidMemoryDateFormat = "Invalid date, use format YYYY-MM-DD"
)

// memoryError memetakan error business logic kenangan ke respons HTTP.
func memoryError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, models.ErrPartnerNotActive):
		helpers.Error(c, http.StatusForbidden, "You don't have an active partner")
	case errors.Is(err, gorm.ErrRecordNotFound):
		helpers.Error(c, http.StatusNotFound, "Memory not found")
	default:
		helpers.Error(c, http.StatusInternalServerError, fallback)
	}
}

// memoryIDParam membaca :id dari path dan menulis respons error jika tidak valid.
func memoryIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid memory id")
		return 0, false
	}
	return uint(id), true
}

func CreateMemory(c *gin.Context) {
	var input MemoryInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	memory := models.Memory{
		Title:       strings.TrimSpace(input.Title),
		Description: input.Description,
		Location:    strings.TrimSpace(input.Location),
		EventAt:     input.EventAt,
		LinkURL:     strings.TrimSpace(input.LinkURL),
	}
	if memory.Title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}
	if !models.IsValidMemoryLink(memory.LinkURL) {
		helpers.Error(c, http.StatusBadRequest, invalidMemoryLinkMsg)
		return
	}

	if err := models.CreateMemory(config.DB.WithContext(c.Request.Context()), userId, &memory); err != nil {
		memoryError(c, err, "Failed to save memory")
		return
	}

	helpers.Success(c, http.StatusCreated, "Memory added successfully", memory)
}

// GetMemories menampilkan kalender kenangan pasangan aktif:
// GET /memories?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD (keduanya opsional, inklusif)
func GetMemories(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	var start, end time.Time
	if s := c.Query("start_date"); s != "" {
		parsed, err := time.ParseInLocation(memoryDateLayout, s, time.Local)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, invalidMemoryDateFormat)
			return
		}
		start = parsed
	}
	if e := c.Query("end_date"); e != "" {
		parsed, err := time.ParseInLocation(memoryDateLayout, e, time.Local)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, invalidMemoryDateFormat)
			return
		}
		// end_date inklusif: batas atas eksklusif adalah awal hari berikutnya.
		end = parsed.AddDate(0, 0, 1)
	}
	if !start.IsZero() && !end.IsZero() && !start.Before(end) {
		helpers.Error(c, http.StatusBadRequest, "start_date must not be after end_date")
		return
	}

	memories, err := models.GetMemories(config.DB.WithContext(c.Request.Context()), userId, start, end)
	if err != nil {
		memoryError(c, err, "Failed to fetch memories")
		return
	}

	helpers.Success(c, http.StatusOK, "Memories fetched successfully", memories)
}

func GetMemory(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := memoryIDParam(c)
	if !ok {
		return
	}

	memory, err := models.FindMemoryByID(config.DB.WithContext(c.Request.Context()), id, userId)
	if err != nil {
		memoryError(c, err, "Failed to fetch memory")
		return
	}

	helpers.Success(c, http.StatusOK, "Memory fetched successfully", memory)
}

func UpdateMemory(c *gin.Context) {
	var input MemoryInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := memoryIDParam(c)
	if !ok {
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		helpers.Error(c, http.StatusBadRequest, "Title is required")
		return
	}
	link := strings.TrimSpace(input.LinkURL)
	if !models.IsValidMemoryLink(link) {
		helpers.Error(c, http.StatusBadRequest, invalidMemoryLinkMsg)
		return
	}

	memory, err := models.UpdateMemory(config.DB.WithContext(c.Request.Context()), id, userId, title, input.Description, strings.TrimSpace(input.Location), input.EventAt, link)
	if err != nil {
		memoryError(c, err, "Failed to update memory")
		return
	}

	helpers.Success(c, http.StatusOK, "Memory updated successfully", memory)
}

func DeleteMemory(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, ok := memoryIDParam(c)
	if !ok {
		return
	}

	if err := models.DeleteMemory(config.DB.WithContext(c.Request.Context()), id, userId); err != nil {
		memoryError(c, err, "Failed to delete memory")
		return
	}

	helpers.Success(c, http.StatusOK, "Memory deleted successfully", nil)
}
