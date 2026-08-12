package enums

import "strings"

type Status int16

const (
	StatusUnknown   Status = 0
	StatusActive    Status = 1
	StatusInactive  Status = 2
	StatusPending   Status = 3
	StatusSuspended Status = 4
	StatusBlocked   Status = 5
)

func StatusFromString(value string) (Status, bool) {
	switch strings.ToLower(value) {
	case "active":
		return StatusActive, true
	case "inactive":
		return StatusInactive, true
	case "pending":
		return StatusPending, true
	case "suspended":
		return StatusSuspended, true
	case "blocked":
		return StatusBlocked, true
	default:
		return StatusUnknown, false
	}
}

func (status Status) IsValid() bool {
	switch status {
	case StatusActive,
		StatusInactive,
		StatusPending,
		StatusSuspended,
		StatusBlocked:
		return true
	default:
		return false
	}
}
