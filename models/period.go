package models

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrPeriodFemaleOnly = errors.New("only female users can record periods")
	ErrPeriodMaleOnly   = errors.New("only male users can view their partner's periods")
	ErrPeriodOverlap    = errors.New("period overlaps with an existing period record")
	ErrPeriodInvalid    = errors.New("end_date cannot be before start_date")
)

// Period mencatat satu siklus datang bulan. EndDate kosong (NULL) berarti
// periode masih berlangsung. Tanggal berformat YYYY-MM-DD sehingga bisa
// dibandingkan secara leksikografis.
type Period struct {
	gorm.Model
	UserId    uint    `json:"user_id" gorm:"not null;index"`
	StartDate string  `json:"start_date" gorm:"type:varchar(10);not null"`
	EndDate   *string `json:"end_date" gorm:"type:varchar(10)"`
	Note      string  `json:"note" gorm:"not null"`
}

// PeriodStatus adalah status periode partner pada tanggal tertentu.
type PeriodStatus struct {
	Date       string  `json:"date"`
	IsOnPeriod bool    `json:"is_on_period"`
	Period     *Period `json:"period"`
}

// requireGender memastikan user ada dan bergender sesuai; selain itu mengembalikan wrongGender.
func requireGender(db *gorm.DB, userId uint, gender string, wrongGender error) error {
	var user User
	if err := db.Select("id", "gender").First(&user, userId).Error; err != nil {
		return err
	}
	if strings.ToLower(user.Gender) != gender {
		return wrongGender
	}
	return nil
}

// hasPeriodOverlap mengecek apakah rentang [start, end] bertabrakan dengan periode
// lain milik user. end nil dianggap tanpa batas. excludeId=0 berarti tidak ada yang dikecualikan.
func hasPeriodOverlap(db *gorm.DB, userId uint, start string, end *string, excludeId uint) (bool, error) {
	query := db.Model(&Period{}).
		Where("user_id = ? AND id <> ?", userId, excludeId).
		Where("(end_date IS NULL OR end_date >= ?)", start)
	if end != nil {
		query = query.Where("start_date <= ?", *end)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func validatePeriod(db *gorm.DB, period *Period, excludeId uint) error {
	if period.EndDate != nil && *period.EndDate < period.StartDate {
		return ErrPeriodInvalid
	}
	overlap, err := hasPeriodOverlap(db, period.UserId, period.StartDate, period.EndDate, excludeId)
	if err != nil {
		return err
	}
	if overlap {
		return ErrPeriodOverlap
	}
	return nil
}

// CreatePeriod menyimpan periode baru; hanya user female yang boleh.
func CreatePeriod(db *gorm.DB, period *Period) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockUsers(tx, period.UserId); err != nil {
			return err
		}
		if err := requireGender(tx, period.UserId, GenderFemale, ErrPeriodFemaleOnly); err != nil {
			return err
		}
		if err := validatePeriod(tx, period, 0); err != nil {
			return err
		}
		return tx.Create(period).Error
	})
}

// UpdatePeriod menyimpan perubahan periode milik user; hanya user female yang boleh.
func UpdatePeriod(db *gorm.DB, period *Period) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockUsers(tx, period.UserId); err != nil {
			return err
		}
		if err := requireGender(tx, period.UserId, GenderFemale, ErrPeriodFemaleOnly); err != nil {
			return err
		}
		if err := validatePeriod(tx, period, period.ID); err != nil {
			return err
		}
		return tx.Save(period).Error
	})
}

// FindPeriodByID hanya mengembalikan periode milik userId tersebut.
func FindPeriodByID(db *gorm.DB, id, userId uint) (Period, error) {
	var period Period
	err := db.Where("id = ? AND user_id = ?", id, userId).First(&period).Error
	return period, err
}

// GetPeriods mengambil riwayat periode user, terbaru dulu.
func GetPeriods(db *gorm.DB, userId uint) ([]Period, error) {
	periods := []Period{}
	err := db.Where("user_id = ?", userId).Order("start_date DESC").Find(&periods).Error
	return periods, err
}

// requireActivePartnerOfMale memastikan userId laki-laki dan mengembalikan partner aktifnya.
func requireActivePartnerOfMale(db *gorm.DB, userId uint) (PartnerView, error) {
	if err := requireGender(db, userId, GenderMale, ErrPeriodMaleOnly); err != nil {
		return PartnerView{}, err
	}
	partner, err := FindActivePartner(db, userId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return partner, ErrPartnerNotActive
	}
	return partner, err
}

// GetActivePartnerPeriods mengambil riwayat periode pasangan aktif; hanya untuk user male.
func GetActivePartnerPeriods(db *gorm.DB, userId uint) ([]Period, error) {
	partner, err := requireActivePartnerOfMale(db, userId)
	if err != nil {
		return nil, err
	}
	return GetPeriods(db, partner.PartnerUserID)
}

// GetActivePartnerPeriodStatus menunjukkan apakah pasangan aktif sedang periode pada date.
func GetActivePartnerPeriodStatus(db *gorm.DB, userId uint, date string) (PeriodStatus, error) {
	status := PeriodStatus{Date: date}

	partner, err := requireActivePartnerOfMale(db, userId)
	if err != nil {
		return status, err
	}

	var period Period
	err = db.Where("user_id = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)", partner.PartnerUserID, date, date).
		Order("start_date DESC").
		First(&period).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status, nil
	}
	if err != nil {
		return status, err
	}

	status.IsOnPeriod = true
	status.Period = &period
	return status, nil
}
