package repository

import (
	"VMQ-api-go/internal/model"

	"gorm.io/gorm"
)

type TmpPriceRepository interface {
	DeleteWithOID(oid string) error
}

type tmpPriceRepository struct {
	db *gorm.DB
}

func NewTmpPriceRepository(db *gorm.DB) TmpPriceRepository {
	return &tmpPriceRepository{db: db}
}

func (r *tmpPriceRepository) DeleteWithOID(oid string) error {
	return r.db.Where("oid = ?", oid).Delete(&model.TmpPrice{}).Error
}
