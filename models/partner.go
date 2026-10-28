package models

import (
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PartnerStatusPending  = "pending"  // pending when partner request a invitation
	PartnerStatusActive   = "active"   // active when partner in relationship
	PartnerStatusDisabled = "disabled" // disabled when partner are broke up
	PartnerStatusRejected = "rejected" // reject partner invitation
)

var (
	ErrPartnerSelfInvite     = errors.New("you cannot invite yourself")
	ErrPartnerInvalidGender  = errors.New("gender must be 'male' or 'female' to use partner feature")
	ErrPartnerSameGender     = errors.New("partner must be of the opposite gender")
	ErrPartnerMaleOnlyInvite = errors.New("only male users can send partner invitations")
	ErrPartnerInviterTaken   = errors.New("you already have a partner")
	ErrPartnerInviteeTaken   = errors.New("this user already has a partner")
	ErrPartnerPendingExists  = errors.New("a pending invitation already exists between you and this user")
	ErrPartnerForbidden      = errors.New("you are not allowed to perform this action")
	ErrPartnerNotPending     = errors.New("invitation is no longer pending")
	ErrPartnerNotActive      = errors.New("partner relationship is not active")
)

type Partner struct {
	gorm.Model
	Status    string    `json:"status" gorm:"not null;default:pending"`
	MaleId    uint      `json:"male_id" gorm:"not null;index"`
	FemaleId  uint      `json:"female_id" gorm:"not null;index"`
	InvitedBy uint      `json:"invited_by" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PartnerView adalah data relasi dari sudut pandang satu user, berisi profil lawannya.
type PartnerView struct {
	ID            uint      `json:"id"`
	Status        string    `json:"status"`
	InvitedBy     uint      `json:"invited_by"`
	PartnerUserID uint      `json:"partner_user_id"`
	FullName      string    `json:"full_name"`
	Gender        string    `json:"gender"`
	UniqueCode    *string   `json:"unique_code"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p Partner) hasMember(userId uint) bool {
	return p.MaleId == userId || p.FemaleId == userId
}

// lockUsers mengunci baris user (urut id, mencegah deadlock) agar pengecekan
// "satu user hanya satu partner aktif" aman dari race condition.
func lockUsers(tx *gorm.DB, ids ...uint) (map[uint]User, error) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var users []User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&users).Error; err != nil {
		return nil, err
	}

	result := make(map[uint]User, len(users))
	for _, user := range users {
		result[user.ID] = user
	}
	return result, nil
}

func hasActivePartner(tx *gorm.DB, userId uint) (bool, error) {
	var count int64
	err := tx.Model(&Partner{}).
		Where("status = ? AND (male_id = ? OR female_id = ?)", PartnerStatusActive, userId, userId).
		Count(&count).Error
	return count > 0, err
}

// InvitePartner membuat undangan dari inviterId ke user pemilik code.
func InvitePartner(db *gorm.DB, inviterId uint, code string) (Partner, error) {
	var partner Partner

	err := db.Transaction(func(tx *gorm.DB) error {
		target, err := FindUserByUniqueCode(tx, code)
		if err != nil {
			return err // gorm.ErrRecordNotFound jika kode tidak ada
		}
		if target.ID == inviterId {
			return ErrPartnerSelfInvite
		}

		users, err := lockUsers(tx, inviterId, target.ID)
		if err != nil {
			return err
		}
		inviter, invitee := users[inviterId], users[target.ID]
		if inviter.ID == 0 || invitee.ID == 0 {
			return gorm.ErrRecordNotFound
		}

		inviterGender, inviteeGender := strings.ToLower(inviter.Gender), strings.ToLower(invitee.Gender)
		if !isPartnerGender(inviterGender) || !isPartnerGender(inviteeGender) {
			return ErrPartnerInvalidGender
		}
		if inviterGender == inviteeGender {
			return ErrPartnerSameGender
		}

		if inviterGender != GenderMale {
			return ErrPartnerMaleOnlyInvite
		}

		if taken, err := hasActivePartner(tx, inviter.ID); err != nil {
			return err
		} else if taken {
			return ErrPartnerInviterTaken
		}
		if taken, err := hasActivePartner(tx, invitee.ID); err != nil {
			return err
		} else if taken {
			return ErrPartnerInviteeTaken
		}

		var pending int64
		if err := tx.Model(&Partner{}).
			Where("status = ? AND (male_id IN ? AND female_id IN ?)", PartnerStatusPending, []uint{inviter.ID, invitee.ID}, []uint{inviter.ID, invitee.ID}).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrPartnerPendingExists
		}

		partner = Partner{Status: PartnerStatusPending, InvitedBy: inviter.ID, MaleId: inviter.ID, FemaleId: invitee.ID}
		return tx.Create(&partner).Error
	})

	return partner, err
}

func isPartnerGender(gender string) bool {
	return gender == GenderMale || gender == GenderFemale
}

