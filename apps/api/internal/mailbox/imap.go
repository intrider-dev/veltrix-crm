package mailbox

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	messageMail "github.com/emersion/go-message/mail"
)

type IMAPTransport interface {
	ListFolders(context.Context, ConnectionConfig, string) ([]RemoteFolder, error)
	FetchMessages(context.Context, ConnectionConfig, string, string, uint32, int) (RemoteFolderPage, error)
	ReadBody(context.Context, ConnectionConfig, string, string, uint32) (string, error)
}

type ConnectionConfig struct {
	Host     string
	Port     int
	Security string
	Username string
}

type EmersionIMAP struct {
	Policy          EndpointPolicy
	CommandTimeout  time.Duration
	MaxBodyBytes    int64
	MaxMessageBytes int64
}

func (transport EmersionIMAP) ListFolders(
	ctx context.Context, config ConnectionConfig, password string,
) ([]RemoteFolder, error) {
	client, err := transport.connect(ctx, config, password)
	if err != nil {
		return nil, err
	}
	defer logoutIMAP(client)

	mailboxes := make(chan *imap.MailboxInfo, MaxFolders+1)
	done := make(chan error, 1)
	go func() { done <- client.List("", "*", mailboxes) }()
	folders := make([]RemoteFolder, 0, 16)
	for info := range mailboxes {
		if info == nil {
			continue
		}
		if len(folders) >= MaxFolders {
			_ = client.Terminate()
			return nil, errors.New("mailbox folder limit exceeded")
		}
		folders = append(folders, RemoteFolder{
			Name: info.Name, DisplayName: info.Name, Delimiter: info.Delimiter,
			SpecialUse: specialUse(info),
		})
	}
	if err := <-done; err != nil {
		return nil, classifyIMAPError(err)
	}
	return folders, nil
}

func (transport EmersionIMAP) FetchMessages(
	ctx context.Context, config ConnectionConfig, password, folderName string, sinceUID uint32, limit int,
) (RemoteFolderPage, error) {
	if limit < 1 || limit > MaxSyncMessages {
		return RemoteFolderPage{}, errors.New("invalid IMAP fetch limit")
	}
	client, err := transport.connect(ctx, config, password)
	if err != nil {
		return RemoteFolderPage{}, err
	}
	defer logoutIMAP(client)
	status, err := client.Select(folderName, true)
	if err != nil {
		return RemoteFolderPage{}, classifyIMAPError(err)
	}
	page := RemoteFolderPage{UIDValidity: status.UidValidity, UIDNext: status.UidNext, Total: status.Messages, Unseen: status.Unseen}
	if status.Messages == 0 || status.UidNext <= 1 {
		return page, nil
	}
	start := sinceUID + 1
	if sinceUID == 0 {
		if status.UidNext > uint32(limit) {
			start = status.UidNext - uint32(limit)
		} else {
			start = 1
		}
	} else if status.UidNext > uint32(limit) && start < status.UidNext-uint32(limit) {
		// Never request an unbounded backlog in one synchronization pass. Older
		// mail remains available through a future explicit history-import path.
		start = status.UidNext - uint32(limit)
	}
	if start >= status.UidNext {
		return page, nil
	}
	sequence := new(imap.SeqSet)
	sequence.AddRange(start, status.UidNext-1)
	items := []imap.FetchItem{
		imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate,
		imap.FetchRFC822Size, imap.FetchBodyStructure,
	}
	messages := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() { done <- client.UidFetch(sequence, items, messages) }()
	page.Messages = make([]RemoteMessage, 0, limit)
	for item := range messages {
		if item == nil || item.Envelope == nil || item.Uid == 0 {
			continue
		}
		if len(page.Messages) >= limit {
			_ = client.Terminate()
			return RemoteFolderPage{}, errors.New("IMAP server exceeded requested message bound")
		}
		page.Messages = append(page.Messages, remoteMessage(item))
	}
	if err := <-done; err != nil {
		return RemoteFolderPage{}, classifyIMAPError(err)
	}
	return page, nil
}

