package models

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Guild represents a Discord guild (server)
type Guild struct {
	ID             int64         `json:"id"`
	DiscordGuildID string        `json:"discord_guild_id"`
	Name           string        `json:"name"`
	Icon           sql.NullString `json:"icon"`
	OwnerID        sql.NullString `json:"owner_id"`
	Permissions    int64         `json:"permissions"`
	Features       pq.StringArray `json:"features"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// UserGuild represents a user's membership in a guild
type UserGuild struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	GuildID   int64     `json:"guild_id"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
}
