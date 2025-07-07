package server

import (
	shortenerv1 "GURLS-Backend/gen/go/shortener/v1"
	"GURLS-Backend/internal/analytics"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	shortenerv1.UnimplementedShortenerServer
	log                *zap.Logger
	urlShortener       *service.URLShortenerService
	storage            repository.Storage
	analyticsProcessor analytics.ProcessorInterface
}

func Register(gRPCServer *grpc.Server, log *zap.Logger, urlShortener *service.URLShortenerService, storage repository.Storage, analyticsProcessor analytics.ProcessorInterface) {
	shortenerv1.RegisterShortenerServer(gRPCServer, &Server{
		log:                log,
		urlShortener:       urlShortener,
		storage:            storage,
		analyticsProcessor: analyticsProcessor,
	})
}

func (s *Server) CreateLink(ctx context.Context, req *shortenerv1.CreateLinkRequest) (*shortenerv1.CreateLinkResponse, error) {
	log := s.log.With(zap.String("rpc", "CreateLink"), zap.Int64("tg_id", req.GetUserTgId()))
	if req.GetOriginalUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "original_url is required")
	}
	if req.GetUserTgId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_tg_id is required")
	}

	user, err := s.storage.FindOrCreateUser(ctx, req.GetUserTgId())
	if err != nil {
		log.Error("failed to find or create user", zap.Error(err))
		return nil, status.Error(codes.Internal, "user processing failed")
	}

	link := &domain.Link{UserID: user.ID, OriginalURL: req.GetOriginalUrl()}
	if req.Title != nil {
		link.Title = req.Title
	}
	if req.ExpiresAt != nil {
		expiresAt := req.GetExpiresAt().AsTime()
		link.ExpiresAt = &expiresAt
	}

	alias, err := s.urlShortener.Shorten(ctx, link, req.CustomAlias)
	if err != nil {
		if errors.Is(err, repository.ErrAliasExists) {
			return nil, status.Error(codes.AlreadyExists, "this alias is already taken")
		}
		log.Error("failed to shorten link", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create link")
	}

	log.Info("link created successfully", zap.String("alias", alias))
	return &shortenerv1.CreateLinkResponse{Alias: alias}, nil
}

