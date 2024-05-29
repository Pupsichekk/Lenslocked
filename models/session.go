package models

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/Pupsichekk/lenslocked/rand"
)

const (
	// The minimum number of bytes used for each session token.
	MinBytesPerToken = 32
)

type Session struct {
	ID     int
	UserID int
	// Token is only set when creating a new session.
	// When looking up a session this will be left empty as we only store the hash of token in db
	// and we cannot reverse it into a raw token
	Token     string
	TokenHash string
}

type SessionService struct {
	DB *sql.DB
	// BytesPerToken is used to determine how much bytes
	// shall we use to generate a session token.
	// If specified bytes are less than MinBytesPerToken
	// MinBytesPerToken will be set instead of BytesPerToken.
	BytesPerToken int
}

func (ss *SessionService) Create(userID int) (*Session, error) {
	bytesPerToken := ss.BytesPerToken
	if bytesPerToken < MinBytesPerToken {
		bytesPerToken = MinBytesPerToken
	}
	token, err := rand.String(bytesPerToken)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	session := Session{
		UserID:    userID,
		Token:     token,
		TokenHash: ss.Hash(token),
	}

	// PostgreSql concrete realization
	// Trying to update session token, if fails
	// ErrNoRows err is provided and if ErrNoRows
	// is provided then we can create session token
	row := ss.DB.QueryRow(`
		insert into sessions (user_id, token_hash)
		values ($1, $2) on conflict (user_id) do
		update
		set token_hash = $2
		returning id;`, session.UserID, session.TokenHash)
	err = row.Scan(&session.ID)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	return &session, nil
}

// User method on SessionService requires a token and returns a user
func (ss *SessionService) User(token string) (*User, error) {
	tokenHash := ss.Hash(token)
	var user User
	row := ss.DB.QueryRow(`
		select users.id, users.email, users.password_hash 
		from sessions
		join users on users.id = sessions.user_id
		where sessions.token_hash = $1;`, tokenHash)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}
	return &user, nil
}

func (ss *SessionService) Delete(token string) error {
	tokenHash := ss.Hash(token)
	_, err := ss.DB.Exec(`
	delete from sessions
	where token_hash = $1;`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (ss *SessionService) Hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
