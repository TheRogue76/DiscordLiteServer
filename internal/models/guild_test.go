package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Guild Tests
// ============================================================================

func TestGuild_Creation(t *testing.T) {
	now := time.Now()
	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           "My Awesome Server",
		Icon:           sql.NullString{String: "icon_hash_123", Valid: true},
		OwnerID:        sql.NullString{String: "owner_id_456", Valid: true},
		Permissions:    2147483647, // Administrator permissions
		Features:       pq.StringArray{"ANIMATED_ICON", "BANNER", "COMMUNITY"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	assert.Equal(t, int64(1), guild.ID)
	assert.Equal(t, "1234567890", guild.DiscordGuildID)
	assert.Equal(t, "My Awesome Server", guild.Name)
	assert.True(t, guild.Icon.Valid)
	assert.Equal(t, "icon_hash_123", guild.Icon.String)
	assert.True(t, guild.OwnerID.Valid)
	assert.Equal(t, "owner_id_456", guild.OwnerID.String)
	assert.Equal(t, int64(2147483647), guild.Permissions)
	assert.Len(t, guild.Features, 3)
	assert.Contains(t, guild.Features, "ANIMATED_ICON")
	assert.Contains(t, guild.Features, "BANNER")
	assert.Contains(t, guild.Features, "COMMUNITY")
}

func TestGuild_WithoutIcon(t *testing.T) {
	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           "Simple Server",
		Icon:           sql.NullString{Valid: false}, // No icon
		OwnerID:        sql.NullString{String: "owner123", Valid: true},
		Permissions:    0,
		Features:       pq.StringArray{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.False(t, guild.Icon.Valid, "Guild may not have icon")
}

func TestGuild_WithoutOwner(t *testing.T) {
	// In some rare cases, owner might not be known
	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           "Server",
		Icon:           sql.NullString{Valid: false},
		OwnerID:        sql.NullString{Valid: false}, // No owner ID
		Permissions:    0,
		Features:       pq.StringArray{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.False(t, guild.OwnerID.Valid, "Guild may not have owner ID in some cases")
}

func TestGuild_WithoutFeatures(t *testing.T) {
	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           "Basic Server",
		Icon:           sql.NullString{Valid: false},
		OwnerID:        sql.NullString{String: "owner123", Valid: true},
		Permissions:    0,
		Features:       pq.StringArray{}, // No features
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Empty(t, guild.Features, "Guild may have no features")
}

func TestGuild_CommonFeatures(t *testing.T) {
	tests := []struct {
		name     string
		features []string
	}{
		{
			"Community server",
			[]string{"COMMUNITY", "NEWS", "WELCOME_SCREEN_ENABLED"},
		},
		{
			"Partnered server",
			[]string{"PARTNERED", "VANITY_URL", "INVITE_SPLASH"},
		},
		{
			"Verified server",
			[]string{"VERIFIED", "DISCOVERABLE"},
		},
		{
			"Boosted server (Level 3)",
			[]string{"ANIMATED_ICON", "ANIMATED_BANNER", "BANNER", "VANITY_URL"},
		},
		{
			"Server with threads",
			[]string{"THREADS_ENABLED", "THREE_DAY_THREAD_ARCHIVE", "SEVEN_DAY_THREAD_ARCHIVE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guild := &Guild{
				ID:             1,
				DiscordGuildID: "1234567890",
				Name:           "Test Server",
				Icon:           sql.NullString{String: "icon", Valid: true},
				OwnerID:        sql.NullString{String: "owner", Valid: true},
				Permissions:    0,
				Features:       pq.StringArray(tt.features),
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			assert.Len(t, guild.Features, len(tt.features))
			for _, feature := range tt.features {
				assert.Contains(t, guild.Features, feature)
			}
		})
	}
}

func TestGuild_AllFeatures(t *testing.T) {
	// Test with many Discord features
	allFeatures := []string{
		"ANIMATED_ICON",
		"BANNER",
		"COMMERCE",
		"COMMUNITY",
		"DISCOVERABLE",
		"FEATURABLE",
		"INVITE_SPLASH",
		"MEMBER_VERIFICATION_GATE_ENABLED",
		"NEWS",
		"PARTNERED",
		"PREVIEW_ENABLED",
		"VANITY_URL",
		"VERIFIED",
		"VIP_REGIONS",
		"WELCOME_SCREEN_ENABLED",
		"TICKETED_EVENTS_ENABLED",
		"MONETIZATION_ENABLED",
		"MORE_STICKERS",
		"THREE_DAY_THREAD_ARCHIVE",
		"SEVEN_DAY_THREAD_ARCHIVE",
		"PRIVATE_THREADS",
		"ROLE_ICONS",
	}

	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           "Feature-Rich Server",
		Icon:           sql.NullString{String: "icon", Valid: true},
		OwnerID:        sql.NullString{String: "owner", Valid: true},
		Permissions:    2147483647,
		Features:       pq.StringArray(allFeatures),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Len(t, guild.Features, len(allFeatures))
	for _, feature := range allFeatures {
		assert.Contains(t, guild.Features, feature)
	}
}

func TestGuild_Permissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions int64
		description string
	}{
		{"No permissions", 0, "User has no permissions"},
		{"Read messages", 1024, "VIEW_CHANNEL permission"},
		{"Send messages", 2048, "SEND_MESSAGES permission"},
		{"Administrator", 2147483647, "All permissions (Administrator)"},
		{"Moderate members", 1099511627775, "Full moderation permissions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guild := &Guild{
				ID:             1,
				DiscordGuildID: "1234567890",
				Name:           "Test Server",
				OwnerID:        sql.NullString{String: "owner", Valid: true},
				Permissions:    tt.permissions,
				Features:       pq.StringArray{},
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			assert.Equal(t, tt.permissions, guild.Permissions)
		})
	}
}