func (transport EmersionIMAP) ReadBody(
	ctx context.Context, config ConnectionConfig, password, folderName string, uid uint32,
) (string, error) {
	if uid == 0 {
		return "", ErrMalformedMessage
	}
	client, err := transport.connect(ctx, config, password)
	if err != nil {
		return "", err
	}
	defer logoutIMAP(client)
	if _, err := client.Select(folderName, true); err != nil {
		return "", classifyIMAPError(err)
	}
	maxMessage := transport.MaxMessageBytes
	if maxMessage <= 0 || maxMessage > MaxMessageBytes {
		maxMessage = MaxMessageBytes
	}
	section := &imap.BodySectionName{Peek: true, Partial: []int{0, int(maxMessage + 1)}}
	sequence := new(imap.SeqSet)
	sequence.AddNum(uid)
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- client.UidFetch(sequence, []imap.FetchItem{imap.FetchUid, section.FetchItem()}, messages)
	}()
	var body io.Reader
	for item := range messages {
		if item != nil && item.Uid == uid {
			body = item.GetBody(section)
		}
	}
	if err := <-done; err != nil {
		return "", classifyIMAPError(err)
	}
	if body == nil {
		return "", ErrMalformedMessage
	}
	return parsePlainText(body, maxMessage, transport.bodyLimit())
}

func (transport EmersionIMAP) connect(
	ctx context.Context, config ConnectionConfig, password string,
) (*imapclient.Client, error) {
	if password == "" || len(password) > 4096 {
		return nil, errors.New("mail credentials are invalid")
	}
	policy := transport.Policy
	if policy.IMAPPorts == nil {
		policy = DefaultEndpointPolicy()
	}
	raw, err := policy.DialContext(ctx, config.Host, config.Port, "imap")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = raw.Close()
		}
	}()
	deadline := commandDeadline(ctx, transport.CommandTimeout)
	_ = raw.SetDeadline(deadline)
	var client *imapclient.Client
	if config.Security == "tls" {
		tlsConnection := tls.Client(raw, tlsForHost(config.Host))
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return nil, errors.New("mail TLS handshake failed")
		}
		client, err = imapclient.New(tlsConnection)
	} else if config.Security == "starttls" {
		client, err = imapclient.New(raw)
		if err == nil {
			err = client.StartTLS(tlsForHost(config.Host))
		}
	} else {
		return nil, ErrEndpointRejected
	}
	if err != nil {
		return nil, classifyIMAPError(err)
	}
	client.Timeout = boundedCommandTimeout(transport.CommandTimeout)
	client.ErrorLog = log.New(io.Discard, "", 0)
	if err := client.Login(config.Username, password); err != nil {
		_ = client.Terminate()
		return nil, classifyIMAPError(err)
	}
	keep = true
	return client, nil
}

func (transport EmersionIMAP) bodyLimit() int64 {
	limit := transport.MaxBodyBytes
	if limit <= 0 || limit > MaxBodyBytes {
		return MaxBodyBytes
	}
	return limit
}

func parsePlainText(source io.Reader, maxMessageBytes, maxBodyBytes int64) (string, error) {
	if maxMessageBytes < 1 || maxMessageBytes > MaxMessageBytes || maxBodyBytes < 1 || maxBodyBytes > MaxBodyBytes {
		return "", ErrMessageTooLarge
	}
	limited := &boundedReader{reader: source, remaining: maxMessageBytes + 1}
	reader, err := messageMail.CreateReader(limited)
	if err != nil {
		return "", ErrMalformedMessage
	}
	parts := 0
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return "", ErrMalformedMessage
		}
		parts++
		if parts > MaxMIMEParts {
			return "", ErrMessageTooLarge
		}
		inline, ok := part.Header.(*messageMail.InlineHeader)
		if !ok {
			continue
		}
		mediaType, _, contentErr := inline.ContentType()
		if contentErr != nil || !strings.EqualFold(mediaType, "text/plain") {
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(part.Body, maxBodyBytes+1))
		if readErr != nil || int64(len(content)) > maxBodyBytes {
			return "", ErrMessageTooLarge
		}
		return normalizePlainText(string(content)), nil
	}
	if limited.remaining <= 0 {
		return "", ErrMessageTooLarge
	}
	// HTML is deliberately not returned: rendering arbitrary remote HTML would
	// enable active-content and tracking attacks. A future sanitizer can be
	// introduced as a separate, reviewed adapter.
	return "", nil
}

