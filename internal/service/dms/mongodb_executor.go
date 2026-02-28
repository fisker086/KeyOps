package dms

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/crypto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBExecutor struct {
	instance *model.DBInstance
	crypto   *crypto.Crypto
	client   *mongo.Client
	password string
}

func NewMongoDBExecutor(instance *model.DBInstance, crypto *crypto.Crypto) (*MongoDBExecutor, error) {
	var password string
	var err error
	
	// 如果密码为空，直接使用空字符串（支持无密码的 MongoDB）
	if instance.Password == "" {
		password = ""
	} else {
		// 尝试解密密码
		password, err = crypto.Decrypt(instance.Password)
		if err != nil {
			return nil, fmt.Errorf("解密密码失败: %w", err)
		}
	}

	return &MongoDBExecutor{
		instance: instance,
		crypto:   crypto,
		password: password,
	}, nil
}

func (e *MongoDBExecutor) getClient(ctx context.Context) (*mongo.Client, error) {
	if e.client != nil {
		return e.client, nil
	}

	var uri string
	var credential *options.Credential

	if e.instance.ConnectionString != "" {
		// 如果提供了连接字符串，直接使用
		uri = e.instance.ConnectionString
	} else {
		// 构建基础 URI
		uri = fmt.Sprintf("mongodb://%s:%d/", e.instance.Host, e.instance.Port)

		// 如果有用户名和密码，使用 Credential 选项（更可靠）
		if e.instance.Username != "" && e.password != "" {
			authDB := e.instance.AuthDatabase
			if authDB == "" {
				authDB = "admin"
			}

			// 使用 Credential 选项显式配置认证机制
			// 不指定 AuthMechanism，让驱动自动协商（会优先尝试 SCRAM-SHA-256，失败时降级到 SCRAM-SHA-1）
			credential = &options.Credential{
				AuthSource: authDB,
				Username:   e.instance.Username,
				Password:   e.password,
			}
		}
	}

	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetConnectTimeout(10 * time.Second)
	clientOptions.SetServerSelectionTimeout(10 * time.Second)

	// 如果配置了认证信息，设置 Credential
	if credential != nil {
		clientOptions.SetAuth(*credential)
	}

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		// 提供更详细的错误信息
		errMsg := fmt.Sprintf("连接失败: %v", err)
		if strings.Contains(err.Error(), "AuthenticationFailed") || strings.Contains(err.Error(), "authentication") {
			authDB := e.instance.AuthDatabase
			if authDB == "" {
				authDB = "admin"
			}
			errMsg = fmt.Sprintf("认证失败: 请检查用户名、密码和认证数据库(authDatabase)是否正确。当前认证数据库: %s。错误详情: %v", authDB, err)
		}
		return nil, fmt.Errorf(errMsg)
	}

	e.client = client
	return client, nil
}

func (e *MongoDBExecutor) TestConnection(ctx context.Context) error {
	client, err := e.getClient(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = client.Ping(ctx, nil)
	if err != nil {
		// 提供更详细的错误信息
		if strings.Contains(err.Error(), "AuthenticationFailed") || strings.Contains(err.Error(), "authentication") {
			authDB := e.instance.AuthDatabase
			if authDB == "" {
				authDB = "admin"
			}
			return fmt.Errorf("认证失败: 请检查用户名、密码和认证数据库(authDatabase)是否正确。当前认证数据库: %s。错误详情: %v", authDB, err)
		}
		return fmt.Errorf("连接测试失败: %w", err)
	}
	return nil
}

func (e *MongoDBExecutor) GetDatabases(ctx context.Context) ([]string, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	databases, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("获取数据库列表失败: %w", err)
	}

	var result []string
	excludeDbs := map[string]bool{
		"admin":  true,
		"local":  true,
		"config": true,
	}
	for _, db := range databases {
		if !excludeDbs[db] {
			result = append(result, db)
		}
	}

	return result, nil
}