func TestGuild_LongName(t *testing.T) {
	// Discord allows guild names up to 100 characters
	longName := "This Is A Very Long Guild Name That Discord Allows Up To One Hundred Characters For Server Names Wow"

	guild := &Guild{
		ID:             1,
		DiscordGuildID: "1234567890",
		Name:           longName,
		OwnerID:        sql.NullString{String: "owner", Valid: true},
		Permissions:    0,
		Features:       pq.StringArray{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, longName, guild.Name)
	assert.LessOrEqual(t, len(guild.Name), 100, "Guild name should be within Discord limits")
}

// ============================================================================
// UserGuild Tests
// ============================================================================

func TestUserGuild_Creation(t *testing.T) {
	now := time.Now()
	userGuild := &UserGuild{
		ID:        1,
		UserID:    100,
		GuildID:   200,
		JoinedAt:  now.Add(-30 * 24 * time.Hour), // Joined 30 days ago
		CreatedAt: now,
	}

	assert.Equal(t, int64(1), userGuild.ID)
	assert.Equal(t, int64(100), userGuild.UserID)
	assert.Equal(t, int64(200), userGuild.GuildID)
	assert.True(t, userGuild.JoinedAt.Before(now))
}

func TestUserGuild_NewMember(t *testing.T) {
	now := time.Now()
	userGuild := &UserGuild{
		ID:        1,
		UserID:    100,
		GuildID:   200,
		JoinedAt:  now, // Just joined
		CreatedAt: now,
	}

	timeSinceJoin := time.Since(userGuild.JoinedAt)
	assert.Less(t, timeSinceJoin, 1*time.Minute, "User just joined")
}

func TestUserGuild_LongTimeMember(t *testing.T) {
	now := time.Now()
	userGuild := &UserGuild{
		ID:        1,
		UserID:    100,
		GuildID:   200,
		JoinedAt:  now.Add(-365 * 24 * time.Hour), // Joined 1 year ago
		CreatedAt: now.Add(-365 * 24 * time.Hour),
	}

	timeSinceJoin := time.Since(userGuild.JoinedAt)
	assert.Greater(t, timeSinceJoin, 364*24*time.Hour, "User has been member for over a year")
}

func TestUserGuild_MultipleGuilds(t *testing.T) {
	// Test a user belonging to multiple guilds
	userID := int64(100)
	guilds := []int64{200, 201, 202, 203, 204}

	for i, guildID := range guilds {
		userGuild := &UserGuild{
			ID:        int64(i + 1),
			UserID:    userID,
			GuildID:   guildID,
			JoinedAt:  time.Now().Add(-time.Duration(i*7) * 24 * time.Hour), // Different join times
			CreatedAt: time.Now(),
		}

		assert.Equal(t, userID, userGuild.UserID)
		assert.Equal(t, guildID, userGuild.GuildID)
	}
}

func TestUserGuild_MultipleUsersInGuild(t *testing.T) {
	// Test multiple users in the same guild
	guildID := int64(200)
	userIDs := []int64{100, 101, 102, 103, 104}

	for i, userID := range userIDs {
		userGuild := &UserGuild{
			ID:        int64(i + 1),
			UserID:    userID,
			GuildID:   guildID,
			JoinedAt:  time.Now(),
			CreatedAt: time.Now(),
		}

		assert.Equal(t, userID, userGuild.UserID)
		assert.Equal(t, guildID, userGuild.GuildID)
	}
}

func TestUserGuild_JoinedAtBeforeCreatedAt(t *testing.T) {
	// JoinedAt is the Discord join time, CreatedAt is when we stored it
	// JoinedAt can be before CreatedAt
	discordJoinTime := time.Now().Add(-7 * 24 * time.Hour) // Joined Discord guild 7 days ago
	databaseCreatedTime := time.Now()                      // Just stored in our database

	userGuild := &UserGuild{
		ID:        1,
		UserID:    100,
		GuildID:   200,
		JoinedAt:  discordJoinTime,
		CreatedAt: databaseCreatedTime,
	}

	assert.True(t, userGuild.JoinedAt.Before(userGuild.CreatedAt),
		"Discord join time can be before database storage time")
}

func TestUserGuild_SameUserMultipleTimesInDifferentGuilds(t *testing.T) {
	// A user can be in many guilds
	userID := int64(100)
	numberOfGuilds := 100 // Discord allows users to be in up to 100 guilds

	for i := 0; i < numberOfGuilds; i++ {
		userGuild := &UserGuild{
			ID:        int64(i + 1),
			UserID:    userID,
			GuildID:   int64(200 + i),
			JoinedAt:  time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			CreatedAt: time.Now(),
		}

		assert.Equal(t, userID, userGuild.UserID, "User ID should be consistent")
		assert.Equal(t, int64(200+i), userGuild.GuildID, "Guild ID should be unique for each membership")
	}
}
