package server

import (
	shortenerv1 "GURLS-Backend/gen/go/shortener/v1"
	"GURLS-Backend/internal/config"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/analytics"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MockAnalyticsProcessor is a mock implementation of analytics.Processor
type MockAnalyticsProcessor struct {
    mock.Mock
}

func (m *MockAnalyticsProcessor) SubmitClick(clickData *analytics.ClickData) error {
    args := m.Called(clickData)
    return args.Error(0)
}

func (m *MockAnalyticsProcessor) Start() error {
    args := m.Called()
    return args.Error(0)
}

func (m *MockAnalyticsProcessor) Stop() error {
    args := m.Called()
    return args.Error(0)
}

func (m *MockAnalyticsProcessor) GetStats() map[string]interface{} {
    args := m.Called()
    if args.Get(0) == nil {
        return nil
    }
    return args.Get(0).(map[string]interface{})
}

// MockStorage is a mock implementation of repository.Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) FindOrCreateUser(ctx context.Context, tgID int64) (*domain.User, error) {
	args := m.Called(ctx, tgID)
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockStorage) GetUserByTGID(ctx context.Context, tgID int64) (*domain.User, error) {
	args := m.Called(ctx, tgID)
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockStorage) SaveLink(ctx context.Context, link *domain.Link) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockStorage) GetLink(ctx context.Context, alias string) (*domain.Link, error) {
	args := m.Called(ctx, alias)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Link), args.Error(1)
}

func (m *MockStorage) DeleteLink(ctx context.Context, alias string) error {
	args := m.Called(ctx, alias)
	return args.Error(0)
}

func (m *MockStorage) AliasExists(ctx context.Context, alias string) (bool, error) {
	args := m.Called(ctx, alias)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorage) RecordClick(ctx context.Context, alias string, deviceType string) error {
	args := m.Called(ctx, alias, deviceType)
	return args.Error(0)
}

func (m *MockStorage) ListUserLinks(ctx context.Context, userID int64) ([]*domain.Link, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.Link), args.Error(1)
}

func (m *MockStorage) RecordClickAdvanced(ctx context.Context, alias string, deviceType string, ipAddress *string, userAgent *string, referer *string, clickedAt *time.Time) error {
	args := m.Called(ctx, alias, deviceType, ipAddress, userAgent, referer, clickedAt)
	return args.Error(0)
}

