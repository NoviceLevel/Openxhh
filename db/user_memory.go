package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"openxhh/loger"
	"openxhh/pg"
	"openxhh/sqlite"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	botMemoryMoodKey         = "mood"
	userMemorySummaryMaxRune = 360
	userMemorySnippetMaxRune = 48
)

type UserMemory struct {
	UserID            int64
	UserName          string
	Summary           string
	InteractionCount  int
	LastInteractionAt int64
	UpdatedAt         int64
}

func migrateUserMemoryTables() {
	ctx := context.Background()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS user_memories (
			user_id BIGINT PRIMARY KEY,
			user_name TEXT DEFAULT '',
			summary TEXT DEFAULT '',
			interaction_count BIGINT DEFAULT 0,
			last_interaction_at BIGINT DEFAULT 0,
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建用户记忆表", zap.Error(err))
		}
		_, err = pg.Conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS bot_memory_state (
			key TEXT PRIMARY KEY,
			value TEXT DEFAULT '',
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建短期状态表", zap.Error(err))
		}
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`CREATE TABLE IF NOT EXISTS user_memories (
			user_id BIGINT PRIMARY KEY,
			user_name TEXT DEFAULT '',
			summary TEXT DEFAULT '',
			interaction_count BIGINT DEFAULT 0,
			last_interaction_at BIGINT DEFAULT 0,
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建用户记忆表", zap.Error(err))
		}
		_, err = sqlite.Db.Exec(`CREATE TABLE IF NOT EXISTS bot_memory_state (
			key TEXT PRIMARY KEY,
			value TEXT DEFAULT '',
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建短期状态表", zap.Error(err))
		}
	}
}

func GetUserMemory(userID int64) (UserMemory, bool) {
	if userID <= 0 || !messageStreamDatabaseReady() {
		return UserMemory{}, false
	}
	var mem UserMemory
	var err error
	if cfg.Type == "pg" {
		err = pg.Conn.QueryRow(context.Background(), `SELECT user_id,COALESCE(user_name,''),COALESCE(summary,''),COALESCE(interaction_count,0),COALESCE(last_interaction_at,0),COALESCE(updated_at,0)
			FROM user_memories WHERE user_id=$1`, userID).Scan(&mem.UserID, &mem.UserName, &mem.Summary, &mem.InteractionCount, &mem.LastInteractionAt, &mem.UpdatedAt)
	} else if cfg.Type == "sqlite" {
		err = sqlite.Db.QueryRow(`SELECT user_id,COALESCE(user_name,''),COALESCE(summary,''),COALESCE(interaction_count,0),COALESCE(last_interaction_at,0),COALESCE(updated_at,0)
			FROM user_memories WHERE user_id=?`, userID).Scan(&mem.UserID, &mem.UserName, &mem.Summary, &mem.InteractionCount, &mem.LastInteractionAt, &mem.UpdatedAt)
	} else {
		return UserMemory{}, false
	}
	if err != nil {
		if !isMemoryNoRows(err) {
			loger.Loger.Warn("[DB]无法读取用户记忆", zap.Error(err), zap.Int64("user_id", userID))
		}
		return UserMemory{}, false
	}
	mem.UserName = strings.TrimSpace(mem.UserName)
	mem.Summary = strings.TrimSpace(mem.Summary)
	return mem, mem.Summary != "" || mem.InteractionCount > 0
}

func SaveUserInteraction(userID int64, userName, question, reply string, at int64) bool {
	if userID <= 0 || !messageStreamDatabaseReady() {
		return false
	}
	if at <= 0 {
		at = time.Now().Unix()
	}
	existing, _ := GetUserMemory(userID)
	userName = strings.TrimSpace(userName)
	if userName == "" {
		userName = existing.UserName
	}
	summary := mergeUserMemorySummary(existing.Summary, userName, question, reply)
	interactionCount := existing.InteractionCount + 1
	updatedAt := time.Now().Unix()
	ctx := context.Background()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(ctx, `INSERT INTO user_memories (user_id,user_name,summary,interaction_count,last_interaction_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (user_id) DO UPDATE SET user_name=$2, summary=$3, interaction_count=$4, last_interaction_at=$5, updated_at=$6`,
			userID, userName, summary, interactionCount, at, updatedAt)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存用户记忆", zap.Error(err), zap.Int64("user_id", userID))
			return false
		}
		return true
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`INSERT INTO user_memories (user_id,user_name,summary,interaction_count,last_interaction_at,updated_at)
			VALUES (?,?,?,?,?,?)
			ON CONFLICT (user_id) DO UPDATE SET user_name=excluded.user_name, summary=excluded.summary, interaction_count=excluded.interaction_count, last_interaction_at=excluded.last_interaction_at, updated_at=excluded.updated_at`,
			userID, userName, summary, interactionCount, at, updatedAt)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存用户记忆", zap.Error(err), zap.Int64("user_id", userID))
			return false
		}
		return true
	}
	return false
}

func BotMood() string {
	value, ok := botMemoryState(botMemoryMoodKey)
	if !ok {
		return ""
	}
	return value
}

