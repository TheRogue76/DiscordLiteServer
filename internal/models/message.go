package models

import (
	"database/sql"
	"time"
)

// MessageType represents Discord message types
type MessageType int

// Discord message type constants
const (
	MessageTypeDefault                                 MessageType = 0
	MessageTypeRecipientAdd                            MessageType = 1
	MessageTypeRecipientRemove                         MessageType = 2
	MessageTypeCall                                    MessageType = 3
	MessageTypeChannelNameChange                       MessageType = 4
	MessageTypeChannelIconChange                       MessageType = 5
	MessageTypeChannelPinnedMessage                    MessageType = 6
	MessageTypeGuildMemberJoin                         MessageType = 7
	MessageTypeUserPremiumGuildSubscription            MessageType = 8
	MessageTypeUserPremiumGuildSubscriptionTier1       MessageType = 9
	MessageTypeUserPremiumGuildSubscriptionTier2       MessageType = 10
	MessageTypeUserPremiumGuildSubscriptionTier3       MessageType = 11
	MessageTypeChannelFollowAdd                        MessageType = 12
	MessageTypeGuildDiscoveryDisqualified              MessageType = 14
	MessageTypeGuildDiscoveryRequalified               MessageType = 15
	MessageTypeGuildDiscoveryGracePeriodInitialWarning MessageType = 16
	MessageTypeGuildDiscoveryGracePeriodFinalWarning   MessageType = 17
	MessageTypeThreadCreated                           MessageType = 18
	MessageTypeReply                                   MessageType = 19
	MessageTypeChatInputCommand                        MessageType = 20
	MessageTypeThreadStarterMessage                    MessageType = 21
	MessageTypeGuildInviteReminder                     MessageType = 22
	MessageTypeContextMenuCommand                      MessageType = 23
	MessageTypeAutoModerationAction                    MessageType = 24
)

// Message represents a Discord message
type Message struct {
	ID                  int64          `json:"id"`
	DiscordMessageID    string         `json:"discord_message_id"`
	ChannelID           int64          `json:"channel_id"`
	AuthorID            string         `json:"author_id"`
	AuthorUsername      string         `json:"author_username"`
	AuthorAvatar        sql.NullString `json:"author_avatar"`
	Content             sql.NullString `json:"content"`
	Timestamp           time.Time      `json:"timestamp"`
	EditedTimestamp     sql.NullTime   `json:"edited_timestamp"`
	MessageType         MessageType    `json:"message_type"`
	ReferencedMessageID sql.NullString `json:"referenced_message_id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// MessageAttachment represents a file attachment in a message
type MessageAttachment struct {
	ID           int64          `json:"id"`
	MessageID    int64          `json:"message_id"`
	AttachmentID string         `json:"attachment_id"`
	Filename     string         `json:"filename"`
	URL          string         `json:"url"`
	ProxyURL     sql.NullString `json:"proxy_url"`
	SizeBytes    int            `json:"size_bytes"`
	Width        sql.NullInt64  `json:"width"`
	Height       sql.NullInt64  `json:"height"`
	ContentType  sql.NullString `json:"content_type"`
	CreatedAt    time.Time      `json:"created_at"`
}
