package models

import (
	"net/url"
	"time"

	"gorm.io/gorm"
)

// Memory adalah kenangan/perjalanan bersama satu relasi partner (PartnerId),
// mirip event kalender yang menyimpan link dokumentasi (gdrive, dll). Kedua
// anggota relasi aktif bisa menambah, melihat, mengubah, dan menghapusnya; semua
// query selalu dibatasi oleh partner_id dari relasi aktif milik user yang login.
type Memory struct {
	gorm.Model
	PartnerId   uint      `json:"partner_id" gorm:"not null;index"`
	CreatedBy   uint      `json:"created_by" gorm:"not null"`
	Title       string    `json:"title" gorm:"type:varchar(150);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Location    string    `json:"location" gorm:"type:varchar(150)"`
	EventAt     time.Time `json:"event_at" gorm:"not null;index"`
	LinkURL     string    `json:"link_url" gorm:"type:varchar(500)"`
}

func CreateMemory(db *gorm.DB, userId uint, memory *Memory) error {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return err
	}

	memory.PartnerId = partnerId
	memory.CreatedBy = userId
	return db.Create(memory).Error
}

// GetMemories mengambil kenangan milik relasi aktif userId, urut dari yang
// terlama. start dan end opsional (zero value berarti tidak dibatasi); end
// bersifat eksklusif sehingga pemanggil mengirim awal hari berikutnya.
func GetMemories(db *gorm.DB, userId uint, start, end time.Time) ([]Memory, error) {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return nil, err
	}

	memories := []Memory{}
	query := db.Where("partner_id = ?", partnerId)
	if !start.IsZero() {
		query = query.Where("event_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("event_at < ?", end)
	}
	err = query.Order("event_at ASC, id ASC").Find(&memories).Error
	return memories, err
}

// FindMemoryByID hanya mengembalikan kenangan milik relasi aktif userId.
// Kenangan pasangan lain dikembalikan sebagai gorm.ErrRecordNotFound.
func FindMemoryByID(db *gorm.DB, id, userId uint) (Memory, error) {
	var memory Memory

	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return memory, err
	}

	err = db.Where("id = ? AND partner_id = ?", id, partnerId).First(&memory).Error
	return memory, err
}

func UpdateMemory(db *gorm.DB, id, userId uint, title, description, location string, eventAt time.Time, linkURL string) (Memory, error) {
	memory, err := FindMemoryByID(db, id, userId)
	if err != nil {
		return memory, err
	}

	memory.Title = title
	memory.Description = description
	memory.Location = location
	memory.EventAt = eventAt
	memory.LinkURL = linkURL
	err = db.Save(&memory).Error
	return memory, err
}

func DeleteMemory(db *gorm.DB, id, userId uint) error {
	memory, err := FindMemoryByID(db, id, userId)
	if err != nil {
		return err
	}
	return db.Delete(&memory).Error
}

// IsValidMemoryLink menerima link kosong (opsional) atau URL absolut http(s).
func IsValidMemoryLink(link string) bool {
	if link == "" {
		return true
	}
	u, err := url.ParseRequestURI(link)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
