package models

import (
	"database/sql"
	"time"
)

// ChannelType represents Discord channel types
type ChannelType int

// Discord channel type constants
const (
	ChannelTypeGuildText          ChannelType = 0
	ChannelTypeDM                 ChannelType = 1
	ChannelTypeGuildVoice         ChannelType = 2
	ChannelTypeGroupDM            ChannelType = 3
	ChannelTypeGuildCategory      ChannelType = 4
	ChannelTypeGuildNews          ChannelType = 5
	ChannelTypeGuildStore         ChannelType = 6
	ChannelTypeGuildNewsThread    ChannelType = 10
	ChannelTypeGuildPublicThread  ChannelType = 11
	ChannelTypeGuildPrivateThread ChannelType = 12
	ChannelTypeGuildStageVoice    ChannelType = 13
	ChannelTypeGuildForum         ChannelType = 15
)

// Channel represents a Discord channel
type Channel struct {
	ID               int64          `json:"id"`
	DiscordChannelID string         `json:"discord_channel_id"`
	GuildID          int64          `json:"guild_id"`
	Name             string         `json:"name"`
	Type             ChannelType    `json:"type"`
	Position         int            `json:"position"`
	ParentID         sql.NullString `json:"parent_id"`
	Topic            sql.NullString `json:"topic"`
	NSFW             bool           `json:"nsfw"`
	LastMessageID    sql.NullString `json:"last_message_id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}
