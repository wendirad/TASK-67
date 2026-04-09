package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(`
		SELECT id, username, password_hash, role, display_name, email, phone, status,
		       failed_login_attempts, locked_until, created_at, updated_at
		FROM users WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.DisplayName,
		&user.Email, &user.Phone, &user.Status, &user.FailedLoginAttempts,
		&user.LockedUntil, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return user, nil
}

func (r *UserRepository) FindByID(id string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(`
		SELECT id, username, password_hash, role, display_name, email, phone, status,
		       failed_login_attempts, locked_until, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.DisplayName,
		&user.Email, &user.Phone, &user.Status, &user.FailedLoginAttempts,
		&user.LockedUntil, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

func (r *UserRepository) IncrementFailedAttempts(userID string) error {
	_, err := r.db.Exec(`
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= 5 THEN NOW() + INTERVAL '15 minutes'
		        ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

func (r *UserRepository) ResetFailedAttempts(userID string) error {
	_, err := r.db.Exec(`
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

func (r *UserRepository) UpdatePasswordHash(userID, hash string) error {
	_, err := r.db.Exec(`
		UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2
	`, hash, userID)
	return err
}

func (r *UserRepository) Create(user *models.User) error {
	return r.db.QueryRow(`
		INSERT INTO users (username, password_hash, role, display_name, email, phone, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, user.Username, user.PasswordHash, user.Role, user.DisplayName,
		user.Email, user.Phone, user.Status,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) UsernameExists(username string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	return exists, err
}

func (r *UserRepository) UpdateStatus(userID, status string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(`
		UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2
		RETURNING id, username, role, display_name, email, phone, status, created_at, updated_at
	`, status, userID).Scan(
		&user.ID, &user.Username, &user.Role, &user.DisplayName,
		&user.Email, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update user status: %w", err)
	}
	return user, nil
}

func (r *UserRepository) ListUsers(page, pageSize int, role, status, search string) ([]models.User, int, error) {
	baseQuery := `FROM users WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if role != "" {
		baseQuery += fmt.Sprintf(` AND role = $%d`, argIdx)
		args = append(args, role)
		argIdx++
	}
	if status != "" {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if search != "" {
		baseQuery += fmt.Sprintf(` AND (username ILIKE $%d OR display_name ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	var total int
	countQuery := `SELECT COUNT(*) ` + baseQuery
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, username, role, display_name, email, phone, status, created_at, updated_at
		%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Role, &u.DisplayName,
			&u.Email, &u.Phone, &u.Status, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, total, rows.Err()
}
