package http

import (
	"GURLS-Backend/internal/auth"
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// SubscriptionHandler handles subscription-related HTTP requests
type SubscriptionHandler struct {
	storage repository.Storage
	log     *zap.Logger
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(storage repository.Storage, log *zap.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{
		storage: storage,
		log:     log,
	}
}

// SubscriptionPlanResponse represents a subscription plan in API responses
type SubscriptionPlanResponse struct {
	ID                     int16   `json:"id"`
	Name                   string  `json:"name"`
	DisplayName            string  `json:"display_name"`
	PriceMonthly           float64 `json:"price_monthly"`
	PriceYearly            float64 `json:"price_yearly"`
	MaxLinksPerMonth       *int    `json:"max_links_per_month,omitempty"`
	MaxClicksPerMonth      *int    `json:"max_clicks_per_month,omitempty"`
	AnalyticsRetentionDays int16   `json:"analytics_retention_days"`
	LinkExpirationDays     *int16  `json:"link_expiration_days,omitempty"`
	CustomAliases          bool    `json:"custom_aliases"`
	PasswordProtectedLinks bool    `json:"password_protected_links"`
	APIAccess              bool    `json:"api_access"`
	CustomDomains          bool    `json:"custom_domains"`
	PrioritySupport        bool    `json:"priority_support"`
	IsActive               bool    `json:"is_active"`
}

// UserSubscriptionResponse represents user's current subscription
type UserSubscriptionResponse struct {
	CurrentPlan       SubscriptionPlanResponse `json:"current_plan"`
	ExpiresAt         *string                  `json:"expires_at,omitempty"`
	LinksUsedThisMonth int64                   `json:"links_used_this_month"`
	ClicksThisMonth    int64                   `json:"clicks_this_month"`
	CanUpgrade         bool                     `json:"can_upgrade"`
	AvailablePlans     []SubscriptionPlanResponse `json:"available_plans"`
}

// UpgradeSubscriptionRequest represents subscription upgrade request
type UpgradeSubscriptionRequest struct {
	NewSubscriptionID int16  `json:"new_subscription_id"`
	BillingCycle      string `json:"billing_cycle"` // "monthly" or "yearly"
}

// UpgradeSubscriptionResponse represents subscription upgrade response
type UpgradeSubscriptionResponse struct {
	Message       string  `json:"message"`
	PaymentID     *string `json:"payment_id,omitempty"`
	PaymentURL    *string `json:"payment_url,omitempty"`
	RequiresPayment bool  `json:"requires_payment"`
}

// ListSubscriptionPlans handles GET /api/subscriptions/plans
func (h *SubscriptionHandler) ListSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	// Get all subscription types
	subscriptionTypes, err := h.storage.ListSubscriptionTypes(r.Context())
	if err != nil {
		h.log.Error("failed to list subscription types", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var plans []SubscriptionPlanResponse
	for _, sub := range subscriptionTypes {
		plan := SubscriptionPlanResponse{
			ID:                     sub.ID,
			Name:                   sub.Name,
			DisplayName:            sub.DisplayName,
			PriceMonthly:           sub.PriceMonthly,
			PriceYearly:            sub.PriceYearly,
			MaxLinksPerMonth:       sub.MaxLinksPerMonth,
			MaxClicksPerMonth:      sub.MaxClicksPerMonth,
			AnalyticsRetentionDays: sub.AnalyticsRetentionDays,
			LinkExpirationDays:     sub.LinkExpirationDays,
			CustomAliases:          sub.CustomAliases,
			PasswordProtectedLinks: sub.PasswordProtectedLinks,
			APIAccess:              sub.APIAccess,
			CustomDomains:          sub.CustomDomains,
			PrioritySupport:        sub.PrioritySupport,
			IsActive:               sub.IsActive,
		}
		plans = append(plans, plan)
	}

	h.writeJSON(w, plans, http.StatusOK)
}

// GetCurrentSubscription handles GET /api/subscriptions/current
func (h *SubscriptionHandler) GetCurrentSubscription(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get user with subscription details
	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get user", zap.Int64("user_id", userID), zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get current subscription type
	subscriptionType, err := h.storage.GetSubscriptionType(r.Context(), user.SubscriptionTypeID)
	if err != nil {
		h.log.Error("failed to get subscription type", zap.Int16("subscription_id", user.SubscriptionTypeID), zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get user stats for usage information
	userStats, err := h.getUserStats(r.Context(), userID)
	if err != nil {
		h.log.Warn("failed to get user stats", zap.Int64("user_id", userID), zap.Error(err))
		// Continue without stats
	}

	// Get all available plans for upgrade options
	allPlans, err := h.storage.ListSubscriptionTypes(r.Context())
	if err != nil {
		h.log.Error("failed to list subscription types", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Prepare current plan response
	currentPlan := SubscriptionPlanResponse{
		ID:                     subscriptionType.ID,
		Name:                   subscriptionType.Name,
		DisplayName:            subscriptionType.DisplayName,
		PriceMonthly:           subscriptionType.PriceMonthly,
		PriceYearly:            subscriptionType.PriceYearly,
		MaxLinksPerMonth:       subscriptionType.MaxLinksPerMonth,
		MaxClicksPerMonth:      subscriptionType.MaxClicksPerMonth,
		AnalyticsRetentionDays: subscriptionType.AnalyticsRetentionDays,
		LinkExpirationDays:     subscriptionType.LinkExpirationDays,
		CustomAliases:          subscriptionType.CustomAliases,
		PasswordProtectedLinks: subscriptionType.PasswordProtectedLinks,
		APIAccess:              subscriptionType.APIAccess,
		CustomDomains:          subscriptionType.CustomDomains,
		PrioritySupport:        subscriptionType.PrioritySupport,
		IsActive:               subscriptionType.IsActive,
	}

	// Prepare available plans (excluding current plan)
	var availablePlans []SubscriptionPlanResponse
	canUpgrade := false
	for _, plan := range allPlans {
		if plan.ID != user.SubscriptionTypeID {
			planResp := SubscriptionPlanResponse{
				ID:                     plan.ID,
				Name:                   plan.Name,
				DisplayName:            plan.DisplayName,
				PriceMonthly:           plan.PriceMonthly,
				PriceYearly:            plan.PriceYearly,
				MaxLinksPerMonth:       plan.MaxLinksPerMonth,
				MaxClicksPerMonth:      plan.MaxClicksPerMonth,
				AnalyticsRetentionDays: plan.AnalyticsRetentionDays,
				LinkExpirationDays:     plan.LinkExpirationDays,
				CustomAliases:          plan.CustomAliases,
				PasswordProtectedLinks: plan.PasswordProtectedLinks,
				APIAccess:              plan.APIAccess,
				CustomDomains:          plan.CustomDomains,
				PrioritySupport:        plan.PrioritySupport,
				IsActive:               plan.IsActive,
			}
			availablePlans = append(availablePlans, planResp)
			
			// Check if this is an upgrade (higher price)
			if plan.PriceMonthly > subscriptionType.PriceMonthly {
				canUpgrade = true
			}
		}
	}

	// Prepare response
	response := UserSubscriptionResponse{
		CurrentPlan:        currentPlan,
		LinksUsedThisMonth: 0, // Default value
		ClicksThisMonth:    0, // Default value
		CanUpgrade:         canUpgrade,
		AvailablePlans:     availablePlans,
	}

	// Add expiration date if available
	if user.SubscriptionExpiresAt != nil {
		expiresAt := user.SubscriptionExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &expiresAt
	}

	// Add usage stats if available
	if userStats != nil {
		response.LinksUsedThisMonth = int64(userStats.LinksCreatedThisMonth)
		response.ClicksThisMonth = int64(userStats.ClicksReceivedThisMonth)
	}

	h.writeJSON(w, response, http.StatusOK)
}

// UpgradeSubscription handles POST /api/subscriptions/upgrade
func (h *SubscriptionHandler) UpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req UpgradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Debug("invalid upgrade subscription request", zap.Error(err))
		h.writeError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.NewSubscriptionID == 0 {
		h.writeError(w, "new_subscription_id is required", http.StatusBadRequest)
		return
	}
	if req.BillingCycle != "monthly" && req.BillingCycle != "yearly" {
		req.BillingCycle = "monthly" // default
	}

	// Get user current subscription
	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get user", zap.Int64("user_id", userID), zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if it's actually an upgrade
	if req.NewSubscriptionID == user.SubscriptionTypeID {
		h.writeError(w, "User already has this subscription", http.StatusBadRequest)
		return
	}

	// Get target subscription type
	targetSubscription, err := h.storage.GetSubscriptionType(r.Context(), req.NewSubscriptionID)
	if err != nil {
		if err == repository.ErrSubscriptionTypeNotFound {
			h.writeError(w, "Invalid subscription type", http.StatusBadRequest)
			return
		}
		h.log.Error("failed to get target subscription", zap.Error(err))
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if downgrading to free plan
	if targetSubscription.PriceMonthly == 0 {
		// Free downgrade - no payment required
		err = h.performFreeDowngrade(r.Context(), user, targetSubscription)
		if err != nil {
			h.log.Error("failed to perform free downgrade", zap.Error(err))
			h.writeError(w, "Failed to downgrade subscription", http.StatusInternalServerError)
			return
		}

		response := UpgradeSubscriptionResponse{
			Message:         "Subscription downgraded successfully",
			RequiresPayment: false,
		}
		h.writeJSON(w, response, http.StatusOK)
		return
	}

	// Paid upgrade - return payment information
	// For now, we'll just return a message indicating payment is required
	response := UpgradeSubscriptionResponse{
		Message:         "Payment required for subscription upgrade. Please use the payment API to complete the upgrade.",
		RequiresPayment: true,
	}

	h.writeJSON(w, response, http.StatusOK)
}

// Helper methods

func (h *SubscriptionHandler) getUserStats(ctx context.Context, userID int64) (*domain.UserStats, error) {
	// This is a placeholder - in a real implementation, you'd have a method to get user stats
	// For now, we'll return nil and handle it gracefully in the caller
	return nil, nil
}

func (h *SubscriptionHandler) performFreeDowngrade(ctx context.Context, user *domain.User, targetSubscription *domain.SubscriptionType) error {
	// Create subscription change record
	subscriptionChange := &domain.SubscriptionChange{
		UserID:            user.ID,
		OldSubscriptionID: &user.SubscriptionTypeID,
		NewSubscriptionID: targetSubscription.ID,
		ChangeType:        string(domain.SubscriptionChangeTypeDowngrade),
		EffectiveDate:     time.Now(),
		IsActive:          true,
	}

	err := h.storage.CreateSubscriptionChange(ctx, subscriptionChange)
	if err != nil {
		return err
	}

	// Update user subscription
	user.SubscriptionTypeID = targetSubscription.ID
	// Free plan doesn't expire
	user.SubscriptionExpiresAt = nil

	err = h.storage.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	h.log.Info("subscription downgraded to free",
		zap.Int64("user_id", user.ID),
		zap.Int16("old_subscription", *subscriptionChange.OldSubscriptionID),
		zap.Int16("new_subscription", targetSubscription.ID),
	)

	return nil
}

func (h *SubscriptionHandler) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *SubscriptionHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}