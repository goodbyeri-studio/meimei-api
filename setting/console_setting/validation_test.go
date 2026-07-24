package console_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAnnouncementsOnlyReturnsManuallyPublishedEntries(t *testing.T) {
	settings := GetConsoleSetting()
	original := settings.Announcements
	t.Cleanup(func() {
		settings.Announcements = original
	})

	settings.Announcements = `[
		{"id":1,"content":"legacy entry","publishDate":"2026-07-22T10:00:00Z","type":"default"},
		{"id":2,"content":"draft entry","publishDate":"2026-07-24T10:00:00Z","type":"warning","published":false},
		{"id":3,"content":"published entry","publishDate":"2026-07-23T10:00:00Z","type":"success","published":true}
	]`

	announcements := GetAnnouncements()
	require.Len(t, announcements, 2)
	assert.Equal(t, float64(3), announcements[0]["id"])
	assert.Equal(t, float64(1), announcements[1]["id"])
}

func TestValidateAnnouncementsRejectsInvalidPublishedState(t *testing.T) {
	err := ValidateConsoleSettings(
		`[{"content":"entry","publishDate":"2026-07-24T10:00:00Z","published":"yes"}]`,
		"Announcements",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "发布状态必须为布尔值")
}
