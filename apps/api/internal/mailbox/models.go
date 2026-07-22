package mailbox

import (
	"encoding/json"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const (
	MaxAccounts                    = 16
	MaxFolders                     = 64
	MaxFoldersPerSync              = 8
	MaxSyncMessages                = 100
	MaxMessagePage                 = 100
	MaxRecipients                  = 100
	MaxBodyBytes             int64 = 2 << 20
	MaxMessageBytes          int64 = 25 << 20
	MaxMIMEParts                   = 100
	MaxConcurrentConnections       = 4
)

type AccountInput struct {
	DisplayName  string
	Email        string
	Username     string
	IMAPHost     string
	IMAPPort     int
	IMAPSecurity string
	SMTPHost     string
	SMTPPort     int
	SMTPSecurity string
	Password     string
	SyncEnabled  bool
}

type Account struct {
	ID            ids.UUID   `json:"id"`
	DisplayName   string     `json:"displayName"`
	Email         string     `json:"email"`
	Username      string     `json:"username"`
	IMAPHost      string     `json:"imapHost"`
	IMAPPort      int        `json:"imapPort"`
	IMAPSecurity  string     `json:"imapSecurity"`
	SMTPHost      string     `json:"smtpHost"`
	SMTPPort      int        `json:"smtpPort"`
	SMTPSecurity  string     `json:"smtpSecurity"`
	SyncEnabled   bool       `json:"syncEnabled"`
	SyncState     string     `json:"syncState"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
	LastErrorCode *string    `json:"lastErrorCode,omitempty"`
	Version       int64      `json:"version"`
}

type Folder struct {
	ID          ids.UUID   `json:"id"`
	AccountID   ids.UUID   `json:"accountId"`
	RemoteName  string     `json:"remoteName"`
	DisplayName string     `json:"displayName"`
	SpecialUse  *string    `json:"specialUse,omitempty"`
	UIDValidity *int64     `json:"uidValidity,omitempty"`
	UIDNext     *int64     `json:"uidNext,omitempty"`
	HighestUID  int64      `json:"highestUid"`
	TotalCount  int32      `json:"totalCount"`
	UnreadCount int32      `json:"unreadCount"`
	LastSyncAt  *time.Time `json:"lastSyncAt,omitempty"`
}

type Address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
}

type Message struct {
	ID                ids.UUID        `json:"id"`
	AccountID         ids.UUID        `json:"accountId"`
	FolderID          ids.UUID        `json:"folderId"`
	RemoteUID         int64           `json:"remoteUid"`
	InternetMessageID *string         `json:"internetMessageId,omitempty"`
	Subject           string          `json:"subject"`
	Sender            Address         `json:"sender"`
	Recipients        json.RawMessage `json:"recipients"`
	SentAt            *time.Time      `json:"sentAt,omitempty"`
	ReceivedAt        time.Time       `json:"receivedAt"`
	Flags             []string        `json:"flags"`
	SizeBytes         int64           `json:"sizeBytes"`
	Snippet           string          `json:"snippet"`
	HasAttachments    bool            `json:"hasAttachments"`
	BodyState         string          `json:"bodyState"`
}

type MessagePage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type RecipientSet struct {
	To  []Address `json:"to"`
	Cc  []Address `json:"cc,omitempty"`
	Bcc []Address `json:"bcc,omitempty"`
}

type SendInput struct {
	Recipients RecipientSet
	Subject    string
	PlainText  string
}

type RemoteFolder struct {
	Name        string
	DisplayName string
	Delimiter   string
	SpecialUse  string
	UIDValidity uint32
	UIDNext     uint32
	Total       uint32
	Unseen      uint32
}

type RemoteMessage struct {
	UID               uint32
	InternetMessageID string
	Subject           string
	Sender            Address
	Recipients        RecipientSet
	SentAt            time.Time
	ReceivedAt        time.Time
	Flags             []string
	SizeBytes         uint32
	HasAttachments    bool
}

type RemoteFolderPage struct {
	UIDValidity uint32
	UIDNext     uint32
	Total       uint32
	Unseen      uint32
	Messages    []RemoteMessage
}
