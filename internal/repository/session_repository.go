package repository

import (
	"context"
	"errors"
	"hash/crc32"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

// SessionRepository 支持双引擎：MySQL / MongoDB（logins / recordings / commands 三集合）
type SessionRepository struct {
	useMongo        bool
	db              *gorm.DB
	mongoLogins     *mongo.Collection
	mongoRecordings *mongo.Collection
	mongoCommands   *mongo.Collection
}

// NewSessionRepository 创建仓库；logins/recordings/commands 全为 nil 时使用 MySQL。
func NewSessionRepository(db *gorm.DB, logins, recordings, commands *mongo.Collection) *SessionRepository {
	use := logins != nil && recordings != nil && commands != nil
	return &SessionRepository{
		useMongo:        use,
		db:              db,
		mongoLogins:     logins,
		mongoRecordings: recordings,
		mongoCommands:   commands,
	}
}

// UsesMongo 是否使用 Mongo 存储堡垒机会话相关数据
func (r *SessionRepository) UsesMongo() bool {
	return r.useMongo
}

// GetDB 返回数据库实例（用于 Service 层主机等仍走 SQL 的查询）
func (r *SessionRepository) GetDB() *gorm.DB {
	return r.db
}

// CreateLoginRecord 创建登录记录（双引擎）
func (r *SessionRepository) CreateLoginRecord(record *model.LoginRecord) error {
	if r.useMongo {
		doc := loginRecordToBSON(record)
		_, err := r.mongoLogins.InsertOne(context.Background(), doc)
		return err
	}
	return r.db.Create(record).Error
}

func loginRecordToBSON(record *model.LoginRecord) bson.M {
	doc := bson.M{
		"id":          record.ID,
		"user_id":     record.UserID,
		"host_id":     record.HostID,
		"host_name":   record.HostName,
		"host_ip":     record.HostIP,
		"username":    record.Username,
		"login_ip":    record.LoginIP,
		"user_agent":  record.UserAgent,
		"login_time":  record.LoginTime,
		"logout_time": record.LogoutTime,
		"duration":    record.Duration,
		"status":      record.Status,
		"session_id":  record.SessionID,
		"created_at":  record.CreatedAt,
	}
	return doc
}

// UpdateLogoutTime 更新登出时间（双引擎）
func (r *SessionRepository) UpdateLogoutTime(id string) error {
	if r.useMongo {
		now := time.Now()
		_, err := r.mongoLogins.UpdateMany(
			context.Background(),
			bson.M{"id": id},
			bson.M{"$set": bson.M{"logout_time": now, "status": "completed"}})
		return err
	}
	now := time.Now()
	return r.db.Model(&model.LoginRecord{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"logout_time": now,
			"status":      "completed",
		}).Error
}

// CalculateDuration 计算会话时长（双引擎）
func (r *SessionRepository) CalculateDuration(id string) error {
	if r.useMongo {
		ctx := context.Background()
		var lr model.LoginRecord
		err := r.mongoLogins.FindOne(ctx, bson.M{"id": id}).Decode(&lr)
		if err != nil {
			return err
		}
		if lr.LogoutTime == nil {
			return nil
		}
		duration := int(lr.LogoutTime.Sub(lr.LoginTime).Seconds())
		_, err = r.mongoLogins.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"duration": duration}})
		return err
	}
	var record model.LoginRecord
	if err := r.db.Where("id = ?", id).First(&record).Error; err != nil {
		return err
	}
	if record.LogoutTime != nil {
		duration := int(record.LogoutTime.Sub(record.LoginTime).Seconds())
		return r.db.Model(&model.LoginRecord{}).Where("id = ?", id).
			Update("duration", duration).Error
	}
	return nil
}

