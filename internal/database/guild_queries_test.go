package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Helper Functions
// ============================================================================

func generateGuild(discordGuildID string) *models.Guild {
	return &models.Guild{
		DiscordGuildID: discordGuildID,
		Name:           "Test Guild " + discordGuildID,
		Icon:           sql.NullString{String: "icon_hash_" + discordGuildID, Valid: true},
		OwnerID:        sql.NullString{String: "owner_" + discordGuildID, Valid: true},
		Permissions:    2147483647, // All permissions
		Features:       pq.StringArray{"COMMUNITY", "DISCOVERABLE"},
	}
}

func assertGuildEqual(t *testing.T, expected, actual *models.Guild) {
	t.Helper()
	assert.Equal(t, expected.DiscordGuildID, actual.DiscordGuildID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Icon, actual.Icon)
	assert.Equal(t, expected.OwnerID, actual.OwnerID)
	assert.Equal(t, expected.Permissions, actual.Permissions)
	assert.ElementsMatch(t, expected.Features, actual.Features)
}

// ============================================================================
// Guild CRUD Tests
// ============================================================================

func TestCreateOrUpdateGuild_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	guild := generateGuild("123456789")

	err = db.CreateOrUpdateGuild(ctx, guild)

	require.NoError(t, err)
	assert.NotZero(t, guild.ID)
	assert.NotZero(t, guild.CreatedAt)
	assert.NotZero(t, guild.UpdatedAt)
	assert.WithinDuration(t, time.Now(), guild.CreatedAt, 2*time.Second)
}

func TestCreateOrUpdateGuild_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create initial guild
	guild1 := generateGuild("123456789")
	guild1.Name = "Original Name"
	guild1.Features = pq.StringArray{"COMMUNITY"}
	err = db.CreateOrUpdateGuild(ctx, guild1)
	require.NoError(t, err)

	originalID := guild1.ID
	originalCreatedAt := guild1.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Upsert with same discord_guild_id but different data
	guild2 := generateGuild("123456789")
	guild2.Name = "Updated Name"
	guild2.Features = pq.StringArray{"COMMUNITY", "DISCOVERABLE", "VERIFIED"}
	err = db.CreateOrUpdateGuild(ctx, guild2)
	require.NoError(t, err)

	// ID should remain the same (upsert, not duplicate)
	assert.Equal(t, originalID, guild2.ID)

	// Created_at should not change
	assert.WithinDuration(t, originalCreatedAt, guild2.CreatedAt, 1*time.Second)

	// Updated_at should be newer
	assert.True(t, guild2.UpdatedAt.After(guild2.CreatedAt))

	// Verify updated data in database
	retrieved, err := db.GetGuildByDiscordID(ctx, "123456789")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.ElementsMatch(t, pq.StringArray{"COMMUNITY", "DISCOVERABLE", "VERIFIED"}, retrieved.Features)
}

func TestGetGuildByID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("123456789")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Retrieve guild
	retrieved, err := db.GetGuildByID(ctx, guild.ID)

	require.NoError(t, err)
	assertGuildEqual(t, guild, retrieved)
}

func TestGetGuildByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	guild, err := db.GetGuildByID(ctx, 99999)

	assert.Error(t, err)
	assert.Nil(t, guild)
	assert.Contains(t, err.Error(), "guild not found")
}

func TestGetGuildByDiscordID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("123456789")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Retrieve guild
	retrieved, err := db.GetGuildByDiscordID(ctx, "123456789")

	require.NoError(t, err)
	assertGuildEqual(t, guild, retrieved)
}

func TestGetGuildByDiscordID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	guild, err := db.GetGuildByDiscordID(ctx, "nonexistent_guild_id")

	assert.Error(t, err)
	assert.Nil(t, guild)
	assert.Contains(t, err.Error(), "guild not found")
}

func TestDeleteGuild_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("123456789")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Delete guild
	err = db.DeleteGuild(ctx, guild.ID)
	require.NoError(t, err)

	// Verify guild is gone
	retrieved, err := db.GetGuildByID(ctx, guild.ID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestDeleteGuild_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteGuild(ctx, 99999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "guild not found")
}

