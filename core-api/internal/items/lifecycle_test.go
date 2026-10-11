package items

import (
	"testing"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

// allStatuses exhaustively lists every status so the transition table below
// can be checked for every one of the 9x9 pairs.
var allStatuses = []api.ItemStatus{
	api.InUse, api.Stored, api.Lent, api.ToGiveAway,
	api.Consumed, api.GivenAway, api.Sold, api.Discarded, api.Lost,
}

func TestCanTransition(t *testing.T) {
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			got := CanTransition(from, to)

			want := activeStatuses[from] // a terminal `from` accepts nothing
			if want {
				want = activeStatuses[to] || terminalStatuses[to]
			}

			if got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestCanTransitionTerminalIsOneWay(t *testing.T) {
	for from := range terminalStatuses {
		for _, to := range allStatuses {
			if CanTransition(from, to) {
				t.Errorf("CanTransition(%s, %s) = true, want false: terminal statuses accept no further change", from, to)
			}
		}
	}
}

func TestCanTransitionActiveSetMovesFreely(t *testing.T) {
	for from := range activeStatuses {
		for to := range activeStatuses {
			if !CanTransition(from, to) {
				t.Errorf("CanTransition(%s, %s) = false, want true: the active set moves freely among itself", from, to)
			}
		}
	}
}

func TestIsTerminal(t *testing.T) {
	for _, status := range allStatuses {
		if got, want := IsTerminal(status), terminalStatuses[status]; got != want {
			t.Errorf("IsTerminal(%s) = %v, want %v", status, got, want)
		}
	}
}

func TestIsActive(t *testing.T) {
	for _, status := range allStatuses {
		if got, want := IsActive(status), activeStatuses[status]; got != want {
			t.Errorf("IsActive(%s) = %v, want %v", status, got, want)
		}
	}
}
