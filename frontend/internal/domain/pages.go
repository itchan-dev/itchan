package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

type BoardWithAccess struct {
	domain.Board
	Accessible bool
}

type IndexPageData struct {
	PublicBoards    []domain.Board
	CorporateBoards []BoardWithAccess
}

type BlacklistedUsers struct {
	Users []domain.BlacklistEntry
	Page  int
}

type RefStatsRow struct {
	Source string
	Counts []int
}

type RefStatsPivot struct {
	Actions []string
	Rows    []RefStatsRow
}

// PivotRefStats converts flat source/action/count rows into a pivot table.
func PivotRefStats(stats []domain.ReferralActionStats) *RefStatsPivot {
	if len(stats) == 0 {
		return nil
	}

	actionIndex := map[string]int{}
	var actions []string
	sourceIndex := map[string]int{}
	var sources []string

	for _, s := range stats {
		if _, ok := actionIndex[s.Action]; !ok {
			actionIndex[s.Action] = len(actions)
			actions = append(actions, s.Action)
		}
		if _, ok := sourceIndex[s.Source]; !ok {
			sourceIndex[s.Source] = len(sources)
			sources = append(sources, s.Source)
		}
	}

	rows := make([]RefStatsRow, len(sources))
	for i, src := range sources {
		rows[i] = RefStatsRow{Source: src, Counts: make([]int, len(actions))}
	}

	for _, s := range stats {
		rows[sourceIndex[s.Source]].Counts[actionIndex[s.Action]] = s.Count
	}

	return &RefStatsPivot{Actions: actions, Rows: rows}
}

type AdminPageData struct {
	Blacklisted BlacklistedUsers
	RefStats    *RefStatsPivot
}
