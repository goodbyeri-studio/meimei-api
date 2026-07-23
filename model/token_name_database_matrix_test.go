package model

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTokenNameUniquenessDatabaseMatrix(t *testing.T) {
	dialect := os.Getenv("TOKEN_NAME_TEST_DIALECT")
	dsn := os.Getenv("TOKEN_NAME_TEST_DSN")
	if dialect == "" {
		dialect = "sqlite"
		dsn = "file:token-name-database-matrix?mode=memory&cache=shared"
	}

	var (
		db     *gorm.DB
		dbType common.DatabaseType
		err    error
	)
	switch dialect {
	case "sqlite":
		dbType = common.DatabaseTypeSQLite
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case "mysql":
		if dsn == "" {
			t.Skip("TOKEN_NAME_TEST_DSN is required for MySQL")
		}
		dbType = common.DatabaseTypeMySQL
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		if dsn == "" {
			t.Skip("TOKEN_NAME_TEST_DSN is required for PostgreSQL")
		}
		dbType = common.DatabaseTypePostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported TOKEN_NAME_TEST_DIALECT %q", dialect)
	}
	require.NoError(t, err)

	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(dbType, dbType)
	managedTokensTable := false
	t.Cleanup(func() {
		common.SetDatabaseTypes(previousMainType, previousLogType)
		if managedTokensTable {
			_ = db.Migrator().DropTable(&Token{})
		}
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if db.Migrator().HasTable(&Token{}) {
		t.Fatalf("refusing to use %s database with an existing tokens table", dialect)
	}
	managedTokensTable = true
	require.NoError(t, db.AutoMigrate(&Token{}))

	legacy := []Token{
		{UserId: 1, Key: "legacy-key-1", Name: "same"},
		{UserId: 1, Key: "legacy-key-2", Name: " same "},
		{UserId: 2, Key: "legacy-key-3", Name: "same"},
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&legacy).Error)
	require.NoError(t, migrateDuplicateTokenNames(db))

	var migrated []Token
	require.NoError(t, db.Where("user_id = ?", 1).Order("id asc").Find(&migrated).Error)
	require.Len(t, migrated, 2)
	assert.Equal(t, "same", migrated[0].Name)
	assert.True(t, strings.HasPrefix(migrated[1].Name, "same ("))

	duplicate := &Token{UserId: 1, Key: "duplicate-key", Name: "same"}
	assert.Error(t, db.Create(duplicate).Error)
	require.NoError(t, db.Create(&Token{UserId: 2, Key: "case-key", Name: "SAME"}).Error)

	require.NoError(t, db.Delete(&migrated[0]).Error)
	require.NoError(t, db.Create(&Token{
		UserId: 1,
		Key:    fmt.Sprintf("reused-key-%s", dialect),
		Name:   "same",
	}).Error)
}