type boundedReader struct {
	reader    io.Reader
	remaining int64
}

func (reader *boundedReader) Read(buffer []byte) (int, error) {
	if reader.remaining <= 0 {
		return 0, ErrMessageTooLarge
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	read, err := reader.reader.Read(buffer)
	reader.remaining -= int64(read)
	return read, err
}

func normalizePlainText(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	return strings.TrimSpace(strings.ToValidUTF8(value, "�"))
}

func remoteMessage(item *imap.Message) RemoteMessage {
	envelope := item.Envelope
	sender := firstAddress(envelope.From)
	return RemoteMessage{
		UID: item.Uid, InternetMessageID: truncateRunes(sanitizeHeaderText(envelope.MessageId), 998),
		Subject: truncateRunes(sanitizeHeaderText(envelope.Subject), 2000), Sender: sender,
		Recipients: RecipientSet{To: addresses(envelope.To), Cc: addresses(envelope.Cc)},
		SentAt:     envelope.Date.UTC(), ReceivedAt: item.InternalDate.UTC(), Flags: boundedFlags(item.Flags),
		SizeBytes: item.Size, HasAttachments: bodyHasAttachment(item.BodyStructure),
	}
}

func firstAddress(values []*imap.Address) Address {
	if len(values) == 0 || values[0] == nil {
		return Address{}
	}
	return mapIMAPAddress(values[0])
}

func addresses(values []*imap.Address) []Address {
	result := make([]Address, 0, min(len(values), MaxRecipients))
	for _, value := range values {
		if value != nil && len(result) < MaxRecipients {
			result = append(result, mapIMAPAddress(value))
		}
	}
	return result
}

func mapIMAPAddress(value *imap.Address) Address {
	address := value.MailboxName + "@" + value.HostName
	return Address{
		Name:    truncateRunes(sanitizeHeaderText(value.PersonalName), 320),
		Address: truncateRunes(sanitizeHeaderText(address), 320),
	}
}

func boundedFlags(flags []string) []string {
	result := make([]string, 0, min(len(flags), 64))
	for _, flag := range flags {
		if len(result) >= 64 {
			break
		}
		flag = strings.TrimSpace(flag)
		if flag != "" && len(flag) <= 128 {
			result = append(result, flag)
		}
	}
	return result
}

func bodyHasAttachment(structure *imap.BodyStructure) bool {
	if structure == nil {
		return false
	}
	found := false
	structure.Walk(func(_ []int, part *imap.BodyStructure) bool {
		if strings.EqualFold(part.Disposition, "attachment") {
			found = true
		}
		if filename, err := part.Filename(); err == nil && strings.TrimSpace(filename) != "" {
			found = true
		}
		return true
	})
	return found
}

func specialUse(info *imap.MailboxInfo) string {
	if strings.EqualFold(info.Name, "INBOX") {
		return "inbox"
	}
	for _, attribute := range info.Attributes {
		switch strings.ToLower(attribute) {
		case `\sent`:
			return "sent"
		case `\drafts`:
			return "drafts"
		case `\trash`:
			return "trash"
		case `\archive`, `\all`:
			return "archive"
		case `\junk`:
			return "junk"
		}
	}
	return ""
}

func classifyIMAPError(err error) error {
	if err == nil {
		return nil
	}
	return ErrIMAPOperation
}

func logoutIMAP(client *imapclient.Client) {
	if client == nil {
		return
	}
	if err := client.Logout(); err != nil {
		_ = client.Terminate()
	}
}

func boundedCommandTimeout(value time.Duration) time.Duration {
	if value <= 0 || value > 30*time.Second {
		return 15 * time.Second
	}
	return value
}

func commandDeadline(ctx context.Context, timeout time.Duration) time.Time {
	deadline := time.Now().Add(boundedCommandTimeout(timeout))
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		return contextDeadline
	}
	return deadline
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, "�"))
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func sanitizeHeaderText(value string) string {
	return strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return -1
		}
		return character
	}, value)
}
