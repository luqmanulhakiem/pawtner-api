package models

import (
	"crypto/rand"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	GenderMale   = "male"
	GenderFemale = "female"
)

// uniqueCodeAlphabet tanpa karakter yang mirip (0/O, 1/I) agar mudah dibaca.
// Panjangnya 32 sehingga pemetaan byte -> karakter tidak bias.
const uniqueCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const uniqueCodeLength = 8

type User struct {
	gorm.Model
	FullName     string    `json:"full_name" gorm:"not null"`
	Email        string    `json:"email" gorm:"unique"`
	Password     string    `json:"password" gorm:"not null"`
	Gender       string    `json:"gender" gorm:"not null"`
	UniqueCode   *string   `json:"unique_code" gorm:"size:10;uniqueIndex"` // kode untuk dicari/diundang oleh calon partner
	IsActive     bool      `json:"is_active" gorm:"default:false"`
	RefreshToken *string   `json:"refresh_token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BeforeCreate mengisi UniqueCode otomatis saat user baru dibuat.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.UniqueCode != nil {
		return nil
	}
	code, err := generateUniqueCode(tx)
	if err != nil {
		return err
	}
	u.UniqueCode = &code
	return nil
}

func generateUniqueCode(db *gorm.DB) (string, error) {
	db = db.Session(&gorm.Session{NewDB: true})
	for i := 0; i < 5; i++ {
		buf := make([]byte, uniqueCodeLength)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for j, b := range buf {
			buf[j] = uniqueCodeAlphabet[int(b)%len(uniqueCodeAlphabet)]
		}
		code := string(buf)

		var count int64
		if err := db.Unscoped().Model(&User{}).Where("unique_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("could not generate unique code")
}

// BackfillUserUniqueCodes memberi kode untuk user lama yang belum memilikinya.
func BackfillUserUniqueCodes(db *gorm.DB) error {
	var users []User
	if err := db.Where("unique_code IS NULL").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		code, err := generateUniqueCode(db)
		if err != nil {
			return err
		}
		if err := db.Model(&User{}).Where("id = ?", user.ID).UpdateColumn("unique_code", code).Error; err != nil {
			return err
		}
	}
	return nil
}

func FindUserByUniqueCode(db *gorm.DB, code string) (User, error) {
	var user User
	err := db.Where("unique_code = ?", code).First(&user).Error
	return user, err
}
