package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateDuplicateTokenNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:token-name-migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))

	longName := strings.Repeat("界", MaxTokenNameLength+1)
	tokens := []Token{
		{UserId: 1, Key: "key-1", Name: "grok"},
		{UserId: 1, Key: "key-2", Name: " grok "},
		{UserId: 1, Key: "key-3", Name: "grok (2)"},
		{UserId: 1, Key: "key-4", Name: "   "},
		{UserId: 2, Key: "key-5", Name: "grok"},
		{UserId: 1, Key: "key-6", Name: longName},
		{UserId: 1, Key: "key-7", Name: longName},
		{UserId: 1, Key: "key-8", Name: "grok"},
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&tokens).Error)
	require.NoError(t, db.Delete(&tokens[7]).Error)

	require.NoError(t, migrateDuplicateTokenNames(db))
	require.NoError(t, migrateDuplicateTokenNames(db))

	var activeTokens []Token
	require.NoError(t, db.Order("id asc").Find(&activeTokens).Error)
	namesByKey := make(map[string]string, len(activeTokens))
	seenByUser := make(map[int]map[string]struct{})
	for _, token := range activeTokens {
		namesByKey[token.Key] = token.Name
		if seenByUser[token.UserId] == nil {
			seenByUser[token.UserId] = make(map[string]struct{})
		}
		_, duplicate := seenByUser[token.UserId][token.Name]
		assert.False(t, duplicate, "duplicate active name %q for user %d", token.Name, token.UserId)
		seenByUser[token.UserId][token.Name] = struct{}{}
	}

	assert.Equal(t, "grok", namesByKey["key-1"])
	assert.Equal(t, "grok (3)", namesByKey["key-2"])
	assert.Equal(t, "grok (2)", namesByKey["key-3"])
	assert.Equal(t, "API Key", namesByKey["key-4"])
	assert.Equal(t, "grok", namesByKey["key-5"])
	assert.Equal(t, strings.Repeat("界", MaxTokenNameLength), namesByKey["key-6"])
	assert.NotEqual(t, longName, namesByKey["key-7"])
	assert.LessOrEqual(t, len([]rune(namesByKey["key-7"])), MaxTokenNameLength)

	var deleted Token
	require.NoError(t, db.Unscoped().First(&deleted, tokens[7].Id).Error)
	assert.Equal(t, "grok", deleted.Name)
	assert.True(t, deleted.DeletedAt.Valid)
	assert.NotEmpty(t, deleted.NameFingerprint)

	reusable := &Token{UserId: 1, Key: "key-reusable", Name: "reusable"}
	require.NoError(t, db.Create(reusable).Error)
	require.NoError(t, db.Delete(reusable).Error)
	var deletedReusable Token
	require.NoError(t, db.Unscoped().First(&deletedReusable, reusable.Id).Error)
	assert.True(t, deletedReusable.DeletedAt.Valid)
	assert.Equal(t, deletedTokenNameFingerprint(reusable.Id), deletedReusable.NameFingerprint)
	require.NoError(t, db.Create(&Token{UserId: 1, Key: "key-reusable-new", Name: "reusable"}).Error)

	assert.Error(t, db.Create(&Token{UserId: 1, Key: "key-duplicate", Name: "grok"}).Error)
	require.NoError(t, db.Create(&Token{UserId: 2, Key: "key-case", Name: "GROK"}).Error)
}

func TestNormalizeTokenNameUsesUnicodeCharacterLimit(t *testing.T) {
	name, err := NormalizeTokenName("  中文名称  ")
	require.NoError(t, err)
	assert.Equal(t, "中文名称", name)

	_, err = NormalizeTokenName(strings.Repeat("界", MaxTokenNameLength+1))
	assert.ErrorIs(t, err, ErrTokenNameTooLong)
	_, err = NormalizeTokenName("   ")
	assert.ErrorIs(t, err, ErrTokenNameEmpty)
}

func TestBatchDeleteTokensReleasesNameFingerprint(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:token-name-batch-delete?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))
	require.NoError(t, migrateDuplicateTokenNames(db))

	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	token := &Token{UserId: 9, Key: "batch-delete-key", Name: "reusable"}
	require.NoError(t, token.Insert())
	deleted, err := BatchDeleteTokens([]int{token.Id}, token.UserId)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	require.NoError(t, (&Token{UserId: 9, Key: "batch-delete-new-key", Name: "reusable"}).Insert())
}
