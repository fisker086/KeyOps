package repository

import (
	"github.com/fisker086/keyops/internal/model"
	"gorm.io/gorm"
)

// DomainCertificateRepository 域名证书仓库
type DomainCertificateRepository struct {
	db *gorm.DB
}

func NewDomainCertificateRepository(db *gorm.DB) *DomainCertificateRepository {
	return &DomainCertificateRepository{db: db}
}

func (r *DomainCertificateRepository) Create(cert *model.DomainCertificate) error {
	// 使用 Select 明确指定要创建的字段，排除 alert_channel_ids（已废弃，使用模板中的渠道配置）
	return r.db.Model(cert).
		Select("domain", "port", "ssl_certificate", "ssl_certificate_key", "start_time", "expire_time", "expire_days", "is_monitor", "auto_update", "connect_status", "alert_days", "alert_template_id", "last_alert_time", "comment").
		Create(cert).Error
}

func (r *DomainCertificateRepository) Update(cert *model.DomainCertificate) error {
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
			"alert_template_id":  cert.AlertTemplateID,
			"last_alert_time":    cert.LastAlertTime,
			"comment":             cert.Comment,
		}).Error
}

func (r *DomainCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.DomainCertificate{}, "id = ?", id).Error
}

func (r *DomainCertificateRepository) FindByID(id uint) (*model.DomainCertificate, error) {
	var cert model.DomainCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *DomainCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.DomainCertificate, err error) {
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

// SSLCertificateRepository SSL证书仓库
type SSLCertificateRepository struct {
	db *gorm.DB
}

func NewSSLCertificateRepository(db *gorm.DB) *SSLCertificateRepository {
	return &SSLCertificateRepository{db: db}
}

func (r *SSLCertificateRepository) Create(cert *model.SSLCertificate) error {
	return r.db.Create(cert).Error
}

func (r *SSLCertificateRepository) Update(cert *model.SSLCertificate) error {
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

func (r *SSLCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.SSLCertificate{}, "id = ?", id).Error
}

func (r *SSLCertificateRepository) FindByID(id uint) (*model.SSLCertificate, error) {
	var cert model.SSLCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *SSLCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.SSLCertificate, err error) {
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

// HostedCertificateRepository 托管证书仓库
type HostedCertificateRepository struct {
	db *gorm.DB
}

func NewHostedCertificateRepository(db *gorm.DB) *HostedCertificateRepository {
	return &HostedCertificateRepository{db: db}
}

func (r *HostedCertificateRepository) Create(cert *model.HostedCertificate) error {
	return r.db.Create(cert).Error
}

func (r *HostedCertificateRepository) Update(cert *model.HostedCertificate) error {
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

func (r *HostedCertificateRepository) Delete(id uint) error {
	return r.db.Delete(&model.HostedCertificate{}, "id = ?", id).Error
}

func (r *HostedCertificateRepository) FindByID(id uint) (*model.HostedCertificate, error) {
	var cert model.HostedCertificate
	err := r.db.Where("id = ?", id).First(&cert).Error
	return &cert, err
}

func (r *HostedCertificateRepository) List(page, pageSize int, keyword string) (total int64, certs []model.HostedCertificate, err error) {
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
