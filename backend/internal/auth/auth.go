package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"familyflix/internal/db"
	"familyflix/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "familyflix_session"
const sessionDuration = 24 * time.Hour

type Session struct {
	Username string
	Role     string
}

func Login(w http.ResponseWriter, r *http.Request, database *db.DB, sessionKey []byte, username, password string) error {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return errors.New("invalid username or password")
	}

	expiresAt := time.Now().Add(sessionDuration)
	value := signSession(sessionKey, username, expiresAt)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
		MaxAge:   int(sessionDuration.Seconds()),
		Expires:  expiresAt,
	})
	return nil
}

func CurrentUser(r *http.Request, database *db.DB, sessionKey []byte) (*models.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	username, err := verifySession(sessionKey, cookie.Value)
	if err != nil {
		return nil, err
	}
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func RequireAuth(database *db.DB, sessionKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := CurrentUser(r, database, sessionKey); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdmin(database *db.DB, sessionKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if user.Role != "admin" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func signSession(key []byte, username string, expiresAt time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(username + "\n" + strconv.FormatInt(expiresAt.Unix(), 10)))
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func verifySession(key []byte, value string) (string, error) {
	encodedUsername, signature, ok := strings.Cut(value, ".")
	if !ok {
		return "", errors.New("invalid session")
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", errors.New("invalid session")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(encodedUsername))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return "", errors.New("invalid session")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedUsername)
	if err != nil {
		return "", errors.New("invalid session")
	}
	username, expiresText, ok := strings.Cut(string(payload), "\n")
	if !ok || username == "" {
		return "", errors.New("invalid session")
	}
	expiresAt, err := strconv.ParseInt(expiresText, 10, 64)
	if err != nil || time.Now().Unix() >= expiresAt {
		return "", errors.New("invalid session")
	}
	return username, nil
}
