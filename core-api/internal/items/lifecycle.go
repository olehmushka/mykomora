package items

import "github.com/olehmushka/mykomora/core-api/internal/api"

// activeStatuses move freely among themselves: nothing about choosing
// in_use over stored over lent is a one-way decision.
var activeStatuses = map[api.ItemStatus]bool{
	api.InUse:      true,
	api.Stored:     true,
	api.Lent:       true,
	api.ToGiveAway: true,
}

// terminalStatuses are reachable from any active status but accept nothing
// further — SPEC 7's "archive rather than delete" NFR, applied to items.
var terminalStatuses = map[api.ItemStatus]bool{
	api.Consumed:  true,
	api.GivenAway: true,
	api.Sold:      true,
	api.Discarded: true,
	api.Lost:      true,
}

// CanTransition reports whether moving from one status to another is legal.
// A terminal status accepts no further change; any active status may move
// to any other status, active or terminal.
func CanTransition(from, to api.ItemStatus) bool {
	if terminalStatuses[from] {
		return false
	}

	return activeStatuses[to] || terminalStatuses[to]
}

// IsTerminal reports whether reaching a status should also archive the
// item.
func IsTerminal(status api.ItemStatus) bool {
	return terminalStatuses[status]
}

// IsActive reports whether a status is one of the freely-interchangeable
// active set — used to reject a caller-supplied terminal status at create
// time, since an item always starts with a created event before any
// terminal one.
func IsActive(status api.ItemStatus) bool {
	return activeStatuses[status]
}
