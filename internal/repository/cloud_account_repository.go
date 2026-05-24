package repository

import (
	"log"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type CloudAccountRepository interface {
	GetByID(id uint) (*model.CloudAccount, error)
	List(cloudType string) ([]model.CloudAccount, error)
	Create(acc *model.CloudAccount) error
	Update(acc *model.CloudAccount) error
	Delete(id uint) error
	UpdateLastImport(id uint) error
}

type cloudAccountRepository struct {
	db *gorm.DB
}

func NewCloudAccountRepository(db *gorm.DB) CloudAccountRepository {
	return &cloudAccountRepository{db: db}
}

func (r *cloudAccountRepository) GetByID(id uint) (*model.CloudAccount, error) {
	var acc model.CloudAccount
	err := r.db.First(&acc, id).Error
	return &acc, err
}

func (r *cloudAccountRepository) List(cloudType string) ([]model.CloudAccount, error) {
	var list []model.CloudAccount
	q := r.db
	if cloudType != "" {
		q = q.Where("cloud_type = ?", cloudType)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *cloudAccountRepository) Create(acc *model.CloudAccount) error {
	log.Printf("[CloudAccountRepo] creating account: name=%s, cloudType=%s, id=%d", acc.Name, acc.CloudType, acc.ID)
	err := r.db.Create(acc).Error
	if err != nil {
		log.Printf("[CloudAccountRepo] create failed: %v", err)
	} else {
		log.Printf("[CloudAccountRepo] created successfully: id=%d", acc.ID)
	}
	return err
}

func (r *cloudAccountRepository) Update(acc *model.CloudAccount) error {
	log.Printf("[CloudAccountRepo] updating account: id=%d, status=%s", acc.ID, acc.Status)
	err := r.db.Save(acc).Error
	if err != nil {
		log.Printf("[CloudAccountRepo] update failed: %v", err)
	}
	return err
}

func (r *cloudAccountRepository) Delete(id uint) error {
	log.Printf("[CloudAccountRepo] deleting account: id=%d", id)
	err := r.db.Delete(&model.CloudAccount{}, id).Error
	if err != nil {
		log.Printf("[CloudAccountRepo] delete failed: %v", err)
	} else {
		log.Printf("[CloudAccountRepo] deleted successfully: id=%d", id)
	}
	return err
}

func (r *cloudAccountRepository) UpdateLastImport(id uint) error {
	now := time.Now().UTC()
	return r.db.Model(&model.CloudAccount{}).Where("id = ?", id).Update("last_import_at", &now).Error
}
