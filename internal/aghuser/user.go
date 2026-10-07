// Package aghuser contains types and logic for dealing with AdGuard Home's web
// users.
package aghuser

import (
	"fmt"

	"github.com/google/uuid"
)

// UserID is the type for the unique IDs of web users.
type UserID uuid.UUID

// NewUserID returns a new web user unique identifier.  Any error returned is an
// error from the cryptographic randomness reader.
func NewUserID() (uid UserID, err error) {
	uuidv7, err := uuid.NewV7()

	return UserID(uuidv7), err
}

// MustNewUserID is a wrapper around [NewUserID] that panics if there is an
// error.  It is currently only used in tests.
func MustNewUserID() (uid UserID) {
	uid, err := NewUserID()
	if err != nil {
		panic(fmt.Errorf("unexpected uuidv7 error: %w", err))
	}

	return uid
}

// User represents a web user.
type User struct {
	// Password stores the password information for the web user.  It must not
	// be nil.
	Password Password

	// Login is the login name of the web user.  It must not be empty.
	Login Login

	// Role is the role of the web user.  Empty means a full administrator;
	// see [RoleTeacher].
	Role string

	// LabID is the ID of the lab a [RoleTeacher] user is restricted to.  It is
	// empty for administrators.
	LabID string

	// ID is the unique identifier for the web user.  It must not be empty.
	ID UserID
}

// RoleTeacher is the role of a restricted user that can only use the lab
// controls of the single lab assigned to them.
const RoleTeacher = "teacher"

// IsTeacher returns true if u is a restricted teacher user.
func (u *User) IsTeacher() (ok bool) {
	return u != nil && u.Role == RoleTeacher
}
