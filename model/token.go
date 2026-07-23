package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	MaxTokenNameLength      = 50
	tokenNameMigrationBatch = 200
	tokenNameUniqueIndex    = "idx_tokens_user_name_fingerprint"
)

var (
	ErrTokenNameEmpty   = errors.New("token name cannot be empty")
	ErrTokenNameTooLong = errors.New("token name is too long")
)

type Token struct {
	Id                 int            `json:"id"`
	UserId             int            `json:"user_id" gorm:"index"`
	Key                string         `json:"key" gorm:"type:varchar(128);uniqueIndex"`
	Status             int            `json:"status" gorm:"default:1"`
	Name               string         `json:"name" gorm:"index" `
	NameFingerprint    string         `json:"-" gorm:"type:char(64);not null;default:''"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	AccessedTime       int64          `json:"accessed_time" gorm:"bigint"`
	ExpiredTime        int64          `json:"expired_time" gorm:"bigint;default:-1"` // -1 means never expired
	RemainQuota        int            `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota     bool           `json:"unlimited_quota"`
	ModelLimitsEnabled bool           `json:"model_limits_enabled"`
	ModelLimits        string         `json:"model_limits" gorm:"type:text"`
	AllowIps           *string        `json:"allow_ips" gorm:"default:''"`
	UsedQuota          int            `json:"used_quota" gorm:"default:0"` // used quota
	Group              string         `json:"group" gorm:"default:''"`
	CrossGroupRetry    bool           `json:"cross_group_retry"` // 跨分组重试，仅auto分组有效
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

type tokenNameUniqueIndexModel struct {
	UserId          int    `gorm:"column:user_id;uniqueIndex:idx_tokens_user_name_fingerprint,priority:1"`
	NameFingerprint string `gorm:"column:name_fingerprint;uniqueIndex:idx_tokens_user_name_fingerprint,priority:2"`
}

func (tokenNameUniqueIndexModel) TableName() string {
	return "tokens"
}

func NormalizeTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrTokenNameEmpty
	}
	if utf8.RuneCountInString(name) > MaxTokenNameLength {
		return "", ErrTokenNameTooLong
	}
	return name, nil
}

func tokenNameFingerprint(name string) string {
	digest := sha256.Sum256([]byte("active\x00" + name))
	return fmt.Sprintf("%x", digest)
}

func deletedTokenNameFingerprint(id int) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("deleted\x00%d", id)))
	return fmt.Sprintf("%x", digest)
}

func truncateTokenName(name string, maxLength int) string {
	runes := []rune(name)
	if len(runes) <= maxLength {
		return name
	}
	return string(runes[:maxLength])
}

func normalizeLegacyTokenName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "API Key"
	}
	return strings.TrimSpace(truncateTokenName(name, MaxTokenNameLength))
}

func (token *Token) prepareName() error {
	name, err := NormalizeTokenName(token.Name)
	if err != nil {
		return err
	}
	token.Name = name
	token.NameFingerprint = tokenNameFingerprint(name)
	return nil
}

func (token *Token) BeforeCreate(_ *gorm.DB) error {
	return token.prepareName()
}

func (token *Token) BeforeDelete(tx *gorm.DB) error {
	if token.Id <= 0 {
		return nil
	}
	fingerprint := deletedTokenNameFingerprint(token.Id)
	if err := tx.Model(&Token{}).Where("id = ?", token.Id).UpdateColumn("name_fingerprint", fingerprint).Error; err != nil {
		return err
	}
	token.NameFingerprint = fingerprint
	return nil
}

func (token *Token) Clean() {
	token.Key = ""
}

func MaskTokenKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	if len(key) <= 8 {
		return key[:2] + "****" + key[len(key)-2:]
	}
	return key[:4] + "**********" + key[len(key)-4:]
}

func (token *Token) GetFullKey() string {
	return token.Key
}

func (token *Token) GetMaskedKey() string {
	return MaskTokenKey(token.Key)
}

func (token *Token) GetIpLimits() []string {
	// delete empty spaces
	//split with \n
	ipLimits := make([]string, 0)
	if token.AllowIps == nil {
		return ipLimits
	}
	cleanIps := strings.ReplaceAll(*token.AllowIps, " ", "")
	if cleanIps == "" {
		return ipLimits
	}
	ips := strings.Split(cleanIps, "\n")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		ip = strings.ReplaceAll(ip, ",", "")
		if ip != "" {
			ipLimits = append(ipLimits, ip)
		}
	}
	return ipLimits
}