func (e *MongoDBExecutor) GetTables(ctx context.Context, databaseName string) ([]string, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db := client.Database(databaseName)
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("获取集合列表失败: %w", err)
	}

	return collections, nil
}

func (e *MongoDBExecutor) GetColumns(ctx context.Context, databaseName, tableName string) ([]ColumnInfo, error) {
	return []ColumnInfo{
		{
			Name:     "_id",
			Type:     "ObjectId",
			Nullable: false,
			Comment:  "MongoDB 主键",
		},
	}, nil
}

func (e *MongoDBExecutor) ExecuteQuery(ctx context.Context, databaseName, query string, limit int) (*QueryResult, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	// 检查是否是禁止的命令（如 show dbs, show databases 等）
	queryTrimmed := strings.TrimSpace(strings.ToLower(query))
	if strings.HasPrefix(queryTrimmed, "show") {
		return &QueryResult{
			Success: false,
			Error:   "不支持 show 命令，请使用界面左侧的数据库导航查看数据库和集合列表",
		}, nil
	}

	// 根据查询大小动态设置超时时间
	sqlSize := len(query)
	timeout := 30 * time.Second
	if sqlSize > 1024*1024 { // 大于1MB
		timeout = 300 * time.Second // 5分钟
	} else if sqlSize > 100*1024 { // 大于100KB
		timeout = 120 * time.Second // 2分钟
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	db := client.Database(databaseName)

	collectionName, filter, isFindOne, err := e.parseMongoQuery(query)
	if err != nil {
		return &QueryResult{Success: false, Error: fmt.Sprintf("解析查询失败: %v，查询: %s", err, query)}, nil
	}

	// 调试信息：检查数据库和集合名称
	if collectionName == "" {
		return &QueryResult{Success: false, Error: fmt.Sprintf("集合名称为空，查询: %s，数据库: %s", query, databaseName)}, nil
	}

	collection := db.Collection(collectionName)
	
	// 调试：检查集合是否存在（可选，不影响查询）
	collections, _ := db.ListCollectionNames(ctx, bson.M{"name": collectionName})
	if len(collections) == 0 {
		// 集合不存在，但仍然尝试查询（可能集合刚创建）
		// 不返回错误，让查询继续执行
	}

	var documents []map[string]interface{}
	if isFindOne {
		// findOne 查询
		var doc map[string]interface{}
		err := collection.FindOne(ctx, filter).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// 没有找到文档，返回空结果
				// 注意：这可能是因为数据库名称不匹配，或者集合中确实没有数据
				return &QueryResult{
					Success:     true,
					ResultCount: 0,
					Rows:        [][]interface{}{},
					Columns:     []string{},
					Error:       fmt.Sprintf("未找到文档（数据库: %s，集合: %s，过滤条件: %v）", databaseName, collectionName, filter),
				}, nil
			}
			return &QueryResult{Success: false, Error: fmt.Sprintf("查询执行失败: %v，数据库: %s，集合: %s，过滤条件: %v", err, databaseName, collectionName, filter)}, nil
		}
		documents = []map[string]interface{}{doc}
	} else {
		// find 查询
		findOptions := options.Find()
		if limit > 0 {
			limit64 := int64(limit)
			findOptions.SetLimit(limit64)
		}

		cursor, err := collection.Find(ctx, filter, findOptions)
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, &documents); err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}
	}

	if len(documents) == 0 {
		return &QueryResult{
			Success:     true,
			ResultCount: 0,
		}, nil
	}

	columnsMap := make(map[string]bool)
	for _, doc := range documents {
		for key := range doc {
			columnsMap[key] = true
		}
	}

	var columns []string
	for key := range columnsMap {
		columns = append(columns, key)
	}

	var rows [][]interface{}
	for _, doc := range documents {
		row := make([]interface{}, len(columns))
		for i, col := range columns {
			if val, ok := doc[col]; ok {
				row[i] = val
			} else {
				row[i] = nil
			}
		}
		rows = append(rows, row)
	}

	return &QueryResult{
		Success:      true,
		Columns:      columns,
		Rows:         rows,
		ResultCount:  len(documents),
		AffectedRows: int64(len(documents)),
	}, nil
}

