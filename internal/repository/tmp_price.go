package repository

import (
	"VMQ-api-go/internal/model"

	"gorm.io/gorm"
)

type TmpPriceRepository interface {
	Delete(oid string) error
}

type tmpPriceRepository struct {
	db *gorm.DB
}

func (r *tmpPriceRepository) Delete(oid string) error {
	return r.db.Delete(&model.TmpPrice{}, oid).Error
}
