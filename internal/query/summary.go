package query

import "time"

// Summary is an aggregated view of journal events over a time window.
type Summary struct {
	TimeFrom time.Time      `json:"time_from"`
	TimeTo   time.Time      `json:"time_to"`
	Total    int            `json:"total"`
	ByType   map[string]int `json:"by_type"`
	Notable  []Event        `json:"notable"`
}
