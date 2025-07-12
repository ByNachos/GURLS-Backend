package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultBcryptCost стандартная сложность bcrypt
	DefaultBcryptCost = 12
)

var (
	ErrInvalidPassword = errors.New("invalid password")
)

// PasswordService сервис для работы с паролями
type PasswordService struct {
	cost int
}

// NewPasswordService создает новый сервис для работы с паролями
func NewPasswordService() *PasswordService {
	return &PasswordService{
		cost: DefaultBcryptCost,
	}
}

// NewPasswordServiceWithCost создает новый сервис с заданной сложностью
func NewPasswordServiceWithCost(cost int) *PasswordService {
	return &PasswordService{
		cost: cost,
	}
}

// HashPassword хеширует пароль с использованием bcrypt
func (s *PasswordService) HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", ErrInvalidPassword
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", err
	}

	return string(hashedBytes), nil
}

// VerifyPassword проверяет соответствие пароля и хеша
func (s *PasswordService) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// IsValidPassword проверяет валидность пароля по базовым критериям
func IsValidPassword(password string) error {
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters long")
	}
	
	if len(password) > 128 {
		return errors.New("password must be no more than 128 characters long")
	}

	return nil
}