// FindLoginRecords 查询登录记录（双引擎）
func (r *SessionRepository) FindLoginRecords(page, pageSize int, hostID string) ([]model.LoginRecord, int64, error) {
	if r.useMongo {
		return r.findLoginRecordsMongo(page, pageSize, hostID)
	}
	var records []model.LoginRecord
	var total int64

	query := r.db.Model(&model.LoginRecord{}).
		Where("host_id IS NOT NULL AND host_id != ''")

	if hostID != "" {
		query = query.Where("host_id = ?", hostID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("login_time DESC").Find(&records).Error

	return records, total, err
}

func (r *SessionRepository) findLoginRecordsMongo(page, pageSize int, hostID string) ([]model.LoginRecord, int64, error) {
	ctx := context.Background()
	filter := bson.M{"host_id": bson.M{"$exists": true, "$ne": ""}}
	if hostID != "" {
		filter["host_id"] = hostID
	}

	total, err := r.mongoLogins.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "login_time", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cur, err := r.mongoLogins.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var records []model.LoginRecord
	if err := cur.All(ctx, &records); err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// FindLoginRecordsByUser 查询登录记录（支持按用户过滤，双引擎）
func (r *SessionRepository) FindLoginRecordsByUser(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
	if r.useMongo {
		return r.findLoginRecordsByUserMongo(page, pageSize, hostID, userID)
	}
	var total int64
	query := r.db.Model(&model.LoginRecord{}).
		Where("host_id IS NOT NULL AND host_id != ''")

	if hostID != "" {
		query = query.Where("host_id = ?", hostID)
	}

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var results []model.LoginRecordWithType
	joinQuery := r.db.Table("login_records lr").
		Select("lr.*, COALESCE(sr.connection_type, 'webshell') as connection_type").
		Joins("LEFT JOIN session_recordings sr ON lr.session_id = sr.session_id").
		Where("lr.host_id IS NOT NULL AND lr.host_id != ''")

	if hostID != "" {
		joinQuery = joinQuery.Where("lr.host_id = ?", hostID)
	}

	if userID != "" {
		joinQuery = joinQuery.Where("lr.user_id = ?", userID)
	}

	err := joinQuery.
		Order("lr.login_time DESC").
		Offset(offset).
		Limit(pageSize).
		Scan(&results).Error

	return results, total, err
}

func (r *SessionRepository) findLoginRecordsByUserMongo(page, pageSize int, hostID, userID string) ([]model.LoginRecordWithType, int64, error) {
	ctx := context.Background()
	match := bson.M{"host_id": bson.M{"$exists": true, "$ne": ""}}
	if hostID != "" {
		match["host_id"] = hostID
	}
	if userID != "" {
		match["user_id"] = userID
	}

	total, err := r.mongoLogins.CountDocuments(ctx, match)
	if err != nil {
		return nil, 0, err
	}

	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$sort", Value: bson.D{{Key: "login_time", Value: -1}}}},
		{{Key: "$skip", Value: int64((page - 1) * pageSize)}},
		{{Key: "$limit", Value: int64(pageSize)}},
		{{Key: "$lookup", Value: bson.M{
			"from":         r.mongoRecordings.Name(),
			"localField":   "session_id",
			"foreignField": "session_id",
			"as":           "sr",
		}}},
		{{Key: "$addFields", Value: bson.M{
			"connection_type": bson.M{
				"$ifNull": bson.A{
					bson.M{"$arrayElemAt": bson.A{"$sr.connection_type", 0}},
					"webshell",
				},
			},
		}}},
		{{Key: "$project", Value: bson.M{"sr": 0}}},
	}

	cur, err := r.mongoLogins.Aggregate(ctx, pipe)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var out []model.LoginRecordWithType
	for cur.Next(ctx) {
		var row model.LoginRecordWithType
		if err := cur.Decode(&row); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, cur.Err()
}

// GetRecentLogins 获取最近登录记录（双引擎）
func (r *SessionRepository) GetRecentLogins(limit int) ([]model.LoginRecord, error) {
	if r.useMongo {
		return r.getRecentLoginsMongo(limit)
	}
	var records []model.LoginRecord
	err := r.db.Where("host_id IS NOT NULL AND host_id != ''").
		Order("login_time DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func (r *SessionRepository) getRecentLoginsMongo(limit int) ([]model.LoginRecord, error) {
	ctx := context.Background()
	opts := options.Find().
		SetSort(bson.D{{Key: "login_time", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := r.mongoLogins.Find(ctx, bson.M{"host_id": bson.M{"$exists": true, "$ne": ""}}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var records []model.LoginRecord
	if err := cur.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// CountRecentLogins 统计最近登录次数（双引擎）
func (r *SessionRepository) CountRecentLogins(hours int) (int64, error) {
	if r.useMongo {
		return r.countRecentLoginsMongo(hours)
	}
	var count int64
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	err := r.db.Model(&model.LoginRecord{}).
		Where("login_time >= ?", cutoff).
		Count(&count).Error
	return count, err
}

func (r *SessionRepository) countRecentLoginsMongo(hours int) (int64, error) {
	ctx := context.Background()
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	filter := bson.M{
		"login_time": bson.M{"$gte": cutoff},
	}
	return r.mongoLogins.CountDocuments(ctx, filter)
}

// CreateSessionRecording 创建会话录制（双引擎）
func (r *SessionRepository) CreateSessionRecording(recording *model.SessionRecording) error {
	if r.useMongo {
		doc := sessionRecordingToBSON(recording)
		_, err := r.mongoRecordings.InsertOne(context.Background(), doc)
		return err
	}
	return r.db.Create(recording).Error
}

func sessionRecordingToBSON(recording *model.SessionRecording) bson.M {
	return bson.M{
		"id":              recording.ID,
		"session_id":      recording.SessionID,
		"connection_type": recording.ConnectionType,
		"proxy_id":        recording.ProxyID,
		"user_id":         recording.UserID,
		"host_id":         recording.HostID,
		"host_name":       recording.HostName,
		"host_ip":         recording.HostIP,
		"username":        recording.Username,
		"start_time":      recording.StartTime,
		"end_time":        recording.EndTime,
		"duration":        recording.Duration,
		"command_count":   recording.CommandCount,
		"status":          recording.Status,
		"recording":       recording.Recording,
		"terminal_cols":   recording.TerminalCols,
		"terminal_rows":   recording.TerminalRows,
		"created_at":      recording.CreatedAt,
		"updated_at":      recording.UpdatedAt,
	}
}

// CreateCommandRecord 创建命令记录（双引擎）
func (r *SessionRepository) CreateCommandRecord(record *model.CommandRecord) error {
	if r.useMongo {
		doc := bson.M{
			"proxy_id":    record.ProxyID,
			"session_id":  record.SessionID,
			"host_id":     record.HostID,
			"user_id":     record.UserID,
			"username":    record.Username,
			"host_ip":     record.HostIP,
			"command":     record.Command,
			"output":      record.Output,
			"exit_code":   record.ExitCode,
			"executed_at": record.ExecutedAt,
			"duration_ms": record.DurationMs,
			"created_at":  record.CreatedAt,
		}
		oid := primitive.NewObjectID()
		doc["_id"] = oid
		res, err := r.mongoCommands.InsertOne(context.Background(), doc)
		if err != nil {
			return err
		}
		if inserted, ok := res.InsertedID.(primitive.ObjectID); ok {
			record.ID = uint(crc32.ChecksumIEEE([]byte(inserted.Hex())))
		}
		return nil
	}
	return r.db.Create(record).Error
}

// FindSessionHistories 查询会话历史
func (r *SessionRepository) FindSessionHistories(page, pageSize int, hostID string) ([]model.LoginRecord, int64, error) {
	if r.useMongo {
		ctx := context.Background()
		filter := bson.M{}
		if hostID != "" {
			filter["host_id"] = hostID
		}
		total, err := r.mongoLogins.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		skip := int64((page - 1) * pageSize)
		limit := int64(pageSize)
		opts := options.Find().SetSort(bson.D{{Key: "login_time", Value: -1}}).SetSkip(skip).SetLimit(limit)

		cur, err := r.mongoLogins.Find(ctx, filter, opts)
		if err != nil {
			return nil, 0, err
		}
		defer cur.Close(ctx)

		var records []model.LoginRecord
		if err := cur.All(ctx, &records); err != nil {
			return nil, 0, err
		}
		return records, total, nil
	}

	var list []model.LoginRecord
	query := r.db.Model(&model.LoginRecord{}).Order("login_time desc").Offset((page - 1) * pageSize).Limit(pageSize)
	if hostID != "" {
		query = query.Where("host_id = ?", hostID)
	}
	err := query.Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	var total int64
	r.db.Model(&model.LoginRecord{}).Count(&total)
	return list, total, err
}

// GetRecentLoginsByUser 获取用户最近登录记录
func (r *SessionRepository) GetRecentLoginsByUser(limit int, userID string) ([]model.LoginRecord, error) {
	if r.useMongo {
		ctx := context.Background()
		filter := bson.M{"user_id": userID}
		opts := options.Find().SetSort(bson.D{{Key: "login_time", Value: -1}}).SetLimit(int64(limit))

		cur, err := r.mongoLogins.Find(ctx, filter, opts)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)

		var records []model.LoginRecord
		err = cur.All(ctx, &records)
		return records, err
	}

	var list []model.LoginRecord
	err := r.db.Where("user_id = ?", userID).Order("login_time desc").Limit(limit).Find(&list).Error
	return list, err
}

// CountRecentLoginsByUser 统计用户最近登录次数
func (r *SessionRepository) CountRecentLoginsByUser(hours int, userID string) (int64, error) {
	if r.useMongo {
		ctx := context.Background()
		since := time.Now().Add(-time.Duration(hours) * time.Hour)
		filter := bson.M{"user_id": userID, "login_time": bson.M{"$gte": since}}
		return r.mongoLogins.CountDocuments(ctx, filter)
	}

	var count int64
	err := r.db.Where("user_id = ? AND login_time >= ?", userID, time.Now().Add(-time.Duration(hours)*time.Hour)).
		Model(&model.LoginRecord{}).Count(&count).Error
	return count, err
}

// CountTodayLoginsByUser 统计用户今日登录次数
func (r *SessionRepository) CountTodayLoginsByUser(userID string) (int64, error) {
	if r.useMongo {
		ctx := context.Background()
		today := time.Now().Truncate(24 * time.Hour)
		filter := bson.M{"user_id": userID, "login_time": bson.M{"$gte": today}}
		return r.mongoLogins.CountDocuments(ctx, filter)
	}

	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Where("user_id = ? AND login_time >= ?", userID, today).
		Model(&model.LoginRecord{}).Count(&count).Error
	return count, err
}

// UpdateSessionRecording 更新会话录制内容（兼容：payload 为 JSON 字节写入 recording 字段）
func (r *SessionRepository) UpdateSessionRecording(sessionID string, recording []byte) error {
	if r.useMongo {
		ctx := context.Background()
		_, err := r.mongoRecordings.UpdateOne(
			ctx,
			bson.M{"session_id": sessionID},
			bson.M{"$set": bson.M{"recording": string(recording), "updated_at": time.Now()}},
		)
		return err
	}
	return r.db.Model(&model.SessionRecording{}).Where("session_id = ?", sessionID).
		Update("recording", string(recording)).Error
}

// FindSessionRecordingBySessionID 按 session_id 查询一条录制
func (r *SessionRepository) FindSessionRecordingBySessionID(sessionID string) (*model.SessionRecording, error) {
	if r.useMongo {
		ctx := context.Background()
		var rec model.SessionRecording
		err := r.mongoRecordings.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&rec)
		if err != nil {
			return nil, err
		}
		return &rec, nil
	}
	var rec model.SessionRecording
	if err := r.db.Where("session_id = ?", sessionID).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateSessionRecordingFields 按 session_id 更新多个字段（map 值为 SQL 侧支持的类型）
func (r *SessionRepository) UpdateSessionRecordingFields(sessionID string, fields map[string]interface{}) error {
	if r.useMongo {
		ctx := context.Background()
		set := bson.M{}
		for k, v := range fields {
			set[k] = v
		}
		set["updated_at"] = time.Now()
		_, err := r.mongoRecordings.UpdateOne(ctx, bson.M{"session_id": sessionID}, bson.M{"$set": set})
		return err
	}
	return r.db.Model(&model.SessionRecording{}).Where("session_id = ?", sessionID).Updates(fields).Error
}

// UpdateLoginBySessionID 按 session_id 更新登录记录
func (r *SessionRepository) UpdateLoginBySessionID(sessionID string, fields map[string]interface{}) error {
	if r.useMongo {
		ctx := context.Background()
		set := bson.M{}
		for k, v := range fields {
			set[k] = v
		}
		_, err := r.mongoLogins.UpdateOne(ctx, bson.M{"session_id": sessionID}, bson.M{"$set": set})
		return err
	}
	return r.db.Model(&model.LoginRecord{}).Where("session_id = ?", sessionID).Updates(fields).Error
}

// UpdateLoginStatusBySessionID 仅更新登录状态（双引擎）
func (r *SessionRepository) UpdateLoginStatusBySessionID(sessionID, status string) error {
	if r.useMongo {
		ctx := context.Background()
		_, err := r.mongoLogins.UpdateOne(ctx, bson.M{"session_id": sessionID}, bson.M{"$set": bson.M{"status": status}})
		return err
	}
	return r.db.Model(&model.LoginRecord{}).Where("session_id = ?", sessionID).Update("status", status).Error
}

// IncrementSessionCommandCount 会话命令计数 +1
func (r *SessionRepository) IncrementSessionCommandCount(sessionID string) error {
	return r.IncrementSessionCommandCountBy(sessionID, 1)
}

// IncrementSessionCommandCountBy 会话命令计数 +delta
func (r *SessionRepository) IncrementSessionCommandCountBy(sessionID string, delta int) error {
	if delta == 0 {
		return nil
	}
	if r.useMongo {
		ctx := context.Background()
		_, err := r.mongoRecordings.UpdateOne(ctx, bson.M{"session_id": sessionID}, bson.M{"$inc": bson.M{"command_count": delta}, "$set": bson.M{"updated_at": time.Now()}})
		return err
	}
	return r.db.Model(&model.SessionRecording{}).
		Where("session_id = ?", sessionID).
		Update("command_count", gorm.Expr("command_count + ?", delta)).Error
}

// FindSessionRecordings 分页查询会话录制列表
func (r *SessionRepository) FindSessionRecordings(page, pageSize int, search string) ([]model.SessionRecording, int64, error) {
	if r.useMongo {
		ctx := context.Background()
		filter := bson.M{}
		if search != "" {
			pat := ".*" + escapeRegex(search) + ".*"
			filter["$or"] = []bson.M{
				{"session_id": bson.M{"$regex": pat, "$options": "i"}},
				{"host_ip": bson.M{"$regex": pat, "$options": "i"}},
				{"username": bson.M{"$regex": pat, "$options": "i"}},
			}
		}
		total, err := r.mongoRecordings.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		opts := options.Find().
			SetSort(bson.D{{Key: "start_time", Value: -1}}).
			SetSkip(int64((page - 1) * pageSize)).
			SetLimit(int64(pageSize))
		cur, err := r.mongoRecordings.Find(ctx, filter, opts)
		if err != nil {
			return nil, 0, err
		}
		defer cur.Close(ctx)
		var list []model.SessionRecording
		if err := cur.All(ctx, &list); err != nil {
			return nil, 0, err
		}
		return list, total, nil
	}
	var list []model.SessionRecording
	var total int64
	q := r.db.Model(&model.SessionRecording{})
	if search != "" {
		q = q.Where("session_id LIKE ? OR host_ip LIKE ? OR username LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("start_time DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func escapeRegex(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ".", `\.`)
	s = strings.ReplaceAll(s, "*", `\*`)
	s = strings.ReplaceAll(s, "+", `\+`)
	s = strings.ReplaceAll(s, "?", `\?`)
	s = strings.ReplaceAll(s, "[", `\[`)
	s = strings.ReplaceAll(s, "]", `\]`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	s = strings.ReplaceAll(s, "^", `\^`)
	s = strings.ReplaceAll(s, "$", `\$`)
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

// FindCommandRecords 分页查询命令记录
func (r *SessionRepository) FindCommandRecords(page, pageSize int, search, hostFilter string) ([]model.CommandRecord, int64, error) {
	if r.useMongo {
		ctx := context.Background()
		filter := bson.M{}
		if search != "" {
			pat := ".*" + escapeRegex(search) + ".*"
			filter["$or"] = []bson.M{
				{"command": bson.M{"$regex": pat, "$options": "i"}},
				{"host_ip": bson.M{"$regex": pat, "$options": "i"}},
			}
		}
		if hostFilter != "" && hostFilter != "all" {
			filter["host_ip"] = hostFilter
		}
		total, err := r.mongoCommands.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		opts := options.Find().
			SetSort(bson.D{{Key: "executed_at", Value: -1}}).
			SetSkip(int64((page - 1) * pageSize)).
			SetLimit(int64(pageSize))
		cur, err := r.mongoCommands.Find(ctx, filter, opts)
		if err != nil {
			return nil, 0, err
		}
		defer cur.Close(ctx)
		var list []model.CommandRecord
		for cur.Next(ctx) {
			var raw bson.M
			if err := cur.Decode(&raw); err != nil {
				return nil, 0, err
			}
			list = append(list, commandRecordFromBSON(raw))
		}
		return list, total, cur.Err()
	}
	var records []model.CommandRecord
	var total int64
	query := r.db.Model(&model.CommandRecord{})
	if search != "" {
		query = query.Where("command LIKE ? OR host_ip LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if hostFilter != "" && hostFilter != "all" {
		query = query.Where("host_ip = ?", hostFilter)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("executed_at DESC").Find(&records).Error
	return records, total, err
}

func commandRecordFromBSON(m bson.M) model.CommandRecord {
	var c model.CommandRecord
	if v, ok := m["proxy_id"].(string); ok {
		c.ProxyID = v
	}
	if v, ok := m["session_id"].(string); ok {
		c.SessionID = v
	}
	if v, ok := m["host_id"].(string); ok {
		c.HostID = v
	}
	if v, ok := m["user_id"].(string); ok {
		c.UserID = v
	}
	if v, ok := m["username"].(string); ok {
		c.Username = v
	}
	if v, ok := m["host_ip"].(string); ok {
		c.HostIP = v
	}
	if v, ok := m["command"].(string); ok {
		c.Command = v
	}
	if v, ok := m["output"].(string); ok {
		c.Output = v
	}
	if v, ok := m["exit_code"].(int32); ok {
		c.ExitCode = int(v)
	} else if v, ok := m["exit_code"].(int64); ok {
		c.ExitCode = int(v)
	} else if v, ok := m["exit_code"].(float64); ok {
		c.ExitCode = int(v)
	}
	if v, ok := m["executed_at"].(primitive.DateTime); ok {
		c.ExecutedAt = v.Time()
	} else if v, ok := m["executed_at"].(time.Time); ok {
		c.ExecutedAt = v
	}
	if v, ok := m["duration_ms"].(int64); ok {
		c.DurationMs = v
	} else if v, ok := m["duration_ms"].(int32); ok {
		c.DurationMs = int64(v)
	}
	if v, ok := m["created_at"].(primitive.DateTime); ok {
		c.CreatedAt = v.Time()
	} else if v, ok := m["created_at"].(time.Time); ok {
		c.CreatedAt = v
	}
	if oid, ok := m["_id"].(primitive.ObjectID); ok {
		c.ID = uint(crc32.ChecksumIEEE([]byte(oid.Hex())))
	}
	return c
}

// FindCommandsBySession 查询某会话全部命令
func (r *SessionRepository) FindCommandsBySession(sessionID string) ([]model.CommandRecord, error) {
	if r.useMongo {
		ctx := context.Background()
		cur, err := r.mongoCommands.Find(ctx, bson.M{"session_id": sessionID}, options.Find().SetSort(bson.D{{Key: "executed_at", Value: 1}}))
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		var list []model.CommandRecord
		for cur.Next(ctx) {
			var raw bson.M
			if err := cur.Decode(&raw); err != nil {
				return nil, err
			}
			list = append(list, commandRecordFromBSON(raw))
		}
		return list, cur.Err()
	}
	var records []model.CommandRecord
	err := r.db.Where("session_id = ?", sessionID).Order("executed_at ASC").Find(&records).Error
	return records, err
}

// CountCommandsBySessions 统计多个 session_id 的命令条数
func (r *SessionRepository) CountCommandsBySessions(sessionIDs []string) (map[string]int, error) {
	out := make(map[string]int)
	if len(sessionIDs) == 0 {
		return out, nil
	}
	if r.useMongo {
		ctx := context.Background()
		cur, err := r.mongoCommands.Aggregate(ctx, mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.M{"session_id": bson.M{"$in": sessionIDs}}}},
			bson.D{{Key: "$group", Value: bson.M{"_id": "$session_id", "n": bson.M{"$sum": 1}}}},
		})
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var row struct {
				ID string `bson:"_id"`
				N  int    `bson:"n"`
			}
			if err := cur.Decode(&row); err != nil {
				return nil, err
			}
			out[row.ID] = row.N
		}
		return out, cur.Err()
	}
	type row struct {
		SessionID string `gorm:"column:session_id"`
		N         int    `gorm:"column:n"`
	}
	var rows []row
	err := r.db.Model(&model.CommandRecord{}).
		Select("session_id, COUNT(*) as n").
		Where("session_id IN ?", sessionIDs).
		Group("session_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.SessionID] = rw.N
	}
	return out, nil
}

// UpsertLoginRecord 按 session_id upsert 登录记录（Proxy 上报）
func (r *SessionRepository) UpsertLoginRecord(rec *model.LoginRecord) error {
	if !r.useMongo {
		var cur model.LoginRecord
		err := r.db.Where("session_id = ?", rec.SessionID).First(&cur).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(rec).Error
		}
		if err != nil {
			return err
		}
		rec.ID = cur.ID
		return r.db.Model(&cur).Updates(map[string]interface{}{
			"user_id":     rec.UserID,
			"host_id":     rec.HostID,
			"host_name":   rec.HostName,
			"host_ip":     rec.HostIP,
			"username":    rec.Username,
			"login_time":  rec.LoginTime,
			"logout_time": rec.LogoutTime,
			"duration":    rec.Duration,
			"status":      rec.Status,
		}).Error
	}
	ctx := context.Background()
	doc := loginRecordToBSON(rec)
	opts := options.Replace().SetUpsert(true)
	_, err := r.mongoLogins.ReplaceOne(ctx, bson.M{"session_id": rec.SessionID}, doc, opts)
	return err
}

// UpsertSessionRecording 按 session_id upsert 会话录制（Proxy 上报）
func (r *SessionRepository) UpsertSessionRecording(rec *model.SessionRecording) error {
	if !r.useMongo {
		var cur model.SessionRecording
		err := r.db.Where("session_id = ?", rec.SessionID).First(&cur).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(rec).Error
		}
		if err != nil {
			return err
		}
		rec.ID = cur.ID
		return r.db.Model(&cur).Updates(map[string]interface{}{
			"connection_type": rec.ConnectionType,
			"proxy_id":        rec.ProxyID,
			"user_id":         rec.UserID,
			"host_id":         rec.HostID,
			"host_name":       rec.HostName,
			"host_ip":         rec.HostIP,
			"username":        rec.Username,
			"status":          rec.Status,
			"start_time":      rec.StartTime,
			"end_time":        rec.EndTime,
			"duration":        rec.Duration,
			"recording":       rec.Recording,
			"terminal_cols":   rec.TerminalCols,
			"terminal_rows":   rec.TerminalRows,
			"command_count":   rec.CommandCount,
			"updated_at":      time.Now(),
		}).Error
	}
	ctx := context.Background()
	doc := sessionRecordingToBSON(rec)
	opts := options.Replace().SetUpsert(true)
	_, err := r.mongoRecordings.ReplaceOne(ctx, bson.M{"session_id": rec.SessionID}, doc, opts)
	return err
}

// CreateCommandRecordBatch 批量插入命令（Mongo）；MySQL 走 GORM CreateInBatches
func (r *SessionRepository) CreateCommandRecordBatch(records []model.CommandRecord) error {
	if len(records) == 0 {
		return nil
	}
	if r.useMongo {
		ctx := context.Background()
		docs := make([]interface{}, 0, len(records))
		for i := range records {
			oid := primitive.NewObjectID()
			doc := bson.M{
				"_id":         oid,
				"proxy_id":    records[i].ProxyID,
				"session_id":  records[i].SessionID,
				"host_id":     records[i].HostID,
				"user_id":     records[i].UserID,
				"username":    records[i].Username,
				"host_ip":     records[i].HostIP,
				"command":     records[i].Command,
				"output":      records[i].Output,
				"exit_code":   records[i].ExitCode,
				"executed_at": records[i].ExecutedAt,
				"duration_ms": records[i].DurationMs,
				"created_at":  records[i].CreatedAt,
			}
			docs = append(docs, doc)
			records[i].ID = uint(crc32.ChecksumIEEE([]byte(oid.Hex())))
		}
		_, err := r.mongoCommands.InsertMany(ctx, docs)
		return err
	}
	return r.db.CreateInBatches(records, 100).Error
}