// parseMongoObject 解析 JavaScript 对象字面量为 BSON
// 支持格式：{ name: "value", age: 28 } 或 { "name": "value", "age": 28 }
func parseMongoObject(objStr string) (bson.M, error) {
	objStr = strings.TrimSpace(objStr)
	if objStr == "" || objStr == "{}" {
		return bson.M{}, nil
	}

	// 确保有大括号
	if !strings.HasPrefix(objStr, "{") {
		objStr = "{" + objStr
	}
	if !strings.HasSuffix(objStr, "}") {
		objStr = objStr + "}"
	}

	// 尝试直接解析为 JSON（如果已经是 JSON 格式）
	var doc bson.M
	if err := bson.UnmarshalExtJSON([]byte(objStr), false, &doc); err == nil {
		return doc, nil
	}

	// 如果不是 JSON 格式，尝试转换为 JSON
	// 将 JavaScript 对象字面量转换为 JSON：
	// { name: "value" } -> { "name": "value" }
	jsonStr := convertJSObjectToJSON(objStr)
	if err := bson.UnmarshalExtJSON([]byte(jsonStr), false, &doc); err != nil {
		return nil, fmt.Errorf("无法解析对象: %v，原始输入: %s", err, objStr)
	}

	return doc, nil
}

// convertJSObjectToJSON 将 JavaScript 对象字面量转换为 JSON
// 为未加引号的键添加双引号
func convertJSObjectToJSON(jsStr string) string {
	// 使用正则表达式匹配未加引号的键名
	// 匹配模式：键名后跟冒号，键名不能以引号开头
	// Go 的 regexp 包支持 Unicode 属性类，使用 \p{L} 匹配所有字母（包括中文）
	// 或者使用 [\p{L}\p{N}_$] 来匹配字母、数字、下划线和美元符号
	re := regexp.MustCompile(`([{,]\s*)([a-zA-Z_$\p{L}][a-zA-Z0-9_$\p{L}\p{N}]*)\s*:`)
	
	result := re.ReplaceAllStringFunc(jsStr, func(match string) string {
		// 提取键名部分（去掉前面的 { 或 , 和空格）
		parts := re.FindStringSubmatch(match)
		if len(parts) >= 3 {
			prefix := parts[1] // { 或 ,
			key := parts[2]    // 键名
			return prefix + `"` + key + `":`
		}
		return match
	})
	
	return result
}

// isValidJSIdentifier 检查是否是有效的 JavaScript 标识符
func isValidJSIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// 第一个字符必须是字母、下划线、$ 或 Unicode 字符
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' && first != '$' && !(first >= 0x4E00 && first <= 0x9FFF) {
		return false
	}
	
	// 后续字符可以是字母、数字、下划线、$ 或 Unicode 字符
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' && !(r >= 0x4E00 && r <= 0x9FFF) {
			return false
		}
	}
	
	return true
}