func GetAllUserTokens(userId int, startIdx int, num int) ([]*Token, error) {
	var tokens []*Token
	var err error
	err = DB.Where("user_id = ?", userId).Order("id desc").Limit(num).Offset(startIdx).Find(&tokens).Error
	return tokens, err
}

// sanitizeLikePattern 校验并清洗用户输入的 LIKE 搜索模式。
// 规则：
//  1. 转义 ! 和 _（使用 ! 作为 ESCAPE 字符，兼容 MySQL/PostgreSQL/SQLite）
//  2. 连续的 % 合并为单个 %
//  3. 最多允许 2 个 %
//  4. 含 % 时（模糊搜索），去掉 % 后关键词长度必须 >= 2
//  5. 不含 % 时按精确匹配
func sanitizeLikePattern(input string) (string, error) {
	// 1. 先转义 ESCAPE 字符 ! 自身，再转义 _
	//    使用 ! 而非 \ 作为 ESCAPE 字符，避免 MySQL 中反斜杠的字符串转义问题
	input = strings.ReplaceAll(input, "!", "!!")
	input = strings.ReplaceAll(input, `_`, `!_`)

	if err := validateLikePattern(input); err != nil {
		return "", err
	}

	// 5. 无 % 时，精确全匹配
	return input, nil
}

func validateLikePattern(input string) error {
	// 1. 连续的 % 直接拒绝
	if strings.Contains(input, "%%") {
		return errors.New("搜索模式中不允许包含连续的 % 通配符")
	}

	// 2. 统计 % 数量，不得超过 2
	count := strings.Count(input, "%")
	if count > 2 {
		return errors.New("搜索模式中最多允许包含 2 个 % 通配符")
	}

	// 3. 含 % 时，去掉 % 后关键词长度必须 >= 2
	if count > 0 {
		stripped := strings.ReplaceAll(input, "%", "")
		if len(stripped) < 2 {
			return errors.New("使用模糊搜索时，关键词长度至少为 2 个字符")
		}
	}

	return nil
}

const searchHardLimit = 100

func SearchUserTokens(userId int, keyword string, token string, offset int, limit int) (tokens []*Token, total int64, err error) {
	// model 层强制截断
	if limit <= 0 || limit > searchHardLimit {
		limit = searchHardLimit
	}
	if offset < 0 {
		offset = 0
	}

	if token != "" {
		token = strings.TrimPrefix(token, "sk-")
	}

	// 超量用户（令牌数超过上限）只允许精确搜索，禁止模糊搜索
	maxTokens := operation_setting.GetMaxUserTokens()
	hasFuzzy := strings.Contains(keyword, "%") || strings.Contains(token, "%")
	if hasFuzzy {
		count, err := CountUserTokens(userId)
		if err != nil {
			common.SysLog("failed to count user tokens: " + err.Error())
			return nil, 0, errors.New("获取令牌数量失败")
		}
		if int(count) > maxTokens {
			return nil, 0, errors.New("令牌数量超过上限，仅允许精确搜索，请勿使用 % 通配符")
		}
	}

	baseQuery := DB.Model(&Token{}).Where("user_id = ?", userId)

	// 非空才加 LIKE 条件，空则跳过（不过滤该字段）
	if keyword != "" {
		keywordPattern, err := sanitizeLikePattern(keyword)
		if err != nil {
			return nil, 0, err
		}
		baseQuery = baseQuery.Where("name LIKE ? ESCAPE '!'", keywordPattern)
	}
	if token != "" {
		tokenPattern, err := sanitizeLikePattern(token)
		if err != nil {
			return nil, 0, err
		}
		baseQuery = baseQuery.Where(commonKeyCol+" LIKE ? ESCAPE '!'", tokenPattern)
	}

	// 先查匹配总数（用于分页，受 maxTokens 上限保护，避免全表 COUNT）
	err = baseQuery.Limit(maxTokens).Count(&total).Error
	if err != nil {
		common.SysError("failed to count search tokens: " + err.Error())
		return nil, 0, errors.New("搜索令牌失败")
	}

	// 再分页查数据
	err = baseQuery.Order("id desc").Offset(offset).Limit(limit).Find(&tokens).Error
	if err != nil {
		common.SysError("failed to search tokens: " + err.Error())
		return nil, 0, errors.New("搜索令牌失败")
	}
	return tokens, total, nil
}

