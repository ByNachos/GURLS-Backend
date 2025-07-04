package server

import (
	shortenerv1 "GURLS-Backend/gen/go/shortener/v1"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	shortenerv1.UnimplementedShortenerServer
	log          *zap.Logger
	urlShortener *service.URLShortenerService
	storage      repository.Storage
}

func Register(gRPCServer *grpc.Server, log *zap.Logger, urlShortener *service.URLShortenerService, storage repository.Storage) {
	shortenerv1.RegisterShortenerServer(gRPCServer, &Server{log: log, urlShortener: urlShortener, storage: storage})
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
		link.Title = *req.Title
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
		if link.Title != "" {
			linkInfo.Title = &link.Title
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

	response := &shortenerv1.GetLinkStatsResponse{
		OriginalUrl:    link.OriginalURL,
		ClickCount:     link.ClickCount,
		ClicksByDevice: link.ClicksByDevice,
	}

	if link.Title != "" {
		response.Title = &link.Title
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

	if err := s.storage.RecordClick(ctx, req.GetAlias(), req.GetDeviceType()); err != nil {
		if errors.Is(err, repository.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		log.Error("failed to record click", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not record click")
	}

	log.Info("click recorded successfully", zap.String("device_type", req.GetDeviceType()))
	return &emptypb.Empty{}, nil
}