// extractMongoArgs 提取 MongoDB 方法调用的参数
// 处理嵌套括号和多行格式
func extractMongoArgs(query string, methodName string) (collectionName string, args []string, err error) {
	// 找到方法调用的位置
	methodCall := fmt.Sprintf(".%s(", methodName)
	methodIdx := strings.Index(query, methodCall)
	if methodIdx == -1 {
		return "", nil, fmt.Errorf("无法找到方法调用 %s，查询: %s", methodCall, query)
	}
	
	// 提取集合名：从 db. 到方法调用之间的部分
	dbIdx := strings.LastIndex(query[:methodIdx], "db.")
	if dbIdx == -1 {
		return "", nil, fmt.Errorf("无法找到 db. 前缀")
	}
	collectionName = strings.TrimSpace(query[dbIdx+3 : methodIdx])
	if collectionName == "" {
		return "", nil, fmt.Errorf("集合名为空")
	}
	
	// 找到参数开始位置（方法调用后的第一个字符）
	argsStartIdx := methodIdx + len(methodCall)
	
	// 提取括号内的内容（处理嵌套括号）
	argsStr := extractNestedParens(query[argsStartIdx-1:]) // 包含左括号
	if argsStr == "" {
		return collectionName, []string{}, nil
	}
	
	// 分割参数（考虑嵌套括号和对象）
	args = splitMongoArgs(argsStr)
	
	return collectionName, args, nil
}

// extractNestedParens 提取嵌套括号的内容
func extractNestedParens(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	
	// 如果第一个字符不是 '('，说明已经去掉了外层括号，直接返回
	if s[0] != '(' {
		return s
	}
	
	depth := 0
	start := 0
	for i, r := range s {
		if r == '(' {
			if depth == 0 {
				start = i + 1
			}
			depth++
		} else if r == ')' {
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start:i])
			}
		}
	}
	
	// 如果没有找到匹配的右括号，返回去掉第一个左括号后的内容
	if start > 0 {
		return strings.TrimSpace(s[start:])
	}
	
	return strings.TrimSpace(s)
}

