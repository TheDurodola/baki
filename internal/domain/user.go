package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleStaff  Role = "staff"
	RoleMember Role = "member"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusInactive  UserStatus = "inactive"
)

type Relationship string

const (
	RelationshipSpouse    Relationship = "spouse"
	RelationshipParent    Relationship = "parent"
	RelationshipSibling   Relationship = "sibling"
	RelationshipChild     Relationship = "child"
	RelationshipFriend    Relationship = "friend"
	RelationshipRelative  Relationship = "relative"
	RelationshipColleague Relationship = "colleague"
	RelationshipOther     Relationship = "other"
)

func (r Relationship) IsValid() bool {
	switch r {
	case RelationshipSpouse, RelationshipParent, RelationshipSibling, RelationshipChild,
		RelationshipFriend, RelationshipRelative, RelationshipColleague, RelationshipOther:
		return true
	default:
		return false
	}
}

type EmergencyContact struct {
	Name         string       `json:"name" bson:"name"`
	Phone        string       `json:"phone" bson:"phone"`
	Relationship Relationship `json:"relationship" bson:"relationship"`
}

type User struct {
	ID               uuid.UUID        `json:"id" bson:"_id"`
	MemberCode       string           `json:"member_code" bson:"member_code"`
	Email            string           `json:"email" bson:"email"`
	Phone            string           `json:"phone" bson:"phone"`
	PasswordHash     string           `json:"-" bson:"password_hash"`
	FirstName        string           `json:"first_name" bson:"first_name"`
	LastName         string           `json:"last_name" bson:"last_name"`
	AvatarURL        string           `json:"avatar_url,omitempty" bson:"avatar_url,omitempty"`
	EmergencyContact EmergencyContact `json:"emergency_contact" bson:"emergency_contact"`
	Role             Role             `json:"role" bson:"role"`
	Status           UserStatus       `json:"status" bson:"status"`
	CreatedAt        time.Time        `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at" bson:"updated_at"`
}
