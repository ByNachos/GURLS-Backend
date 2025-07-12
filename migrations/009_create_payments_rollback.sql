-- 009_create_payments_rollback.sql
-- Rollback payments and subscription_changes tables

-- Drop indexes
DROP INDEX IF EXISTS idx_subscription_changes_change_type;
DROP INDEX IF EXISTS idx_subscription_changes_is_active;
DROP INDEX IF EXISTS idx_subscription_changes_expiration_date;
DROP INDEX IF EXISTS idx_subscription_changes_effective_date;
DROP INDEX IF EXISTS idx_subscription_changes_user_id;

DROP INDEX IF EXISTS idx_payments_created_at;
DROP INDEX IF EXISTS idx_payments_status;
DROP INDEX IF EXISTS idx_payments_yookassa_payment_id;
DROP INDEX IF EXISTS idx_payments_payment_id;
DROP INDEX IF EXISTS idx_payments_user_id;

-- Drop tables
DROP TABLE IF EXISTS subscription_changes;
DROP TABLE IF EXISTS payments;