// ============================================================================
// User-Guild Relationship Tests
// ============================================================================

func TestCreateUserGuild_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create relationship
	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Verify relationship exists
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, guild.DiscordGuildID)
	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestCreateUserGuild_Idempotent(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create relationship twice (should not error due to ON CONFLICT DO NOTHING)
	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err, "Second insert should not fail (idempotent)")
}

func TestGetGuildsByUserID_Empty(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user with no guilds
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Get guilds (should be empty)
	guilds, err := db.GetGuildsByUserID(ctx, user.ID)

	require.NoError(t, err)
	assert.Empty(t, guilds)
}

func TestGetGuildsByUserID_SingleGuild(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Get guilds
	guilds, err := db.GetGuildsByUserID(ctx, user.ID)

	require.NoError(t, err)
	assert.Len(t, guilds, 1)
	assertGuildEqual(t, guild, guilds[0])
}

func TestGetGuildsByUserID_MultipleGuilds(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create 3 guilds
	guild1 := generateGuild("guild1")
	guild1.Name = "Alpha Guild"
	err = db.CreateOrUpdateGuild(ctx, guild1)
	require.NoError(t, err)

	guild2 := generateGuild("guild2")
	guild2.Name = "Beta Guild"
	err = db.CreateOrUpdateGuild(ctx, guild2)
	require.NoError(t, err)

	guild3 := generateGuild("guild3")
	guild3.Name = "Gamma Guild"
	err = db.CreateOrUpdateGuild(ctx, guild3)
	require.NoError(t, err)

	// Add user to guilds
	err = db.CreateUserGuild(ctx, user.ID, guild1.ID)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild2.ID)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild3.ID)
	require.NoError(t, err)

	// Get guilds (should be ordered by name ASC)
	guilds, err := db.GetGuildsByUserID(ctx, user.ID)

	require.NoError(t, err)
	assert.Len(t, guilds, 3)
	assert.Equal(t, "Alpha Guild", guilds[0].Name)
	assert.Equal(t, "Beta Guild", guilds[1].Name)
	assert.Equal(t, "Gamma Guild", guilds[2].Name)
}

func TestDeleteUserGuild_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Delete relationship
	err = db.DeleteUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Verify relationship is gone
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, guild.DiscordGuildID)
	require.NoError(t, err)
	assert.False(t, hasAccess)
}

func TestDeleteUserGuild_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteUserGuild(ctx, 99999, 99999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user-guild relationship not found")
}

// ============================================================================
// Access Control Tests
// ============================================================================

func TestUserHasGuildAccess_True(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Check access
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, guild.DiscordGuildID)

	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestUserHasGuildAccess_False(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild (no relationship)
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Check access (should be false)
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, guild.DiscordGuildID)

	require.NoError(t, err)
	assert.False(t, hasAccess)
}

func TestUserHasGuildAccess_NonexistentUser(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Check access for nonexistent user
	hasAccess, err := db.UserHasGuildAccess(ctx, 99999, guild.DiscordGuildID)

	require.NoError(t, err)
	assert.False(t, hasAccess)
}

func TestUserHasGuildAccess_NonexistentGuild(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Check access for nonexistent guild
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, "nonexistent_guild")

	require.NoError(t, err)
	assert.False(t, hasAccess)
}

// ============================================================================
// Cascade Delete Tests
// ============================================================================

func TestDeleteGuild_CascadesUserGuilds(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and guild
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Delete guild (should cascade to user_guilds)
	err = db.DeleteGuild(ctx, guild.ID)
	require.NoError(t, err)

	// Verify user-guild relationship is gone
	hasAccess, err := db.UserHasGuildAccess(ctx, user.ID, guild.DiscordGuildID)
	require.NoError(t, err)
	assert.False(t, hasAccess)

	// Verify user still exists
	retrievedUser, err := db.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrievedUser)
}
