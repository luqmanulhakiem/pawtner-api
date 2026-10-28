package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ConflictStatusOngoing  = "ongoing"
	ConflictStatusResolved = "resolved"
)

var (
	ErrConflictResolved = errors.New("conflict is already resolved")
)

// Conflict dimiliki oleh satu relasi partner (PartnerId), bukan oleh user. Hanya
// dua anggota relasi aktif tersebut yang bisa membuat dan melihatnya; semua query
// selalu dibatasi oleh partner_id dari relasi aktif milik user yang sedang login.
type Conflict struct {
	gorm.Model
	PartnerId    uint       `json:"partner_id" gorm:"not null;index"`
	CreatedBy    uint       `json:"created_by" gorm:"not null"`
	Title        string     `json:"title" gorm:"type:varchar(150);not null"`
	Description  string     `json:"description" gorm:"type:text"`
	ConflictDate string     `json:"conflict_date" gorm:"type:varchar(10);not null"` // format YYYY-MM-DD
	Status       string     `json:"status" gorm:"type:varchar(10);not null;default:ongoing;index"`
	Resolution   string     `json:"resolution" gorm:"type:text"`
	ResolvedBy   *uint      `json:"resolved_by"`
	ResolvedAt   *time.Time `json:"resolved_at"`
}

// ConflictNote adalah catatan kegiatan/perkembangan pada sebuah conflict.
type ConflictNote struct {
	gorm.Model
	ConflictId uint   `json:"conflict_id" gorm:"not null;index"`
	UserId     uint   `json:"user_id" gorm:"not null"`
	Note       string `json:"note" gorm:"type:text;not null"`
}

// ConflictDetail adalah conflict beserta timeline catatannya.
type ConflictDetail struct {
	Conflict
	Notes []ConflictNote `json:"notes"`
}

// activePartnerRelationID mengembalikan id relasi aktif milik user, atau
// ErrPartnerNotActive jika user tidak punya pasangan aktif.
func activePartnerRelationID(db *gorm.DB, userId uint) (uint, error) {
	partner, err := FindActivePartner(db, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrPartnerNotActive
		}
		return 0, err
	}
	return partner.ID, nil
}

func CreateConflict(db *gorm.DB, userId uint, conflict *Conflict) error {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return err
	}

	conflict.PartnerId = partnerId
	conflict.CreatedBy = userId
	conflict.Status = ConflictStatusOngoing
	return db.Create(conflict).Error
}

// GetConflicts mengambil conflict milik relasi aktif userId, terbaru dulu.
// Jika status tidak kosong, hasil difilter pada status tersebut. from/to (YYYY-MM-DD,
// inklusif) membatasi conflict_date; masing-masing boleh kosong.
func GetConflicts(db *gorm.DB, userId uint, status, from, to string) ([]Conflict, error) {
	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return nil, err
	}

	conflicts := []Conflict{}
	query := db.Where("partner_id = ?", partnerId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if from != "" {
		query = query.Where("conflict_date >= ?", from)
	}
	if to != "" {
		query = query.Where("conflict_date <= ?", to)
	}
	err = query.Order("conflict_date DESC, id DESC").Find(&conflicts).Error
	return conflicts, err
}

// FindConflictByID hanya mengembalikan conflict milik relasi aktif userId.
// Conflict pasangan lain dikembalikan sebagai gorm.ErrRecordNotFound.
func FindConflictByID(db *gorm.DB, id, userId uint) (Conflict, error) {
	var conflict Conflict

	partnerId, err := activePartnerRelationID(db, userId)
	if err != nil {
		return conflict, err
	}

	err = db.Where("id = ? AND partner_id = ?", id, partnerId).First(&conflict).Error
	return conflict, err
}

func GetConflictDetail(db *gorm.DB, id, userId uint) (ConflictDetail, error) {
	detail := ConflictDetail{Notes: []ConflictNote{}}

	conflict, err := FindConflictByID(db, id, userId)
	if err != nil {
		return detail, err
	}
	detail.Conflict = conflict

	err = db.Where("conflict_id = ?", conflict.ID).Order("created_at ASC, id ASC").Find(&detail.Notes).Error
	return detail, err
}

// UpdateConflict mengubah isi conflict yang masih berlangsung.
func UpdateConflict(db *gorm.DB, id, userId uint, title, description, conflictDate string) (Conflict, error) {
	var conflict Conflict

	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		if conflict, err = lockConflict(tx, id, userId); err != nil {
			return err
		}
		if conflict.Status == ConflictStatusResolved {
			return ErrConflictResolved
		}

		conflict.Title = title
		conflict.Description = description
		conflict.ConflictDate = conflictDate
		return tx.Save(&conflict).Error
	})

	return conflict, err
}

// ResolveConflict menandai conflict selesai; boleh dilakukan salah satu dari pasangan.
func ResolveConflict(db *gorm.DB, id, userId uint, resolution string) (Conflict, error) {
	var conflict Conflict

	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		if conflict, err = lockConflict(tx, id, userId); err != nil {
			return err
		}
		if conflict.Status == ConflictStatusResolved {
			return ErrConflictResolved
		}

		now := time.Now()
		conflict.Status = ConflictStatusResolved
		conflict.Resolution = resolution
		conflict.ResolvedBy = &userId
		conflict.ResolvedAt = &now
		return tx.Save(&conflict).Error
	})

	return conflict, err
}

// AddConflictNote menambah catatan ke conflict yang masih berlangsung.
func AddConflictNote(db *gorm.DB, id, userId uint, note string) (ConflictNote, error) {
	conflictNote := ConflictNote{ConflictId: id, UserId: userId, Note: note}

	err := db.Transaction(func(tx *gorm.DB) error {
		conflict, err := lockConflict(tx, id, userId)
		if err != nil {
			return err
		}
		if conflict.Status == ConflictStatusResolved {
			return ErrConflictResolved
		}
		return tx.Create(&conflictNote).Error
	})

	return conflictNote, err
}

// lockConflict mengunci conflict milik relasi aktif userId agar perubahan status
// dan penambahan catatan tidak saling balapan.
func lockConflict(tx *gorm.DB, id, userId uint) (Conflict, error) {
	var conflict Conflict

	partnerId, err := activePartnerRelationID(tx, userId)
	if err != nil {
		return conflict, err
	}

	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND partner_id = ?", id, partnerId).
		First(&conflict).Error
	return conflict, err
}

func IsValidConflictStatus(status string) bool {
	return status == ConflictStatusOngoing || status == ConflictStatusResolved
}
