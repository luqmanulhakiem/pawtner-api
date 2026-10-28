package models

import (
	"strings"

	"gorm.io/gorm"
)

const (
	MoodPeriodMorning = "morning" // bangun tidur
	MoodPeriodNight   = "night"   // sebelum tidur
)

const (
	MoodStatusNeutral = "neutral"
	MoodStatusSad     = "sad"
	MoodStatusAngry   = "angry"
	MoodStatusSick    = "sick"
	MoodStatusHappy   = "happy"
	MoodStatusGlad    = "glad"
)

// DailyMood mencatat mood user. Setiap user maksimal punya 1 mood per
// periode (morning/night) per tanggal, dijaga oleh unique index idx_user_date_period.
type DailyMood struct {
	gorm.Model
	UserId     uint   `json:"user_id" gorm:"not null;uniqueIndex:idx_user_date_period"`
	Date       string `json:"date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_date_period"` // format YYYY-MM-DD
	Period     string `json:"period" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_date_period"`
	MoodStatus string `json:"mood_status" gorm:"not null"`
	Note       string `json:"note" gorm:"not null"`
}

func IsValidMoodPeriod(period string) bool {
	return period == MoodPeriodMorning || period == MoodPeriodNight
}

// NormalizeMoodStatus merapikan input (huruf kecil, tanpa spasi tepi) dan
// melaporkan apakah hasilnya termasuk status baku.
func NormalizeMoodStatus(status string) (string, bool) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case MoodStatusNeutral, MoodStatusSad, MoodStatusAngry, MoodStatusSick, MoodStatusHappy, MoodStatusGlad:
		return status, true
	}
	return status, false
}

func CreateDailyMood(db *gorm.DB, mood *DailyMood) error {
	return db.Create(mood).Error
}

// FindDailyMoodByPeriod mencari mood user pada tanggal dan periode tertentu.
func FindDailyMoodByPeriod(db *gorm.DB, userId uint, date, period string) (DailyMood, error) {
	var mood DailyMood
	err := db.Where("user_id = ? AND date = ? AND period = ?", userId, date, period).First(&mood).Error
	return mood, err
}

// FindDailyMoodByID hanya mengembalikan mood milik userId tersebut.
func FindDailyMoodByID(db *gorm.DB, id, userId uint) (DailyMood, error) {
	var mood DailyMood
	err := db.Where("id = ? AND user_id = ?", id, userId).First(&mood).Error
	return mood, err
}

// GetDailyMoods mengambil semua mood user, terbaru dulu. Jika date tidak
// kosong, hasil difilter pada tanggal tersebut.
func GetDailyMoods(db *gorm.DB, userId uint, date string) ([]DailyMood, error) {
	var moods []DailyMood
	query := db.Where("user_id = ?", userId)
	if date != "" {
		query = query.Where("date = ?", date)
	}
	err := query.Order("date DESC, period ASC").Find(&moods).Error
	return moods, err
}

func UpdateDailyMood(db *gorm.DB, mood *DailyMood) error {
	return db.Save(mood).Error
}
