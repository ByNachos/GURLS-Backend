//go:build integration

package postgres

import (
	"GURLS-Backend/internal/domain"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*PostgresStorage, func()) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := gorm.Open(postgresDriver.Open(connStr), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables for testing
	err = db.AutoMigrate(
		&domain.SubscriptionType{},
		&domain.User{},
		&domain.Link{},
		&domain.Click{},
		&domain.UserStats{},
		&domain.Session{},
		&domain.RefreshToken{},
	)
	require.NoError(t, err)

	// Seed initial subscription types for testing
	subscriptionTypes := []domain.SubscriptionType{
		{
			ID:          1,
			Name:        "free",
			DisplayName: "Free Plan",
			IsActive:    true,
		},
		{
			ID:          2,
			Name:        "base",
			DisplayName: "Base Plan",
			IsActive:    true,
		},
	}

	for _, st := range subscriptionTypes {
		err = db.Create(&st).Error
		if err != nil {
			// Ignore duplicate key errors
			t.Logf("Warning: Could not create subscription type %s: %v", st.Name, err)
		}
	}

	// Create test logger
	logger := zap.NewNop()

	// Create storage instance
	storage := New(db, logger)

	// Cleanup function
	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		pgContainer.Terminate(ctx)
	}

	return storage, cleanup
}

func TestPostgresStorage_FindOrCreateUser(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Test creating new user
	user1, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)
	assert.NotZero(t, user1.ID)
	assert.Equal(t, tgID, *user1.TelegramID)
	assert.Equal(t, "telegram", user1.RegistrationSource)
	assert.True(t, user1.IsActive)

	// Test finding existing user
	user2, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)
	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, user1.TelegramID, user2.TelegramID)
}

func TestPostgresStorage_SaveAndGetLink(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user first
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	// Create link
	title := "Test Link"
	link := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example.com",
		Alias:       "test123",
		Title:       &title,
	}

	// Test saving link
	err = storage.SaveLink(ctx, link)
	require.NoError(t, err)

	// Test getting link
	retrievedLink, err := storage.GetLink(ctx, "test123")
	require.NoError(t, err)
	assert.Equal(t, link.OriginalURL, retrievedLink.OriginalURL)
	assert.Equal(t, link.Alias, retrievedLink.Alias)
	assert.Equal(t, *link.Title, *retrievedLink.Title)
	assert.Equal(t, link.UserID, retrievedLink.UserID)
}

func TestPostgresStorage_RecordClickAdvanced(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user and link
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	link := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example.com",
		Alias:       "test123",
	}
	err = storage.SaveLink(ctx, link)
	require.NoError(t, err)

	// Test recording click with advanced data
	ipAddr := "192.168.1.1"
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
	referer := "https://google.com"
	clickTime := time.Now()

	err = storage.RecordClickAdvanced(ctx, "test123", "desktop", &ipAddr, &userAgent, &referer, &clickTime)
	require.NoError(t, err)

	// Verify click was recorded
	retrievedLink, err := storage.GetLink(ctx, "test123")
	require.NoError(t, err)
	assert.Equal(t, int64(1), retrievedLink.ClickCount)

	// Test clicks by device
	clicksByDevice, err := storage.GetClicksByDevice(ctx, retrievedLink.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), clicksByDevice["desktop"])
}

func TestPostgresStorage_GetClicksByDevice(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user and link
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	link := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example.com",
		Alias:       "test123",
	}
	err = storage.SaveLink(ctx, link)
	require.NoError(t, err)

	// Record clicks from different devices
	err = storage.RecordClickAdvanced(ctx, "test123", "desktop", nil, nil, nil, nil)
	require.NoError(t, err)

	err = storage.RecordClickAdvanced(ctx, "test123", "mobile", nil, nil, nil, nil)
	require.NoError(t, err)

	err = storage.RecordClickAdvanced(ctx, "test123", "mobile", nil, nil, nil, nil)
	require.NoError(t, err)

	// Verify clicks by device
	clicksByDevice, err := storage.GetClicksByDevice(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), clicksByDevice["desktop"])
	assert.Equal(t, int64(2), clicksByDevice["mobile"])
}

func TestPostgresStorage_ListUserLinks(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	// Create multiple links
	link1 := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example1.com",
		Alias:       "test1",
	}
	link2 := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example2.com",
		Alias:       "test2",
	}

	err = storage.SaveLink(ctx, link1)
	require.NoError(t, err)
	err = storage.SaveLink(ctx, link2)
	require.NoError(t, err)

	// List user links
	links, err := storage.ListUserLinks(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, links, 2)

	// Verify links are sorted by created_at DESC
	assert.Equal(t, "test2", links[0].Alias) // Most recent first
	assert.Equal(t, "test1", links[1].Alias)
}

func TestPostgresStorage_DeleteLink(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user and link
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	link := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example.com",
		Alias:       "test123",
	}
	err = storage.SaveLink(ctx, link)
	require.NoError(t, err)

	// Delete link (soft delete)
	err = storage.DeleteLink(ctx, "test123")
	require.NoError(t, err)

	// Verify link is not accessible
	_, err = storage.GetLink(ctx, "test123")
	assert.Error(t, err)
}

func TestPostgresStorage_AliasExists(t *testing.T) {
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	tgID := int64(12345)

	// Create user and link
	user, err := storage.FindOrCreateUser(ctx, tgID)
	require.NoError(t, err)

	link := &domain.Link{
		UserID:      user.ID,
		OriginalURL: "https://example.com",
		Alias:       "test123",
	}
	err = storage.SaveLink(ctx, link)
	require.NoError(t, err)

	// Test alias exists
	exists, err := storage.AliasExists(ctx, "test123")
	require.NoError(t, err)
	assert.True(t, exists)

	// Test alias doesn't exist
	exists, err = storage.AliasExists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}