// respondInvitation dipakai accept/reject: hanya user yang diundang (bukan pengundang) yang boleh.
func respondInvitation(db *gorm.DB, partnerId, userId uint, newStatus string) (Partner, error) {
	var partner Partner

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&partner, partnerId).Error; err != nil {
			return err
		}
		if !partner.hasMember(userId) {
			return gorm.ErrRecordNotFound
		}
		if partner.InvitedBy == userId {
			return ErrPartnerForbidden
		}

		if newStatus == PartnerStatusActive {
			if _, err := lockUsers(tx, partner.MaleId, partner.FemaleId); err != nil {
				return err
			}
		}
		// Status dibaca setelah lock, sehingga hasilnya konsisten
		if partner.Status != PartnerStatusPending {
			return ErrPartnerNotPending
		}

		if newStatus == PartnerStatusActive {
			for _, id := range []uint{partner.MaleId, partner.FemaleId} {
				if taken, err := hasActivePartner(tx, id); err != nil {
					return err
				} else if taken {
					if id == userId {
						return ErrPartnerInviterTaken
					}
					return ErrPartnerInviteeTaken
				}
			}
		}

		partner.Status = newStatus
		if err := tx.Save(&partner).Error; err != nil {
			return err
		}

		if newStatus == PartnerStatusActive {
			// Undangan pending lain yang melibatkan salah satu dari keduanya otomatis ditolak
			ids := []uint{partner.MaleId, partner.FemaleId}
			return tx.Model(&Partner{}).
				Where("status = ? AND id <> ? AND (male_id IN ? OR female_id IN ?)", PartnerStatusPending, partner.ID, ids, ids).
				Update("status", PartnerStatusRejected).Error
		}
		return nil
	})

	return partner, err
}

func AcceptPartner(db *gorm.DB, partnerId, userId uint) (Partner, error) {
	return respondInvitation(db, partnerId, userId, PartnerStatusActive)
}

func RejectPartner(db *gorm.DB, partnerId, userId uint) (Partner, error) {
	return respondInvitation(db, partnerId, userId, PartnerStatusRejected)
}

// DisablePartner memutus relasi aktif; boleh dilakukan salah satu dari pasangan.
func DisablePartner(db *gorm.DB, partnerId, userId uint) (Partner, error) {
	var partner Partner

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&partner, partnerId).Error; err != nil {
			return err
		}
		if !partner.hasMember(userId) {
			return gorm.ErrRecordNotFound
		}
		if partner.Status != PartnerStatusActive {
			return ErrPartnerNotActive
		}

		partner.Status = PartnerStatusDisabled
		return tx.Save(&partner).Error
	})

	return partner, err
}

// partnerViews adalah query dasar: relasi milik userId beserta profil lawannya.
func partnerViews(db *gorm.DB, userId uint) *gorm.DB {
	return db.Model(&Partner{}).
		Select("partners.id, partners.status, partners.invited_by, partners.created_at, partners.updated_at, "+
			"u.id AS partner_user_id, u.full_name, u.gender, u.unique_code").
		Joins("JOIN users u ON u.id = IF(partners.male_id = ?, partners.female_id, partners.male_id)", userId).
		Where("(partners.male_id = ? OR partners.female_id = ?)", userId, userId)
}

// FindActivePartner mengembalikan partner aktif milik user (gorm.ErrRecordNotFound jika tidak ada).
func FindActivePartner(db *gorm.DB, userId uint) (PartnerView, error) {
	var view PartnerView
	result := partnerViews(db, userId).Where("partners.status = ?", PartnerStatusActive).Limit(1).Scan(&view)
	if result.Error != nil {
		return view, result.Error
	}
	if result.RowsAffected == 0 {
		return view, gorm.ErrRecordNotFound
	}
	return view, nil
}

// GetIncomingPartnerRequests mengambil undangan pending yang ditujukan kepada user.
func GetIncomingPartnerRequests(db *gorm.DB, userId uint) ([]PartnerView, error) {
	views := []PartnerView{}
	err := partnerViews(db, userId).
		Where("partners.status = ? AND partners.invited_by <> ?", PartnerStatusPending, userId).
		Order("partners.created_at DESC").
		Scan(&views).Error
	return views, err
}

// GetSentPartnerRequests mengambil undangan yang dikirim user, terbaru dulu.
// Jika status tidak kosong, hasil difilter pada status tersebut.
func GetSentPartnerRequests(db *gorm.DB, userId uint, status string) ([]PartnerView, error) {
	views := []PartnerView{}
	query := partnerViews(db, userId).Where("partners.invited_by = ?", userId)
	if status != "" {
		query = query.Where("partners.status = ?", status)
	}
	err := query.Order("partners.created_at DESC").Scan(&views).Error
	return views, err
}

func IsValidPartnerStatus(status string) bool {
	switch status {
	case PartnerStatusPending, PartnerStatusActive, PartnerStatusDisabled, PartnerStatusRejected:
		return true
	}
	return false
}
