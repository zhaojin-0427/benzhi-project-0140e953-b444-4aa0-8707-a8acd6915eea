package audit

import "sort"

type TimelineEntry struct {
	Sequence       uint64 `json:"sequence"`
	EventType      string `json:"eventType"`
	BatchVersion   int    `json:"batchVersion"`
	Actor          string `json:"actor"`
	Role           string `json:"role"`
	Action         string `json:"action"`
	Result         string `json:"result"`
	OccurredAt     string `json:"occurredAt"`
	PreviousDigest string `json:"previousDigest"`
	Digest         string `json:"digest"`
}

func SortTimeline(entries []TimelineEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Sequence < entries[j].Sequence })
}