func (s *Server) ListUserLinks(ctx context.Context, req *shortenerv1.ListUserLinksRequest) (*shortenerv1.ListUserLinksResponse, error) {
	log := s.log.With(zap.String("rpc", "ListUserLinks"), zap.Int64("tg_id", req.GetUserTgId()))
	if req.GetUserTgId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_tg_id is required")
	}

	user, err := s.storage.GetUserByTGID(ctx, req.GetUserTgId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	links, err := s.storage.ListUserLinks(ctx, user.ID)
	if err != nil {
		log.Error("failed to list user links", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not list links")
	}

	res := &shortenerv1.ListUserLinksResponse{Links: make([]*shortenerv1.LinkInfo, 0, len(links))}
	for _, link := range links {
		linkInfo := &shortenerv1.LinkInfo{Alias: link.Alias, OriginalUrl: link.OriginalURL}
		if link.Title != nil && *link.Title != "" {
			linkInfo.Title = link.Title
		}
		res.Links = append(res.Links, linkInfo)
	}
	return res, nil
}

func (s *Server) DeleteLink(ctx context.Context, req *shortenerv1.DeleteLinkRequest) (*emptypb.Empty, error) {
	log := s.log.With(zap.String("rpc", "DeleteLink"), zap.String("alias", req.GetAlias()))
	if req.GetAlias() == "" {
		return nil, status.Error(codes.InvalidArgument, "alias is required")
	}
	if err := s.storage.DeleteLink(ctx, req.GetAlias()); err != nil {
		if errors.Is(err, repository.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		log.Error("failed to delete link from storage", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not delete link")
	}
	log.Info("link deleted successfully")
	return &emptypb.Empty{}, nil
}

func (s *Server) GetLinkStats(ctx context.Context, req *shortenerv1.GetLinkStatsRequest) (*shortenerv1.GetLinkStatsResponse, error) {
	log := s.log.With(zap.String("rpc", "GetLinkStats"), zap.String("alias", req.GetAlias()))
	if req.GetAlias() == "" {
		return nil, status.Error(codes.InvalidArgument, "alias is required")
	}

	link, err := s.storage.GetLink(ctx, req.GetAlias())
	if err != nil {
		if errors.Is(err, repository.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		log.Error("failed to get link from storage", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not retrieve link stats")
	}

	// Get clicks by device from database
	clicksByDevice, err := s.storage.GetClicksByDevice(ctx, link.ID)
	if err != nil {
		log.Warn("failed to get clicks by device, using empty map", zap.Error(err))
		clicksByDevice = make(map[string]int64)
	}

	response := &shortenerv1.GetLinkStatsResponse{
		OriginalUrl:    link.OriginalURL,
		ClickCount:     link.ClickCount,
		ClicksByDevice: clicksByDevice,
	}

	if link.Title != nil && *link.Title != "" {
		response.Title = link.Title
	}
	if link.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*link.ExpiresAt)
	}

	return response, nil
}

func (s *Server) RecordClick(ctx context.Context, req *shortenerv1.RecordClickRequest) (*emptypb.Empty, error) {
	log := s.log.With(zap.String("rpc", "RecordClick"), zap.String("alias", req.GetAlias()))
	if req.GetAlias() == "" {
		return nil, status.Error(codes.InvalidArgument, "alias is required")
	}
	if req.GetDeviceType() == "" {
		return nil, status.Error(codes.InvalidArgument, "device_type is required")
	}

	// Use advanced recording if additional fields are provided
	var clickedAt *time.Time
	if req.ClickedAt != nil {
		t := req.GetClickedAt().AsTime()
		clickedAt = &t
	}

	err := s.storage.RecordClickAdvanced(
		ctx,
		req.GetAlias(),
		req.GetDeviceType(),
		req.IpAddress,
		req.UserAgent,
		req.Referer,
		clickedAt,
	)

	if err != nil {
		if errors.Is(err, repository.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		log.Error("failed to record click", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not record click")
	}

	log.Info("click recorded successfully", zap.String("device_type", req.GetDeviceType()))
	return &emptypb.Empty{}, nil
}

func (s *Server) RedirectAndRecord(ctx context.Context, req *shortenerv1.RedirectAndRecordRequest) (*shortenerv1.RedirectAndRecordResponse, error) {
	log := s.log.With(zap.String("rpc", "RedirectAndRecord"), zap.String("alias", req.GetAlias()))
	if req.GetAlias() == "" {
		return nil, status.Error(codes.InvalidArgument, "alias is required")
	}

	// Get link first
	link, err := s.storage.GetLink(ctx, req.GetAlias())
	if err != nil {
		if errors.Is(err, repository.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		log.Error("failed to get link from storage", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not retrieve link")
	}

	// Prepare response
	response := &shortenerv1.RedirectAndRecordResponse{
		OriginalUrl: link.OriginalURL,
	}
	if link.Title != nil && *link.Title != "" {
		response.Title = link.Title
	}
	if link.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*link.ExpiresAt)
	}

	// Submit click data to reliable analytics processor
	var clickedAt *time.Time
	if req.ClickedAt != nil {
		t := req.GetClickedAt().AsTime()
		clickedAt = &t
	}

	clickData := &analytics.ClickData{
		Alias:     req.GetAlias(),
		IPAddress: req.IpAddress,
		UserAgent: req.UserAgent,
		Referer:   req.Referer,
		ClickedAt: clickedAt,
	}

	// Submit to analytics processor (non-blocking with retry and error handling)
	if err := s.analyticsProcessor.SubmitClick(clickData); err != nil {
		// Log the submission error, but don't fail the redirect
		log.Error("failed to submit click for analytics processing", 
			zap.String("alias", req.GetAlias()),
			zap.Error(err),
		)
		
		// Fallback: try to record immediately (synchronous fallback)
		// This ensures analytics are recorded even if the processor is unavailable
		log.Info("attempting synchronous analytics fallback")
		fallbackErr := s.storage.RecordClickAdvanced(
			ctx, // Use request context for immediate operation
			req.GetAlias(),
			"unknown", // Device type will be "unknown" in fallback
			req.IpAddress,
			req.UserAgent,
			req.Referer,
			clickedAt,
		)
		if fallbackErr != nil {
			log.Error("synchronous analytics fallback also failed", zap.Error(fallbackErr))
		} else {
			log.Info("synchronous analytics fallback succeeded")
		}
	}

	log.Info("redirect processed successfully")
	return response, nil
}

// Helper function for simple string contains check (fallback only)
func contains(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	// Simple case-insensitive contains check for fallback
	sLower := ""
	substrLower := ""
	
	// Convert to lowercase manually
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			sLower += string(r + ('a' - 'A'))
		} else {
			sLower += string(r)
		}
	}
	
	for _, r := range substr {
		if r >= 'A' && r <= 'Z' {
			substrLower += string(r + ('a' - 'A'))
		} else {
			substrLower += string(r)
		}
	}
	
	return len(sLower) >= len(substrLower) && containsSubstring(sLower, substrLower)
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
