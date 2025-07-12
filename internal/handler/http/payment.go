package http

import (
	"GURLS-Backend/internal/auth"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// PaymentHandler handles payment-related HTTP requests
type PaymentHandler struct {
	storage        repository.Storage
	paymentService *service.PaymentService
	log            *zap.Logger
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(storage repository.Storage, paymentService *service.PaymentService, log *zap.Logger) *PaymentHandler {
	return &PaymentHandler{
		storage:        storage,
		paymentService: paymentService,
		log:            log,
	}
}

// CreatePaymentRequest represents a payment creation request
type CreatePaymentRequest struct {
	SubscriptionTypeID int16   `json:"subscription_type_id"`
	Amount             float64 `json:"amount"`
	Currency           string  `json:"currency"`
	ReturnURL          string  `json:"return_url"`
}

// CreatePaymentResponse represents a payment creation response
type CreatePaymentResponse struct {
	PaymentID       string  `json:"payment_id"`
	Status          string  `json:"status"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	ConfirmationURL string  `json:"confirmation_url"`
	CreatedAt       string  `json:"created_at"`
}

// PaymentStatusResponse represents payment status response
type PaymentStatusResponse struct {
	PaymentID   string  `json:"payment_id"`
	Status      string  `json:"status"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

// CreatePayment handles POST /api/payments/create
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by JWT middleware)
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Debug("invalid create payment request", zap.Error(err))
		h.writeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.SubscriptionTypeID == 0 {
		h.writeError(w, "subscription_type_id is required", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		h.writeError(w, "amount must be positive", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}

	// Check if subscription type exists
	subscriptionType, err := h.storage.GetSubscriptionType(r.Context(), req.SubscriptionTypeID)
	if err != nil {
		if err == repository.ErrSubscriptionTypeNotFound {
			h.writeError(w, "Invalid subscription type", http.StatusBadRequest)
			return
		}
		h.log.Error("failed to get subscription type", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create payment request
	paymentReq := &domain.PaymentRequest{
		UserID:             userID,
		SubscriptionTypeID: req.SubscriptionTypeID,
		Amount:             req.Amount,
		Currency:           req.Currency,
		Description:        fmt.Sprintf("Подписка %s", subscriptionType.DisplayName),
		ReturnURL:          req.ReturnURL,
	}

	// Create payment through service
	payment, err := h.paymentService.CreatePayment(r.Context(), paymentReq)
	if err != nil {
		h.log.Error("failed to create payment", zap.Error(err))
		h.writeError(w, "Failed to create payment", http.StatusInternalServerError)
		return
	}

	// Return response
	response := CreatePaymentResponse{
		PaymentID:       payment.PaymentID,
		Status:          payment.Status,
		Amount:          payment.Amount,
		Currency:        payment.Currency,
		ConfirmationURL: payment.ConfirmationURL,
		CreatedAt:       payment.CreatedAt,
	}

	h.log.Info("payment created", zap.String("payment_id", payment.PaymentID), zap.Int64("user_id", userID))
	h.writeJSON(w, response, http.StatusCreated)
}

// GetPaymentStatus handles GET /api/payments/status/{payment_id}
func (h *PaymentHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Extract payment ID from URL path
	paymentID := extractPaymentIDFromPath(r.URL.Path)
	if paymentID == "" {
		h.writeError(w, "Payment ID is required", http.StatusBadRequest)
		return
	}

	// Get payment from database
	payment, err := h.storage.GetPaymentByID(r.Context(), paymentID)
	if err != nil {
		if err == repository.ErrPaymentNotFound {
			h.writeError(w, "Payment not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get payment", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if payment belongs to user
	if payment.UserID != userID {
		h.writeError(w, "Payment not found", http.StatusNotFound)
		return
	}

	// Prepare response
	response := PaymentStatusResponse{
		PaymentID: payment.PaymentID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		Currency:  payment.Currency,
		CreatedAt: payment.CreatedAt.Format(time.RFC3339),
	}

	if payment.CompletedAt != nil {
		completedAt := payment.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	h.writeJSON(w, response, http.StatusOK)
}

// WebhookHandler handles POST /api/payments/webhook
func (h *PaymentHandler) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	// Parse webhook payload
	var payload domain.YookassaWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Error("invalid webhook payload", zap.Error(err))
		h.writeError(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	h.log.Info("received webhook", 
		zap.String("type", payload.Type),
		zap.String("event", payload.Event),
		zap.String("payment_id", payload.Object.ID),
		zap.String("status", payload.Object.Status),
	)

	// Handle payment status changes
	if payload.Type == "notification" && payload.Event == "payment.succeeded" {
		err := h.paymentService.ProcessSuccessfulPayment(r.Context(), &payload)
		if err != nil {
			h.log.Error("failed to process successful payment", zap.Error(err))
			h.writeError(w, "Failed to process payment", http.StatusInternalServerError)
			return
		}
	} else if payload.Type == "notification" && payload.Event == "payment.canceled" {
		err := h.paymentService.ProcessCanceledPayment(r.Context(), &payload)
		if err != nil {
			h.log.Error("failed to process canceled payment", zap.Error(err))
			h.writeError(w, "Failed to process payment", http.StatusInternalServerError)
			return
		}
	}

	// Return success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ListPayments handles GET /api/payments
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get user's payments
	payments, err := h.storage.ListUserPayments(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to list user payments", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var response []PaymentStatusResponse
	for _, payment := range payments {
		paymentResp := PaymentStatusResponse{
			PaymentID: payment.PaymentID,
			Status:    payment.Status,
			Amount:    payment.Amount,
			Currency:  payment.Currency,
			CreatedAt: payment.CreatedAt.Format(time.RFC3339),
		}
		if payment.CompletedAt != nil {
			completedAt := payment.CompletedAt.Format(time.RFC3339)
			paymentResp.CompletedAt = &completedAt
		}
		response = append(response, paymentResp)
	}

	h.writeJSON(w, response, http.StatusOK)
}

// Helper functions

func extractPaymentIDFromPath(path string) string {
	// Extract payment ID from /api/payments/status/{payment_id}
	parts := strings.Split(path, "/")
	if len(parts) >= 5 && parts[4] != "" {
		return parts[4]
	}
	return ""
}

func (h *PaymentHandler) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *PaymentHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}