func ValidateUserToken(key string) (token *Token, err error) {
	if key == "" {
		return nil, ErrTokenNotProvided
	}
	token, err = GetTokenByKey(key, false)
	if err == nil {
		if token.Status == common.TokenStatusExhausted ||
			token.Status == common.TokenStatusExpired ||
			token.Status != common.TokenStatusEnabled {
			return token, ErrTokenInvalid
		}
		if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
			if !common.RedisEnabled {
				token.Status = common.TokenStatusExpired
				err := token.SelectUpdate()
				if err != nil {
					common.SysLog("failed to update token status" + err.Error())
				}
			}
			return token, ErrTokenInvalid
		}
		if !token.UnlimitedQuota && token.RemainQuota <= 0 {
			if !common.RedisEnabled {
				token.Status = common.TokenStatusExhausted
				err := token.SelectUpdate()
				if err != nil {
					common.SysLog("failed to update token status" + err.Error())
				}
			}
			return token, ErrTokenInvalid
		}
		return token, nil
	}
	common.SysLog("ValidateUserToken: failed to get token: " + err.Error())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenInvalid
	}
	return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
}

func GetTokenByIds(id int, userId int) (*Token, error) {
	if id == 0 || userId == 0 {
		return nil, errors.New("id 或 userId 为空！")
	}
	token := Token{Id: id, UserId: userId}
	var err error = nil
	err = DB.First(&token, "id = ? and user_id = ?", id, userId).Error
	return &token, err
}

func IsTokenNameTaken(userId int, name string, excludeTokenId int) (bool, error) {
	name, err := NormalizeTokenName(name)
	if err != nil {
		return false, err
	}
	query := DB.Model(&Token{}).Where("user_id = ? AND name_fingerprint = ?", userId, tokenNameFingerprint(name))
	if excludeTokenId > 0 {
		query = query.Where("id <> ?", excludeTokenId)
	}
	var count int64
	err = query.Count(&count).Error
	return count > 0, err
}