func SaveBotMood(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !messageStreamDatabaseReady() {
		return false
	}
	now := time.Now().Unix()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(context.Background(), `INSERT INTO bot_memory_state (key,value,updated_at)
			VALUES ($1,$2,$3)
			ON CONFLICT (key) DO UPDATE SET value=$2, updated_at=$3`, botMemoryMoodKey, value, now)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存短期情绪", zap.Error(err))
			return false
		}
		return true
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`INSERT INTO bot_memory_state (key,value,updated_at)
			VALUES (?,?,?)
			ON CONFLICT (key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, botMemoryMoodKey, value, now)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存短期情绪", zap.Error(err))
			return false
		}
		return true
	}
	return false
}

func botMemoryState(key string) (string, bool) {
	if strings.TrimSpace(key) == "" || !messageStreamDatabaseReady() {
		return "", false
	}
	var value string
	var err error
	if cfg.Type == "pg" {
		err = pg.Conn.QueryRow(context.Background(), "SELECT COALESCE(value,'') FROM bot_memory_state WHERE key=$1", key).Scan(&value)
	} else if cfg.Type == "sqlite" {
		err = sqlite.Db.QueryRow("SELECT COALESCE(value,'') FROM bot_memory_state WHERE key=?", key).Scan(&value)
	} else {
		return "", false
	}
	if err != nil {
		if !isMemoryNoRows(err) {
			loger.Loger.Warn("[DB]无法读取短期状态", zap.Error(err), zap.String("key", key))
		}
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

func isMemoryNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

func mergeUserMemorySummary(existing, userName, question, reply string) string {
	existing = compactMemoryText(existing)
	tone := userInteractionTone(question)
	snippet := "最近一次：" + trimMemorySnippet(question)
	if userName = strings.TrimSpace(userName); userName != "" {
		snippet = userName + " " + snippet
	}
	if reply = trimMemorySnippet(reply); reply != "" {
		snippet += "；我回：" + reply
	}
	parts := []string{}
	if existing != "" {
		parts = append(parts, existing)
	}
	if tone != "" && !strings.Contains(existing, tone) {
		parts = append(parts, "互动印象："+tone)
	}
	parts = append(parts, snippet)
	return limitMemoryRunes(strings.Join(parts, "；"), userMemorySummaryMaxRune)
}

func userInteractionTone(question string) string {
	text := strings.TrimSpace(question)
	switch {
	case containsMemoryAny(text, []string{"可爱", "喜欢", "好棒", "厉害", "夸"}):
		return "对方会夸人或表达好感，回应时可以得意但别太端着"
	case userMemoryTransferRoleTarget(text) != "":
		return "对方爱玩转接梗，回应时可以按目标角色口吻接一句，别复读命令"
	case containsMemoryAny(text, []string{"难受", "怎么办", "求助", "建议", "崩溃", "不舒服"}):
		return "对方可能在求助或吐槽，回应时先接住情绪再给短判断"
	case containsMemoryAny(text, []string{"有缘", "又遇到", "见到你"}):
		return "对方把相遇当熟人互动，回应时可以表现出记得和熟络"
	default:
		return "普通互动，先回应具体内容，再自然带一点人设"
	}
}

func userMemoryTransferRoleTarget(text string) string {
	text = strings.TrimSpace(CleanMemoryWhitespace(text))
	text = strings.Trim(text, " \t\r\n，,。.!！?？:：;；“”\"'「」『』（）()[]【】")
	for _, prefix := range []string{"请", "麻烦", "帮我", "帮忙"} {
		text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	if text == "" || !strings.HasPrefix(text, "转") {
		return ""
	}
	for _, prefix := range []string{"转发", "转账", "转让", "转载", "转帖", "转移", "转换", "转职", "转码", "转卖", "转圈", "转身", "转头", "转向"} {
		if strings.HasPrefix(text, prefix) {
			return ""
		}
	}
	target := strings.TrimSpace(strings.TrimPrefix(text, "转"))
	target = strings.TrimPrefix(target, "到")
	target = strings.TrimSpace(target)
	target = strings.Trim(target, " \t\r\n，,。.!！?？:：;；“”\"'「」『』（）()[]【】")
	for _, suffix := range []string{"一下下", "一下", "看看", "看下", "吧", "呗", "啊", "呀", "啦", "呢"} {
		target = strings.TrimSuffix(target, suffix)
	}
	target = strings.TrimSpace(target)
	if target == "" || strings.ContainsAny(target, " \t\r\n，,。.!！?？:：;；") || len([]rune(target)) > 16 {
		return ""
	}
	return target
}

func compactMemoryText(text string) string {
	text = strings.TrimSpace(CleanMemoryWhitespace(text))
	return limitMemoryRunes(text, userMemorySummaryMaxRune/2)
}

func trimMemorySnippet(text string) string {
	return limitMemoryRunes(CleanMemoryWhitespace(strings.TrimSpace(text)), userMemorySnippetMaxRune)
}

func CleanMemoryWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func limitMemoryRunes(text string, max int) string {
	if max <= 0 {
		return strings.TrimSpace(text)
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max]))
}

func containsMemoryAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
