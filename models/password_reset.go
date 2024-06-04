package models

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pupsichekk/lenslocked/rand"
)

const (
	DefaultResetDuration = 1 * time.Hour
)

type PasswordReset struct {
	ID     int
	UserID int

	TokenHash string
	ExpiresAt time.Time
}

type PasswordResetService struct {
	DB *sql.DB
	// BytesPerToken is used to determine how much bytes
	// shall we use to generate a session token.
	// If specified bytes are less than MinBytesPerToken
	// MinBytesPerToken will be set instead of BytesPerToken.
	BytesPerToken int
	// Duration is the amount of time that a PasswordReset is valid for.
	// Defaults to DefaultResetDuration
	Duration time.Duration
}

func (service *PasswordResetService) Create(email string) (*PasswordReset, error) {
	email = strings.ToLower(email)
	var userID int
	row := service.DB.QueryRow(`SELECT id FROM users WHERE email = $1`, email)
	err := row.Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user reset service: %w", err)
	}

	// Build the password reset
	bytesPerToken := service.BytesPerToken
	if bytesPerToken < MinBytesPerToken {
		bytesPerToken = MinBytesPerToken
	}
	token, err := rand.String(bytesPerToken)
	if err != nil {
		return nil, fmt.Errorf("create reset token: %w", err)
	}
	duration := service.Duration
	if duration <= 0 {
		duration = DefaultResetDuration
	}
	pwReset := PasswordReset{
		UserID:    userID,
		TokenHash: service.Hash(token),
		ExpiresAt: time.Now().Add(duration),
	}

	row = service.DB.QueryRow(`
	INSERT INTO password_resets (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3) ON CONFLICT (user_id) DO
	UPDATE 
	SET token_hash = $2, expires_at = $3
	RETURNING id;`, pwReset.UserID, pwReset.TokenHash, pwReset.ExpiresAt)
	err = row.Scan(&pwReset.ID)
	if err != nil {
		return nil, fmt.Errorf("insert user password reset %w", err)
	}
	return &pwReset, nil
}

func (service *PasswordResetService) CheckTokenExpired(tokenHash string) error {
	var count int
	var expiresAt time.Time
	err := service.DB.QueryRow(`
	SELECT COUNT(*), password_resets.expires_at
	FROM password_resets
		JOIN users on users.id = password_resets.user_id
	WHERE password_resets.token_hash = $1`, tokenHash).Scan(&count, &expiresAt)
	if err != nil {
		return fmt.Errorf("error checking token existence: %w", err)
	}
	if count == 0 && time.Now().After(expiresAt) {
		return fmt.Errorf("link is invalid or has expired: %w", err)
	}
	return nil
}

func (service *PasswordResetService) Consume(tokenHash string) (*User, error) {
	// 1: Validate token, shouldn't be expired.
	// 2: Get user information
	// 3: Delete token
	var user User
	var pwReset PasswordReset
	row := service.DB.QueryRow(`
	SELECT password_resets.id,
		password_resets.expires_at,
		users.id,
		users.email,
		users.password_hash
	FROM password_resets
		JOIN users on users.id = password_resets.user_id
	WHERE password_resets.token_hash = $1`, tokenHash)
	err := row.Scan(&pwReset.ID, &pwReset.ExpiresAt, &user.ID,
		&user.Email, &user.PasswordHash)
	if err != nil {
		return nil, ErrNotFound
	}
	if time.Now().After(pwReset.ExpiresAt) {
		return nil, fmt.Errorf("token expired: %v", tokenHash)
	}
	err = service.delete(pwReset.ID)
	if err != nil {
		return nil, fmt.Errorf("consume: %w", err)
	}
	return &user, nil
}

func (service *PasswordResetService) Hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}

func (service *PasswordResetService) delete(id int) error {
	_, err := service.DB.Exec(`
	DELETE FROM password_resets
	WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("err: %w", err)
	}
	return nil
}
