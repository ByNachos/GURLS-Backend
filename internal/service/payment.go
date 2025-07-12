package service

import (
	"GURLS-Backend/internal/config"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// PaymentService handles payment operations with YooKassa integration
type PaymentService struct {
	storage        repository.Storage
	log            *zap.Logger
	shopID         string
	secretKey      string
	apiURL         string
	testMode       bool
	httpClient     *http.Client
}


// YooKassaCreatePaymentRequest represents YooKassa payment creation request
type YooKassaCreatePaymentRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Description  string `json:"description"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Capture  bool `json:"capture"`
	Metadata struct {
		UserID               string `json:"user_id"`
		SubscriptionTypeID   string `json:"subscription_type_id"`
		PaymentID            string `json:"payment_id"`
	} `json:"metadata"`
}

// YooKassaPaymentResponse represents YooKassa payment creation response
type YooKassaPaymentResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Amount       struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Description  string `json:"description"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
	CreatedAt string `json:"created_at"`
	Test      bool   `json:"test"`
	Metadata  struct {
		UserID               string `json:"user_id"`
		SubscriptionTypeID   string `json:"subscription_type_id"`
		PaymentID            string `json:"payment_id"`
	} `json:"metadata"`
}

// NewPaymentService creates a new payment service
func NewPaymentService(storage repository.Storage, paymentConfig *config.Payment, log *zap.Logger) *PaymentService {
	return &PaymentService{
		storage:    storage,
		log:        log,
		shopID:     paymentConfig.ShopID,
		secretKey:  paymentConfig.SecretKey,
		apiURL:     paymentConfig.APIURL,
		testMode:   paymentConfig.TestMode,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreatePayment creates a new payment with YooKassa
func (s *PaymentService) CreatePayment(ctx context.Context, req *domain.PaymentRequest) (*domain.PaymentResponse, error) {
	// Generate unique payment ID
	paymentID, err := s.generatePaymentID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment ID: %w", err)
	}

	// Create payment record in database first
	payment := &domain.Payment{
		UserID:             req.UserID,
		PaymentID:          paymentID,
		Amount:             req.Amount,
		Currency:           req.Currency,
		Status:             string(domain.PaymentStatusPending),
		SubscriptionTypeID: req.SubscriptionTypeID,
	}

	err = s.storage.CreatePayment(ctx, payment)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	// In test mode, create mock payment response without calling YooKassa
	if s.testMode {
		return s.createMockPayment(ctx, payment, req)
	}

	// Prepare YooKassa request
	yooKassaReq := YooKassaCreatePaymentRequest{
		Description: req.Description,
		Capture:     true,
	}

	yooKassaReq.Amount.Value = fmt.Sprintf("%.2f", req.Amount)
	yooKassaReq.Amount.Currency = req.Currency
	yooKassaReq.Confirmation.Type = "redirect"
	yooKassaReq.Confirmation.ReturnURL = req.ReturnURL
	yooKassaReq.Metadata.UserID = strconv.FormatInt(req.UserID, 10)
	yooKassaReq.Metadata.SubscriptionTypeID = strconv.Itoa(int(req.SubscriptionTypeID))
	yooKassaReq.Metadata.PaymentID = paymentID

	// Send request to YooKassa
	yooKassaResp, err := s.sendYooKassaRequest(ctx, "POST", "/payments", yooKassaReq)
	if err != nil {
		// Update payment status to failed
		payment.Status = string(domain.PaymentStatusFailed)
		errMsg := err.Error()
		payment.FailureReason = &errMsg
		s.storage.UpdatePayment(ctx, payment)
		return nil, fmt.Errorf("failed to create YooKassa payment: %w", err)
	}

	// Parse YooKassa response
	var yooKassaPayment YooKassaPaymentResponse
	if err := json.Unmarshal(yooKassaResp, &yooKassaPayment); err != nil {
		return nil, fmt.Errorf("failed to parse YooKassa response: %w", err)
	}

	// Update payment with YooKassa data
	payment.YookassaPaymentID = yooKassaPayment.ID
	yooKassaData, _ := json.Marshal(yooKassaPayment)
	payment.YookassaPaymentData = string(yooKassaData)
	payment.Status = yooKassaPayment.Status

	err = s.storage.UpdatePayment(ctx, payment)
	if err != nil {
		s.log.Error("failed to update payment with YooKassa data", zap.Error(err))
	}

	// Prepare response
	response := &domain.PaymentResponse{
		PaymentID:       paymentID,
		Status:          yooKassaPayment.Status,
		Amount:          req.Amount,
		Currency:        req.Currency,
		ConfirmationURL: yooKassaPayment.Confirmation.ConfirmationURL,
		CreatedAt:       yooKassaPayment.CreatedAt,
	}

	s.log.Info("payment created successfully",
		zap.String("payment_id", paymentID),
		zap.String("yookassa_id", yooKassaPayment.ID),
		zap.Int64("user_id", req.UserID),
	)

	return response, nil
}

// ProcessSuccessfulPayment processes a successful payment webhook
func (s *PaymentService) ProcessSuccessfulPayment(ctx context.Context, webhook *domain.YookassaWebhookPayload) error {
	s.log.Info("processing successful payment",
		zap.String("yookassa_payment_id", webhook.Object.ID),
		zap.String("status", webhook.Object.Status),
	)

	// Find payment by YooKassa payment ID
	payment, err := s.storage.GetPaymentByYooKassaID(ctx, webhook.Object.ID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status
	payment.Status = string(domain.PaymentStatusSucceeded)
	now := time.Now()
	payment.CompletedAt = &now

	err = s.storage.UpdatePayment(ctx, payment)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	// Upgrade user subscription
	err = s.upgradeUserSubscription(ctx, payment)
	if err != nil {
		s.log.Error("failed to upgrade user subscription", zap.Error(err))
		// Don't return error - payment is successful even if subscription upgrade fails
	}

	s.log.Info("payment processed successfully",
		zap.String("payment_id", payment.PaymentID),
		zap.Int64("user_id", payment.UserID),
	)

	return nil
}

// ProcessCanceledPayment processes a canceled payment webhook
func (s *PaymentService) ProcessCanceledPayment(ctx context.Context, webhook *domain.YookassaWebhookPayload) error {
	s.log.Info("processing canceled payment",
		zap.String("yookassa_payment_id", webhook.Object.ID),
		zap.String("status", webhook.Object.Status),
	)

	// Find payment by YooKassa payment ID
	payment, err := s.storage.GetPaymentByYooKassaID(ctx, webhook.Object.ID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status
	payment.Status = string(domain.PaymentStatusCanceled)
	now := time.Now()
	payment.CompletedAt = &now

	err = s.storage.UpdatePayment(ctx, payment)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	s.log.Info("payment canceled successfully",
		zap.String("payment_id", payment.PaymentID),
		zap.Int64("user_id", payment.UserID),
	)

	return nil
}

// upgradeUserSubscription upgrades user's subscription after successful payment
func (s *PaymentService) upgradeUserSubscription(ctx context.Context, payment *domain.Payment) error {
	// Get user
	user, err := s.storage.GetUserByID(ctx, payment.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Create subscription change record
	subscriptionChange := &domain.SubscriptionChange{
		UserID:            payment.UserID,
		OldSubscriptionID: &user.SubscriptionTypeID,
		NewSubscriptionID: payment.SubscriptionTypeID,
		PaymentID:         &payment.ID,
		ChangeType:        string(domain.SubscriptionChangeTypeUpgrade),
		EffectiveDate:     time.Now(),
		IsActive:          true,
	}

	// Calculate expiration date (30 days from now)
	expirationDate := time.Now().AddDate(0, 1, 0) // 1 month
	subscriptionChange.ExpirationDate = &expirationDate

	err = s.storage.CreateSubscriptionChange(ctx, subscriptionChange)
	if err != nil {
		return fmt.Errorf("failed to create subscription change: %w", err)
	}

	// Update user subscription
	user.SubscriptionTypeID = payment.SubscriptionTypeID
	user.SubscriptionExpiresAt = &expirationDate

	err = s.storage.UpdateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user subscription: %w", err)
	}

	s.log.Info("user subscription upgraded",
		zap.Int64("user_id", payment.UserID),
		zap.Int16("old_subscription", *subscriptionChange.OldSubscriptionID),
		zap.Int16("new_subscription", payment.SubscriptionTypeID),
	)

	return nil
}

// sendYooKassaRequest sends HTTP request to YooKassa API
func (s *PaymentService) sendYooKassaRequest(ctx context.Context, method, path string, payload interface{}) ([]byte, error) {
	// Marshal payload
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	// Create request
	url := s.apiURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", s.generateIdempotenceKey())
	
	// Basic authentication (shopID:secretKey)
	auth := base64.StdEncoding.EncodeToString([]byte(s.shopID + ":" + s.secretKey))
	req.Header.Set("Authorization", "Basic "+auth)

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.log.Error("YooKassa API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)),
		)
		return nil, fmt.Errorf("YooKassa API error: %d %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// generatePaymentID generates a unique payment ID
func (s *PaymentService) generatePaymentID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("payment_%x", bytes), nil
}

// generateIdempotenceKey generates an idempotence key for YooKassa requests
func (s *PaymentService) generateIdempotenceKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// createMockPayment creates a mock payment response for testing
func (s *PaymentService) createMockPayment(ctx context.Context, payment *domain.Payment, req *domain.PaymentRequest) (*domain.PaymentResponse, error) {
	// Generate fake YooKassa payment ID
	mockYooKassaID := fmt.Sprintf("test_%x", time.Now().UnixNano())
	
	// Create mock confirmation URL that will simulate payment success
	mockConfirmationURL := fmt.Sprintf("http://localhost:3000/payment-success?payment_id=%s&mock=true", payment.PaymentID)
	
	// Update payment with mock data
	payment.YookassaPaymentID = mockYooKassaID
	payment.Status = "pending"
	
	// Create mock YooKassa data
	mockYooKassaData := map[string]interface{}{
		"id": mockYooKassaID,
		"status": "pending",
		"amount": map[string]interface{}{
			"value": fmt.Sprintf("%.2f", req.Amount),
			"currency": req.Currency,
		},
		"test": true,
		"created_at": time.Now().Format(time.RFC3339),
	}
	
	yooKassaDataBytes, _ := json.Marshal(mockYooKassaData)
	payment.YookassaPaymentData = string(yooKassaDataBytes)
	
	err := s.storage.UpdatePayment(ctx, payment)
	if err != nil {
		s.log.Error("failed to update mock payment", zap.Error(err))
	}
	
	// Simulate successful payment after a short delay (for realistic experience)
	go func() {
		time.Sleep(2 * time.Second)
		s.simulatePaymentSuccess(context.Background(), payment)
	}()
	
	response := &domain.PaymentResponse{
		PaymentID:       payment.PaymentID,
		Status:          "pending",
		Amount:          req.Amount,
		Currency:        req.Currency,
		ConfirmationURL: mockConfirmationURL,
		CreatedAt:       time.Now().Format(time.RFC3339),
	}
	
	s.log.Info("mock payment created successfully",
		zap.String("payment_id", payment.PaymentID),
		zap.String("mock_yookassa_id", mockYooKassaID),
		zap.Int64("user_id", req.UserID),
		zap.Bool("test_mode", true),
	)
	
	return response, nil
}

// simulatePaymentSuccess simulates a successful payment webhook for testing
func (s *PaymentService) simulatePaymentSuccess(ctx context.Context, payment *domain.Payment) {
	s.log.Info("simulating payment success", 
		zap.String("payment_id", payment.PaymentID),
		zap.String("yookassa_id", payment.YookassaPaymentID),
	)
	
	// Update payment status to succeeded
	payment.Status = string(domain.PaymentStatusSucceeded)
	now := time.Now()
	payment.CompletedAt = &now
	
	err := s.storage.UpdatePayment(ctx, payment)
	if err != nil {
		s.log.Error("failed to update simulated payment", zap.Error(err))
		return
	}
	
	// Upgrade user subscription
	err = s.upgradeUserSubscription(ctx, payment)
	if err != nil {
		s.log.Error("failed to upgrade user subscription in simulation", zap.Error(err))
	}
	
	s.log.Info("simulated payment success completed",
		zap.String("payment_id", payment.PaymentID),
		zap.Int64("user_id", payment.UserID),
	)
}