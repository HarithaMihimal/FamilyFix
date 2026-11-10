package auth

import (
	"errors"
	"net/http"
	"time"

	"familyflix/internal/db"
	"familyflix/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "familyflix_session"

type Session struct {
	Username string
	Role     string
}

func Login(w http.ResponseWriter, r *http.Request, database *db.DB, username, password string) error {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return errors.New("invalid username or password")
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    username,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	return nil
}

func CurrentUser(r *http.Request, database *db.DB) (*models.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	user, err := database.GetUserByUsername(cookie.Value)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func RequireAuth(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := CurrentUser(r, database); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdmin(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := CurrentUser(r, database)
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