func migrateDuplicateTokenNames(db *gorm.DB) error {
	if db.Migrator().HasIndex(&Token{}, tokenNameUniqueIndex) {
		return nil
	}

	lastUserId := -1
	for {
		var userIds []int
		if err := db.Unscoped().Model(&Token{}).
			Distinct("user_id").
			Where("user_id > ?", lastUserId).
			Order("user_id asc").
			Limit(tokenNameMigrationBatch).
			Pluck("user_id", &userIds).Error; err != nil {
			return fmt.Errorf("failed to load token owners: %w", err)
		}
		if len(userIds) == 0 {
			break
		}

		for _, userId := range userIds {
			if err := db.Transaction(func(tx *gorm.DB) error {
				var tokens []Token
				if err := lockForUpdate(tx.Unscoped()).Where("user_id = ?", userId).Order("id asc").Find(&tokens).Error; err != nil {
					return fmt.Errorf("failed to load token names for user %d: %w", userId, err)
				}

				reservedNames := make(map[string]struct{})
				for i := range tokens {
					if tokens[i].DeletedAt.Valid {
						continue
					}
					reservedNames[normalizeLegacyTokenName(tokens[i].Name)] = struct{}{}
				}

				seenNames := make(map[string]struct{})
				for i := range tokens {
					token := &tokens[i]
					if token.DeletedAt.Valid {
						fingerprint := deletedTokenNameFingerprint(token.Id)
						if token.NameFingerprint != fingerprint {
							if err := tx.Unscoped().Model(&Token{}).Where("id = ?", token.Id).
								Update("name_fingerprint", fingerprint).Error; err != nil {
								return fmt.Errorf("failed to migrate deleted token %d: %w", token.Id, err)
							}
						}
						continue
					}

					name := normalizeLegacyTokenName(token.Name)
					targetName := name
					if _, exists := seenNames[name]; exists {
						for suffix := 2; ; suffix++ {
							suffixText := fmt.Sprintf(" (%d)", suffix)
							baseLength := MaxTokenNameLength - utf8.RuneCountInString(suffixText)
							candidate := strings.TrimSpace(truncateTokenName(name, baseLength)) + suffixText
							if _, reserved := reservedNames[candidate]; reserved {
								continue
							}
							targetName = candidate
							reservedNames[candidate] = struct{}{}
							break
						}
					}
					seenNames[name] = struct{}{}

					fingerprint := tokenNameFingerprint(targetName)
					if token.Name == targetName && token.NameFingerprint == fingerprint {
						continue
					}
					if err := tx.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]any{
						"name":             targetName,
						"name_fingerprint": fingerprint,
					}).Error; err != nil {
						return fmt.Errorf("failed to normalize token %d name: %w", token.Id, err)
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		lastUserId = userIds[len(userIds)-1]
	}

	var missingFingerprintCount int64
	if err := db.Unscoped().Model(&Token{}).Where("name_fingerprint = '' OR name_fingerprint IS NULL").
		Count(&missingFingerprintCount).Error; err != nil {
		return fmt.Errorf("failed to verify token name fingerprints: %w", err)
	}
	if missingFingerprintCount != 0 {
		return fmt.Errorf("token name migration left %d rows without a fingerprint", missingFingerprintCount)
	}

	if err := db.Migrator().CreateIndex(&tokenNameUniqueIndexModel{}, tokenNameUniqueIndex); err != nil {
		return fmt.Errorf("failed to create token name unique index: %w", err)
	}
	return nil
}

func GetTokenById(id int) (*Token, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	token := Token{Id: id}
	var err error = nil
	err = DB.First(&token, "id = ?", id).Error
	if shouldUpdateRedis(true, err) {
		gopool.Go(func() {
			if err := cacheSetToken(token); err != nil {
				common.SysLog("failed to update user status cache: " + err.Error())
			}
		})
	}
	return &token, err
}

func GetTokenByKey(key string, fromDB bool) (token *Token, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) && token != nil {
			gopool.Go(func() {
				if err := cacheSetToken(*token); err != nil {
					common.SysLog("failed to update user status cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		// Try Redis first
		token, err := cacheGetTokenByKey(key)
		if err == nil {
			return token, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Where(commonKeyCol+" = ?", key).First(&token).Error
	return token, err
}

func (token *Token) Insert() error {
	return DB.Create(token).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (token *Token) Update() (err error) {
	if err = token.prepareName(); err != nil {
		return err
	}
	defer func() {
		if shouldUpdateRedis(true, err) {
			gopool.Go(func() {
				err := cacheSetToken(*token)
				if err != nil {
					common.SysLog("failed to update token cache: " + err.Error())
				}
			})
		}
	}()
	err = DB.Model(token).Select("name", "name_fingerprint", "status", "expired_time", "remain_quota", "unlimited_quota",
		"model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry").Updates(token).Error
	return err
}

func (token *Token) SelectUpdate() (err error) {
	defer func() {
		if shouldUpdateRedis(true, err) {
			gopool.Go(func() {
				err := cacheSetToken(*token)
				if err != nil {
					common.SysLog("failed to update token cache: " + err.Error())
				}
			})
		}
	}()
	// This can update zero values
	return DB.Model(token).Select("accessed_time", "status").Updates(token).Error
}

func (token *Token) Delete() (err error) {
	defer func() {
		if shouldUpdateRedis(true, err) {
			gopool.Go(func() {
				err := cacheDeleteToken(token.Key)
				if err != nil {
					common.SysLog("failed to delete token cache: " + err.Error())
				}
			})
		}
	}()
	err = DB.Transaction(func(tx *gorm.DB) error {
		fingerprint := deletedTokenNameFingerprint(token.Id)
		if err := tx.Model(&Token{}).Where("id = ?", token.Id).
			UpdateColumn("name_fingerprint", fingerprint).Error; err != nil {
			return err
		}
		token.NameFingerprint = fingerprint
		return tx.Session(&gorm.Session{SkipHooks: true}).Delete(token).Error
	})
	return err
}

func (token *Token) IsModelLimitsEnabled() bool {
	return token.ModelLimitsEnabled
}

func (token *Token) GetModelLimits() []string {
	if token.ModelLimits == "" {
		return []string{}
	}
	return strings.Split(token.ModelLimits, ",")
}

func (token *Token) GetModelLimitsMap() map[string]bool {
	limits := token.GetModelLimits()
	limitsMap := make(map[string]bool)
	for _, limit := range limits {
		limitsMap[limit] = true
	}
	return limitsMap
}

func DisableModelLimits(tokenId int) error {
	token, err := GetTokenById(tokenId)
	if err != nil {
		return err
	}
	token.ModelLimitsEnabled = false
	token.ModelLimits = ""
	return token.Update()
}

func DeleteTokenById(id int, userId int) (err error) {
	// Why we need userId here? In case user want to delete other's token.
	if id == 0 || userId == 0 {
		return errors.New("id 或 userId 为空！")
	}
	token := Token{Id: id, UserId: userId}
	err = DB.Where(token).First(&token).Error
	if err != nil {
		return err
	}
	return token.Delete()
}

func IncreaseTokenQuota(tokenId int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if common.RedisEnabled {
		gopool.Go(func() {
			err := cacheIncrTokenQuota(key, int64(quota))
			if err != nil {
				common.SysLog("failed to increase token quota: " + err.Error())
			}
		})
	}
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenQuota, tokenId, quota)
		return nil
	}
	return increaseTokenQuota(tokenId, quota)
}

func increaseTokenQuota(id int, quota int) (err error) {
	err = DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota + ?", quota),
			"used_quota":    gorm.Expr("used_quota - ?", quota),
			"accessed_time": common.GetTimestamp(),
		},
	).Error
	return err
}

func DecreaseTokenQuota(id int, key string, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if common.RedisEnabled {
		gopool.Go(func() {
			err := cacheDecrTokenQuota(key, int64(quota))
			if err != nil {
				common.SysLog("failed to decrease token quota: " + err.Error())
			}
		})
	}
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenQuota, id, -quota)
		return nil
	}
	return decreaseTokenQuota(id, quota)
}

func decreaseTokenQuota(id int, quota int) (err error) {
	err = DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": common.GetTimestamp(),
		},
	).Error
	return err
}

// CountUserTokens returns total number of tokens for the given user, used for pagination
func CountUserTokens(userId int) (int64, error) {
	var total int64
	err := DB.Model(&Token{}).Where("user_id = ?", userId).Count(&total).Error
	return total, err
}

// BatchDeleteTokens 删除指定用户的一组令牌，返回成功删除数量
func BatchDeleteTokens(ids []int, userId int) (int, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空！")
	}

	tx := DB.Begin()

	var tokens []Token
	if err := tx.Where("user_id = ? AND id IN (?)", userId, ids).Find(&tokens).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	for i := range tokens {
		fingerprint := deletedTokenNameFingerprint(tokens[i].Id)
		if err := tx.Model(&Token{}).Where("id = ?", tokens[i].Id).
			Update("name_fingerprint", fingerprint).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	if err := tx.Where("user_id = ? AND id IN (?)", userId, ids).Delete(&Token{}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	if common.RedisEnabled {
		gopool.Go(func() {
			for _, t := range tokens {
				_ = cacheDeleteToken(t.Key)
			}
		})
	}

	return len(tokens), nil
}

func GetTokenKeysByIds(ids []int, userId int) ([]Token, error) {
	var tokens []Token
	err := DB.Select("id", commonKeyCol).
		Where("user_id = ? AND id IN (?)", userId, ids).
		Find(&tokens).Error
	return tokens, err
}

// InvalidateUserTokensCache 清理指定用户所有令牌在 Redis 中的缓存，
// 配合 InvalidateUserCache 使用，可在用户被禁用/删除时立即阻断其令牌的请求。
// 下一次请求将从数据库重新加载令牌及用户状态，从而立即识别出被禁用的用户。
func InvalidateUserTokensCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	if userId <= 0 {
		return errors.New("userId 无效")
	}
	var tokens []Token
	if err := DB.Unscoped().
		Select("id", commonKeyCol).
		Where("user_id = ?", userId).
		Find(&tokens).Error; err != nil {
		return err
	}
	var firstErr error
	for _, t := range tokens {
		if t.Key == "" {
			continue
		}
		if err := cacheDeleteToken(t.Key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
