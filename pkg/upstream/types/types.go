package types

type ReplicatedCursor struct {
	ChannelID       string
	ChannelName     string
	ChannelSequence int64
}

func (rc1 ReplicatedCursor) Equal(rc2 ReplicatedCursor) bool {
	if rc1.ChannelID != "" && rc2.ChannelID != "" {
		return rc1.ChannelID == rc2.ChannelID && rc1.ChannelSequence == rc2.ChannelSequence
	}
	return rc1.ChannelName == rc2.ChannelName && rc1.ChannelSequence == rc2.ChannelSequence
}

type ChannelRelease struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	ReleaseNotes string `json:"releaseNotes"`
}