// findObjectEnd 找到对象的结束位置（匹配大括号）
func findObjectEnd(s string) int {
	depth := 0
	inString := false
	escapeNext := false
	
	for i, r := range s {
		if escapeNext {
			escapeNext = false
			continue
		}
		
		if r == '\\' {
			escapeNext = true
			continue
		}
		
		if r == '"' || r == '\'' {
			inString = !inString
			continue
		}
		
		if !inString {
			if r == '{' {
				depth++
			} else if r == '}' {
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
	}
	
	return -1
}

// splitMongoArgs 分割 MongoDB 参数（考虑嵌套括号和对象）
func splitMongoArgs(argsStr string) []string {
	args := []string{}
	current := strings.Builder{}
	depth := 0
	inString := false
	escapeNext := false
	
	for _, r := range argsStr {
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}
		
		if r == '\\' {
			escapeNext = true
			current.WriteRune(r)
			continue
		}
		
		if r == '"' || r == '\'' {
			inString = !inString
			current.WriteRune(r)
			continue
		}
		
		if !inString {
			if r == '{' || r == '[' || r == '(' {
				depth++
				current.WriteRune(r)
			} else if r == '}' || r == ']' || r == ')' {
				depth--
				current.WriteRune(r)
			} else if r == ',' && depth == 0 {
				arg := strings.TrimSpace(current.String())
				if arg != "" {
					args = append(args, arg)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		} else {
			current.WriteRune(r)
		}
	}
	
	arg := strings.TrimSpace(current.String())
	if arg != "" {
		args = append(args, arg)
	}
	
	return args
}

func (e *MongoDBExecutor) ExecuteUpdate(ctx context.Context, databaseName, query string) (*QueryResult, error) {
	client, err := e.getClient(ctx)
	if err != nil {
		return &QueryResult{Success: false, Error: err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	db := client.Database(databaseName)
	query = strings.TrimSpace(query)
	// 移除换行符和多余空格
	query = strings.ReplaceAll(query, "\n", " ")
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	// insertOne
	if strings.Contains(query, ".insertOne(") {
		collectionName, args, err := extractMongoArgs(query, "insertOne")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v，查询: %s", err, query)}, nil
		}
		
		if len(args) == 0 {
			// 尝试直接提取第一个对象
			methodCall := ".insertOne("
			methodIdx := strings.Index(query, methodCall)
			if methodIdx != -1 {
				argsStartIdx := methodIdx + len(methodCall)
				// 尝试找到第一个完整的对象
				firstObjEnd := findObjectEnd(query[argsStartIdx:])
				if firstObjEnd > 0 {
					docStr := strings.TrimSpace(query[argsStartIdx : argsStartIdx+firstObjEnd])
					args = []string{docStr}
				}
			}
		}
		
		if len(args) == 0 {
			return &QueryResult{Success: false, Error: fmt.Sprintf("insertOne 需要至少一个参数，提取到的参数数量: %d，查询: %s", len(args), query)}, nil
		}
		
		// 第一个参数是文档，第二个参数（如果存在）是选项，我们忽略选项
		docStr := args[0]
		doc, err := parseMongoObject(docStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析文档: %v，文档字符串: %s", err, docStr)}, nil
		}

		result, err := db.Collection(collectionName).InsertOne(ctx, doc)
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: 1,
			ResultCount:  1,
			Rows:         [][]interface{}{{result.InsertedID}},
			Columns:      []string{"_id"},
		}, nil
	}

	// insertMany
	if strings.Contains(query, ".insertMany(") {
		collectionName, args, err := extractMongoArgs(query, "insertMany")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v", err)}, nil
		}
		
		if len(args) == 0 {
			return &QueryResult{Success: false, Error: "insertMany 需要至少一个参数"}, nil
		}
		
		// 第一个参数是文档数组
		docsStr := args[0]
		// 尝试解析为数组
		var docs []interface{}
		if err := bson.UnmarshalExtJSON([]byte(docsStr), false, &docs); err != nil {
			// 如果失败，尝试转换 JavaScript 对象字面量
			jsonStr := convertJSObjectToJSON(docsStr)
			if err2 := bson.UnmarshalExtJSON([]byte(jsonStr), false, &docs); err2 != nil {
				return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析文档数组: %v", err)}, nil
			}
		}

		result, err := db.Collection(collectionName).InsertMany(ctx, docs)
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: int64(len(result.InsertedIDs)),
			ResultCount:  len(result.InsertedIDs),
		}, nil
	}

	// updateOne
	if strings.Contains(query, ".updateOne(") {
		collectionName, args, err := extractMongoArgs(query, "updateOne")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v", err)}, nil
		}
		
		if len(args) < 2 {
			return &QueryResult{Success: false, Error: "updateOne 需要两个参数：过滤条件和更新文档"}, nil
		}
		
		filterStr := args[0]
		updateStr := args[1]

		filter, err := parseMongoObject(filterStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析过滤条件: %v", err)}, nil
		}
		
		update, err := parseMongoObject(updateStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析更新文档: %v", err)}, nil
		}

		result, err := db.Collection(collectionName).UpdateOne(ctx, filter, bson.M{"$set": update})
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: result.ModifiedCount,
			ResultCount:  int(result.ModifiedCount),
		}, nil
	}

	// updateMany
	if strings.Contains(query, ".updateMany(") {
		collectionName, args, err := extractMongoArgs(query, "updateMany")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v", err)}, nil
		}
		
		if len(args) < 2 {
			return &QueryResult{Success: false, Error: "updateMany 需要两个参数：过滤条件和更新文档"}, nil
		}
		
		filterStr := args[0]
		updateStr := args[1]

		filter, err := parseMongoObject(filterStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析过滤条件: %v", err)}, nil
		}
		
		update, err := parseMongoObject(updateStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析更新文档: %v", err)}, nil
		}

		result, err := db.Collection(collectionName).UpdateMany(ctx, filter, bson.M{"$set": update})
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: result.ModifiedCount,
			ResultCount:  int(result.ModifiedCount),
		}, nil
	}

	// deleteOne
	if strings.Contains(query, ".deleteOne(") {
		collectionName, args, err := extractMongoArgs(query, "deleteOne")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v", err)}, nil
		}
		
		if len(args) == 0 {
			return &QueryResult{Success: false, Error: "deleteOne 需要至少一个参数"}, nil
		}
		
		filterStr := args[0]
		filter, err := parseMongoObject(filterStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析过滤条件: %v", err)}, nil
		}

		result, err := db.Collection(collectionName).DeleteOne(ctx, filter)
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: result.DeletedCount,
			ResultCount:  int(result.DeletedCount),
		}, nil
	}

	// deleteMany
	if strings.Contains(query, ".deleteMany(") {
		collectionName, args, err := extractMongoArgs(query, "deleteMany")
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("解析失败: %v", err)}, nil
		}
		
		if len(args) == 0 {
			return &QueryResult{Success: false, Error: "deleteMany 需要至少一个参数"}, nil
		}
		
		filterStr := args[0]
		filter, err := parseMongoObject(filterStr)
		if err != nil {
			return &QueryResult{Success: false, Error: fmt.Sprintf("无法解析过滤条件: %v", err)}, nil
		}

		result, err := db.Collection(collectionName).DeleteMany(ctx, filter)
		if err != nil {
			return &QueryResult{Success: false, Error: err.Error()}, nil
		}

		return &QueryResult{
			Success:      true,
			AffectedRows: result.DeletedCount,
			ResultCount:  int(result.DeletedCount),
		}, nil
	}

	return &QueryResult{
		Success:      false,
		Error:        "无法解析 MongoDB 更新操作，支持的格式: db.collection.insertOne({...}), db.collection.insertMany([{...}]), db.collection.updateOne({filter}, {update}), db.collection.updateMany({filter}, {update}), db.collection.deleteOne({filter}), db.collection.deleteMany({filter})",
		AffectedRows: 0,
	}, nil
}