func (m *MockStorage) GetClicksByDevice(ctx context.Context, linkID int64) (map[string]int64, error) {
	args := m.Called(ctx, linkID)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func setupTestServer() (*Server, *MockStorage, *MockAnalyticsProcessor) {
    mockStorage := &MockStorage{}
    mockAnalytics := &MockAnalyticsProcessor{}
    log := zap.NewNop()

    cfg := &config.URLShortener{AliasLength: 6}
    urlShortener := service.NewURLShortener(mockStorage, cfg)

    server := &Server{
        log:                log,
        urlShortener:       urlShortener,
        storage:            mockStorage,
        analyticsProcessor: mockAnalytics,
    }

    return server, mockStorage, mockAnalytics
}

func TestServer_GetLinkStats(t *testing.T) {
	server, mockStorage, _ := setupTestServer()
	ctx := context.Background()

	// Test successful link retrieval
	t.Run("success", func(t *testing.T) {
		title := "Test Link"
		expiresAt := time.Now().Add(24 * time.Hour)
		link := &domain.Link{
			ID:          1,
			OriginalURL: "https://example.com",
			Alias:       "test123",
			Title:       &title,
			ExpiresAt:   &expiresAt,
			ClickCount:  5,
		}

		clicksByDevice := map[string]int64{
			"desktop": 3,
			"mobile":  2,
		}

		mockStorage.On("GetLink", ctx, "test123").Return(link, nil)
		mockStorage.On("GetClicksByDevice", ctx, int64(1)).Return(clicksByDevice, nil)

		req := &shortenerv1.GetLinkStatsRequest{Alias: "test123"}
		resp, err := server.GetLinkStats(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "https://example.com", resp.OriginalUrl)
		assert.Equal(t, int64(5), resp.ClickCount)
		assert.Equal(t, "Test Link", *resp.Title)
		assert.Equal(t, int64(3), resp.ClicksByDevice["desktop"])
		assert.Equal(t, int64(2), resp.ClicksByDevice["mobile"])
		mockStorage.AssertExpectations(t)
	})

	// Test link not found
	t.Run("link_not_found", func(t *testing.T) {
		mockStorage.On("GetLink", ctx, "notfound").Return(nil, repository.ErrAliasNotFound)

		req := &shortenerv1.GetLinkStatsRequest{Alias: "notfound"}
		resp, err := server.GetLinkStats(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		mockStorage.AssertExpectations(t)
	})

	// Test empty alias
	t.Run("empty_alias", func(t *testing.T) {
		req := &shortenerv1.GetLinkStatsRequest{Alias: ""}
		resp, err := server.GetLinkStats(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestServer_RecordClick(t *testing.T) {
	server, mockStorage, _ := setupTestServer()
	ctx := context.Background()

	// Test successful click recording
	t.Run("success", func(t *testing.T) {
		mockStorage.On("RecordClickAdvanced", ctx, "test123", "desktop", (*string)(nil), (*string)(nil), (*string)(nil), (*time.Time)(nil)).Return(nil)

		req := &shortenerv1.RecordClickRequest{
			Alias:      "test123",
			DeviceType: "desktop",
		}
		resp, err := server.RecordClick(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		mockStorage.AssertExpectations(t)
	})

	// Test with extended data
	t.Run("success_with_extended_data", func(t *testing.T) {
		ipAddr := "192.168.1.1"
		userAgent := "Mozilla/5.0"
		referer := "https://google.com"
		clickedAt := timestamppb.Now()

		mockStorage.On("RecordClickAdvanced", ctx, "test123", "mobile", &ipAddr, &userAgent, &referer, mock.MatchedBy(func(t *time.Time) bool {
			return t != nil
		})).Return(nil)

		req := &shortenerv1.RecordClickRequest{
			Alias:      "test123",
			DeviceType: "mobile",
			IpAddress:  &ipAddr,
			UserAgent:  &userAgent,
			Referer:    &referer,
			ClickedAt:  clickedAt,
		}
		resp, err := server.RecordClick(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		mockStorage.AssertExpectations(t)
	})

	// Test alias not found
	t.Run("alias_not_found", func(t *testing.T) {
		mockStorage.On("RecordClickAdvanced", ctx, "notfound", "desktop", (*string)(nil), (*string)(nil), (*string)(nil), (*time.Time)(nil)).Return(repository.ErrAliasNotFound)

		req := &shortenerv1.RecordClickRequest{
			Alias:      "notfound",
			DeviceType: "desktop",
		}
		resp, err := server.RecordClick(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		mockStorage.AssertExpectations(t)
	})

	// Test invalid arguments
	t.Run("empty_alias", func(t *testing.T) {
		req := &shortenerv1.RecordClickRequest{
			Alias:      "",
			DeviceType: "desktop",
		}
		resp, err := server.RecordClick(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty_device_type", func(t *testing.T) {
		req := &shortenerv1.RecordClickRequest{
			Alias:      "test123",
			DeviceType: "",
		}
		resp, err := server.RecordClick(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestServer_RedirectAndRecord(t *testing.T) {
	server, mockStorage, mockAnalytics := setupTestServer()
	ctx := context.Background()

	// Test successful redirect and record
	t.Run("success", func(t *testing.T) {
		title := "Test Link"
		expiresAt := time.Now().Add(24 * time.Hour)
		link := &domain.Link{
			ID:          1,
			OriginalURL: "https://example.com",
			Alias:       "test123",
			Title:       &title,
			ExpiresAt:   &expiresAt,
		}

		mockStorage.On("GetLink", ctx, "test123").Return(link, nil)
		mockAnalytics.On("SubmitClick", mock.AnythingOfType("*analytics.ClickData")).Return(nil)

		req := &shortenerv1.RedirectAndRecordRequest{Alias: "test123"}
		resp, err := server.RedirectAndRecord(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "https://example.com", resp.OriginalUrl)
		assert.Equal(t, "Test Link", *resp.Title)
		assert.NotNil(t, resp.ExpiresAt)
		
		// Wait a bit for async operation to complete
		time.Sleep(10 * time.Millisecond)
		mockStorage.AssertExpectations(t)
		mockAnalytics.AssertExpectations(t)
	})

	// Test with user agent detection
	t.Run("success_with_mobile_user_agent", func(t *testing.T) {
		link := &domain.Link{
			ID:          1,
			OriginalURL: "https://example.com",
			Alias:       "test123",
		}

		userAgent := "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)"

		mockStorage.On("GetLink", ctx, "test123").Return(link, nil)
		mockAnalytics.On("SubmitClick", mock.AnythingOfType("*analytics.ClickData")).Return(nil)

		req := &shortenerv1.RedirectAndRecordRequest{
			Alias:     "test123",
			UserAgent: &userAgent,
		}
		resp, err := server.RedirectAndRecord(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "https://example.com", resp.OriginalUrl)
		
		// Wait a bit for async operation to complete
		time.Sleep(10 * time.Millisecond)
		mockStorage.AssertExpectations(t)
		mockAnalytics.AssertExpectations(t)
	})

	// Test link not found
	t.Run("link_not_found", func(t *testing.T) {
		mockStorage.On("GetLink", ctx, "notfound").Return(nil, repository.ErrAliasNotFound)

		req := &shortenerv1.RedirectAndRecordRequest{Alias: "notfound"}
		resp, err := server.RedirectAndRecord(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		mockStorage.AssertExpectations(t)
	})

	// Test empty alias
	t.Run("empty_alias", func(t *testing.T) {
		req := &shortenerv1.RedirectAndRecordRequest{Alias: ""}
		resp, err := server.RedirectAndRecord(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestServer_DeleteLink(t *testing.T) {
	server, mockStorage, _ := setupTestServer()
	ctx := context.Background()

	// Test successful deletion
	t.Run("success", func(t *testing.T) {
		mockStorage.On("DeleteLink", ctx, "test123").Return(nil)

		req := &shortenerv1.DeleteLinkRequest{Alias: "test123"}
		resp, err := server.DeleteLink(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		mockStorage.AssertExpectations(t)
	})

	// Test link not found
	t.Run("link_not_found", func(t *testing.T) {
		mockStorage.On("DeleteLink", ctx, "notfound").Return(repository.ErrAliasNotFound)

		req := &shortenerv1.DeleteLinkRequest{Alias: "notfound"}
		resp, err := server.DeleteLink(ctx, req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		mockStorage.AssertExpectations(t)
	})
}