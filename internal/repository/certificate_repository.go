package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

type DomainCertificateRepository interface {
	Create(cert *model.DomainCertificate) error
	Update(cert *model.DomainCertificate) error
	Delete(id uint) error
	FindByID(id uint) (*model.DomainCertificate, error)
	List(page, pageSize int, keyword string) (total int64, certs []model.DomainCertificate, err error)
}

type domainCertificateRepository struct {
	db *gorm.DB
}

func NewDomainCertificateRepository(db *gorm.DB) DomainCertificateRepository {
	return &domainCertificateRepository{db: db}
}

func (r *domainCertificateRepository) Create(cert *model.DomainCertificate) error {
	return r.db.Model(cert).
		Select("domain", "port", "ssl_certificate", "ssl_certificate_key", "start_time", "expire_time", "expire_days", "is_monitor", "auto_update", "connect_status", "alert_days", "alert_template_id", "last_alert_time", "comment").
		Create(cert).Error
}

func (r *domainCertificateRepository) Update(cert *model.DomainCertificate) error {
	return r.db.Model(cert).
		Select("domain", "port", "ssl_certificate", "ssl_certificate_key", "start_time", "expire_time", "expire_days", "is_monitor", "auto_update", "connect_status", "alert_days", "alert_template_id", "last_alert_time", "comment").
		Updates(map[string]interface{}{
			"domain":              cert.Domain,
			"port":                cert.Port,
			"ssl_certificate":     cert.SSLCertificate,
			"ssl_certificate_key": cert.SSLCertificateKey,
			"start_time":          cert.StartTime,
			"expire_time":         cert.ExpireTime,
			"expire_days":         cert.ExpireDays,
			"is_monitor":          cert.IsMonitor,
			"auto_update":         cert.AutoUpdate,
			"connect_status":      cert.ConnectStatus,
			"alert_days":          cert.AlertDays,
			"alert_template_id":   cert.AlertTemplateID,
			"last_alert_time":     cert.LastAlertTime,
			"comment":             cert.Comment,
		}).Error
}

func (r *domainCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.DomainCertificate{}, "id = ?", id).Error
}

func (r *domainCertificateRepository) FindByID(id uint) (*model.DomainCertificate, error) {
	var cert model.DomainCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *domainCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.DomainCertificate, err error) {
	query := r.db.Model(&model.DomainCertificate{})

	if keyword != "" {
		query = query.Where("domain LIKE ?", "%"+keyword+"%")
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.DomainCertificate{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("expire_time ASC, created_at DESC").Find(&certs).Error
	return
}

type SSLCertificateRepository interface {
	Create(cert *model.SSLCertificate) error
	Update(cert *model.SSLCertificate) error
	Delete(id uint) error
	FindByID(id uint) (*model.SSLCertificate, error)
	List(page, pageSize int, keyword string) (total int64, certs []model.SSLCertificate, err error)
}

type sslCertificateRepository struct {
	db *gorm.DB
}

func NewSSLCertificateRepository(db *gorm.DB) SSLCertificateRepository {
	return &sslCertificateRepository{db: db}
}

func (r *sslCertificateRepository) Create(cert *model.SSLCertificate) error {
	return r.db.Create(cert).Error
}

func (r *sslCertificateRepository) Update(cert *model.SSLCertificate) error {
	return r.db.Model(cert).
		Select("domain", "ssl_certificate", "ssl_certificate_key", "start_time", "expire_time", "comment").
		Updates(map[string]interface{}{
			"domain":              cert.Domain,
			"ssl_certificate":     cert.SSLCertificate,
			"ssl_certificate_key": cert.SSLCertificateKey,
			"start_time":          cert.StartTime,
			"expire_time":         cert.ExpireTime,
			"comment":             cert.Comment,
		}).Error
}

func (r *sslCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.SSLCertificate{}, "id = ?", id).Error
}

func (r *sslCertificateRepository) FindByID(id uint) (*model.SSLCertificate, error) {
	var cert model.SSLCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *sslCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.SSLCertificate, err error) {
	query := r.db.Model(&model.SSLCertificate{})

	if keyword != "" {
		query = query.Where("domain LIKE ?", "%"+keyword+"%")
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.SSLCertificate{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("expire_time ASC, created_at DESC").Find(&certs).Error
	return
}

type HostedCertificateRepository interface {
	Create(cert *model.HostedCertificate) error
	Update(cert *model.HostedCertificate) error
	Delete(id uint) error
	FindByID(id uint) (*model.HostedCertificate, error)
	List(page, pageSize int, keyword string) (total int64, certs []model.HostedCertificate, err error)
}

type hostedCertificateRepository struct {
	db *gorm.DB
}

func NewHostedCertificateRepository(db *gorm.DB) HostedCertificateRepository {
	return &hostedCertificateRepository{db: db}
}

func (r *hostedCertificateRepository) Create(cert *model.HostedCertificate) error {
	return r.db.Create(cert).Error
}

func (r *hostedCertificateRepository) Update(cert *model.HostedCertificate) error {
	return r.db.Model(cert).
		Select("domain", "ssl_certificate", "ssl_certificate_key", "start_time", "expire_time", "comment").
		Updates(map[string]interface{}{
			"domain":              cert.Domain,
			"ssl_certificate":     cert.SSLCertificate,
			"ssl_certificate_key": cert.SSLCertificateKey,
			"start_time":          cert.StartTime,
			"expire_time":         cert.ExpireTime,
			"comment":             cert.Comment,
		}).Error
}

func (r *hostedCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.HostedCertificate{}, "id = ?", id).Error
}

func (r *hostedCertificateRepository) FindByID(id uint) (*model.HostedCertificate, error) {
	var cert model.HostedCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *hostedCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.HostedCertificate, err error) {
	query := r.db.Model(&model.HostedCertificate{})

	if keyword != "" {
		query = query.Where("domain LIKE ?", "%"+keyword+"%")
	}

	if err = query.Count(&total).Error; err != nil {
		return
	}

	if total == 0 {
		return 0, []model.HostedCertificate{}, nil
	}

	if pageSize > 0 && page > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err = query.Order("expire_time ASC, created_at DESC").Find(&certs).Error
	return
}