func (e *MongoDBExecutor) parseMongoQuery(query string) (collectionName string, filter bson.M, isFindOne bool, err error) {
	query = strings.TrimSpace(query)
	// 移除换行符和多余空格
	query = strings.ReplaceAll(query, "\n", " ")
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	
	// 支持多种 MongoDB 查询格式：
	// 1. db.collection.find()
	// 2. db.collection.find({})
	// 3. db.collection.find({key: "value"})
	// 4. db.collection.findOne()
	// 5. db.collection.findOne({})
	
	// 匹配 db.collection.find(...) 或 db.collection.findOne(...)
	// 使用更宽松的匹配，支持集合名包含下划线等字符
	re := regexp.MustCompile(`db\.([a-zA-Z0-9_]+)\.(find|findOne)\(([^)]*)\)`)
	matches := re.FindStringSubmatch(query)
	if len(matches) < 3 {
		return "", nil, false, fmt.Errorf("无法解析 MongoDB 查询格式，期望格式: db.collection.find({...}) 或 db.collection.findOne({...})，实际查询: %s", query)
	}

	collectionName = matches[1]
	method := matches[2] // find 或 findOne
	isFindOne = method == "findOne"
	filterStr := ""
	if len(matches) > 3 {
		filterStr = strings.TrimSpace(matches[3])
	}

	// 解析过滤条件
	if filterStr == "" || filterStr == "{}" {
		filter = bson.M{}
	} else {
		// 尝试解析 JSON 格式的过滤条件
		if err := bson.UnmarshalExtJSON([]byte(filterStr), true, &filter); err != nil {
			// 如果严格模式失败，尝试宽松模式
			var temp bson.M
			if err2 := bson.UnmarshalExtJSON([]byte(filterStr), false, &temp); err2 != nil {
				return "", nil, false, fmt.Errorf("无法解析查询条件: %v，请使用 JSON 格式，例如: db.collection.find({\"key\": \"value\"})", err)
			}
			filter = temp
		}
	}

	return collectionName, filter, isFindOne, nil
}

func (e *MongoDBExecutor) Close() error {
	if e.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return e.client.Disconnect(ctx)
	}
	return nil
}
