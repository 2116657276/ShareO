package model

// Post status constants
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// Feed sort constants
const (
	SortLatest = "latest"
	SortHot    = "hot"
)

// User role constants
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User status constants
const (
	UserStatusBanned int8 = 0
	UserStatusActive int8 = 1
)
