package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const composeWindowName = "compose"

// ComposeAttachment is a file already staged inside the draft directory. The
// frontend keeps this object in the draft; Path is used only by the Go sender.
type ComposeAttachment struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	MimeType  string `json:"MimeType"`
	Size      int64  `json:"Size"`
	Path      string `json:"Path"`
	Inline    bool   `json:"Inline"`
	ContentID string `json:"ContentID,omitempty"`
	// Preview is a data URL for inline images so Tiptap can render the image
	// before it is converted to a cid: reference at send time.
	Preview string `json:"Preview,omitempty"`
}

// ComposeDraft is the mutable state shared by the compose window and Go. Large
// HTML and attachment lists stay here, never in the window URL.
type ComposeDraft struct {
	ID           string              `json:"ID"`
	AccountID    string              `json:"AccountID"`
	To           []EmailAddress      `json:"To"`
	Cc           []EmailAddress      `json:"Cc"`
	Bcc          []EmailAddress      `json:"Bcc"`
	Subject      string              `json:"Subject"`
	Text         string              `json:"Text"`
	HTML         string              `json:"HTML"`
	Attachments  []ComposeAttachment `json:"Attachments"`
	InlineImages []ComposeAttachment `json:"InlineImages"`
	InReplyTo    string              `json:"InReplyTo"`
	References   []string            `json:"References"`
}

type OpenComposeInput struct {
	AccountID  string         `json:"AccountID"`
	To         []EmailAddress `json:"To"`
	Cc         []EmailAddress `json:"Cc"`
	Bcc        []EmailAddress `json:"Bcc"`
	Subject    string         `json:"Subject"`
	Text       string         `json:"Text"`
	HTML       string         `json:"HTML"`
	InReplyTo  string         `json:"InReplyTo"`
	References []string       `json:"References"`
}

type OpenComposeResult struct {
	DraftID string `json:"DraftID"`
	Reused  bool   `json:"Reused"`
	Warning string `json:"Warning,omitempty"`
}

type composeState struct {
	draft      ComposeDraft
	dirty      bool
	allowClose bool
}

// ComposeService owns the single compose window, its in-memory drafts and the
// file staging directory. It deliberately does not persist drafts yet: closing
// an unsent window asks the frontend for confirmation instead of silently
// leaving stale state behind.
type ComposeService struct {
	cfg    *ConfigService
	sender *EmailSendService

	mu       sync.Mutex
	state    *composeState
	windowID string
}

func NewComposeService(cfg *ConfigService, sender *EmailSendService) *ComposeService {
	return &ComposeService{cfg: cfg, sender: sender}
}

func (s *ComposeService) OpenCompose(input OpenComposeInput) (OpenComposeResult, error) {
	app := application.Get()
	if app == nil {
		return OpenComposeResult{}, fmt.Errorf("application is not ready")
	}
	accountID, err := s.resolveAccountID(input.AccountID)
	if err != nil {
		return OpenComposeResult{}, err
	}
	input.AccountID = accountID

	s.mu.Lock()
	oldDraftID := ""
	if s.state != nil {
		if s.state.dirty {
			draftID := s.state.draft.ID
			s.mu.Unlock()
			if win, ok := app.Window.GetByName(composeWindowName); ok {
				win.Show()
				win.Focus()
			}
			return OpenComposeResult{
				DraftID: draftID,
				Reused:  true,
				Warning: "another compose draft is still open; finish or discard it before opening a new message",
			}, nil
		}
		oldDraftID = s.state.draft.ID
	}

	draftID := "draft-" + randomHex(12)
	draft := ComposeDraft{
		ID:           draftID,
		AccountID:    input.AccountID,
		To:           input.To,
		Cc:           input.Cc,
		Bcc:          input.Bcc,
		Subject:      input.Subject,
		Text:         input.Text,
		HTML:         input.HTML,
		InReplyTo:    input.InReplyTo,
		References:   input.References,
		Attachments:  []ComposeAttachment{},
		InlineImages: []ComposeAttachment{},
	}
	if strings.TrimSpace(draft.HTML) == "" && strings.TrimSpace(draft.Text) != "" {
		draft.HTML = textToHTML(draft.Text)
	}
	s.state = &composeState{draft: draft}
	s.mu.Unlock()
	if oldDraftID != "" && oldDraftID != draftID {
		removeComposeDraftDir(oldDraftID)
	}

	if win, ok := app.Window.GetByName(composeWindowName); ok {
		win.SetURL("/#/compose?id=" + url.QueryEscape(draftID))
		win.Show()
		win.Focus()
		return OpenComposeResult{DraftID: draftID}, nil
	}
	if err := s.createWindow(app, draftID); err != nil {
		s.mu.Lock()
		s.state = nil
		s.mu.Unlock()
		return OpenComposeResult{}, err
	}
	return OpenComposeResult{DraftID: draftID}, nil
}

func (s *ComposeService) OpenMailto(rawURL, accountID string) (OpenComposeResult, error) {
	input, err := parseMailto(rawURL)
	if err != nil {
		return OpenComposeResult{}, err
	}
	input.AccountID = accountID
	return s.OpenCompose(input)
}

func (s *ComposeService) GetDraft(id string) (ComposeDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil || s.state.draft.ID != id {
		return ComposeDraft{}, fmt.Errorf("compose draft %q not found", id)
	}
	return cloneDraft(s.state.draft), nil
}

func (s *ComposeService) UpdateDraft(draft ComposeDraft) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil || s.state.draft.ID != draft.ID {
		return fmt.Errorf("compose draft %q not found", draft.ID)
	}
	if _, err := s.resolveAccountID(draft.AccountID); err != nil {
		return err
	}
	s.state.draft = cloneDraft(draft)
	s.state.dirty = true
	return nil
}

func (s *ComposeService) SetDirty(id string, dirty bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil || s.state.draft.ID != id {
		return fmt.Errorf("compose draft %q not found", id)
	}
	s.state.dirty = dirty
	return nil
}

func (s *ComposeService) PickAttachments(draftID string) ([]ComposeAttachment, error) {
	return s.pickFiles(draftID, false)
}

func (s *ComposeService) PickInlineImages(draftID string) ([]ComposeAttachment, error) {
	return s.pickFiles(draftID, true)
}

func (s *ComposeService) ImportFiles(draftID string, paths []string, inline bool) ([]ComposeAttachment, error) {
	s.mu.Lock()
	exists := s.state != nil && s.state.draft.ID == draftID
	s.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("compose draft %q not found", draftID)
	}
	return s.stageFiles(draftID, paths, inline)
}

func (s *ComposeService) PreviewAttachment(draftID, attachmentID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil || s.state.draft.ID != draftID {
		return "", fmt.Errorf("compose draft %q not found", draftID)
	}
	for _, attachment := range append(s.state.draft.InlineImages, s.state.draft.Attachments...) {
		if attachment.ID == attachmentID {
			return attachment.Preview, nil
		}
	}
	return "", fmt.Errorf("attachment %q not found", attachmentID)
}

func (s *ComposeService) SendDraft(id string) (*SendEmailResult, error) {
	s.mu.Lock()
	if s.state == nil || s.state.draft.ID != id {
		s.mu.Unlock()
		return nil, fmt.Errorf("compose draft %q not found", id)
	}
	draft := cloneDraft(s.state.draft)
	s.mu.Unlock()

	req := SendEmailRequest{
		AccountID:  draft.AccountID,
		To:         draft.To,
		Cc:         draft.Cc,
		Bcc:        draft.Bcc,
		Subject:    draft.Subject,
		Text:       draft.Text,
		HTML:       draft.HTML,
		InReplyTo:  draft.InReplyTo,
		References: draft.References,
	}
	for _, attachment := range draft.Attachments {
		req.Attachments = append(req.Attachments, SendAttachment{
			Path:        attachment.Path,
			Filename:    attachment.Name,
			ContentType: attachment.MimeType,
		})
	}
	for _, image := range draft.InlineImages {
		req.InlineImages = append(req.InlineImages, SendInlineImage{
			Path:        image.Path,
			Filename:    image.Name,
			ContentType: image.MimeType,
			ContentID:   image.ContentID,
		})
	}
	result, err := s.sender.SendEmail(req)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.state != nil && s.state.draft.ID == id {
		s.state.allowClose = true
	}
	s.mu.Unlock()
	s.closeWindow()
	s.removeDraft(id)

	if app := application.Get(); app != nil {
		app.Event.Emit("emails-synced", draft.AccountID)
	}
	return result, nil
}

func (s *ComposeService) CloseCompose(id string) error {
	s.mu.Lock()
	if s.state == nil || s.state.draft.ID != id {
		s.mu.Unlock()
		return nil
	}
	s.state.allowClose = true
	s.state.dirty = false
	s.mu.Unlock()
	s.closeWindow()
	s.removeDraft(id)
	return nil
}

func (s *ComposeService) createWindow(app *application.App, draftID string) error {
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             composeWindowName,
		Title:            "New Message",
		URL:              "/#/compose?id=" + url.QueryEscape(draftID),
		Width:            920,
		Height:           720,
		MinWidth:         720,
		MinHeight:        560,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(27, 38, 54),
	})
	win.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if s.canClose(draftID) {
			return
		}
		event.Cancel()
		win.EmitEvent("compose:confirm-close", draftID)
	})
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		details := event.Context().DropTargetDetails()
		targetID := ""
		if details != nil {
			targetID = details.ElementID
		}
		win.EmitEvent("compose:files-dropped", map[string]any{
			"draftID":  draftID,
			"files":    event.Context().DroppedFiles(),
			"targetID": targetID,
		})
	})
	s.mu.Lock()
	s.windowID = fmt.Sprintf("%d", win.ID())
	s.mu.Unlock()
	win.Show()
	win.Focus()
	return nil
}

func (s *ComposeService) canClose(draftID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == nil || s.state.draft.ID != draftID || s.state.allowClose
}

func (s *ComposeService) closeWindow() {
	app := application.Get()
	if app == nil {
		return
	}
	if win, ok := app.Window.GetByName(composeWindowName); ok {
		win.Close()
	}
}

func (s *ComposeService) removeDraft(id string) {
	s.mu.Lock()
	if s.state != nil && s.state.draft.ID == id {
		s.state = nil
		s.windowID = ""
	}
	s.mu.Unlock()
	removeComposeDraftDir(id)
}

func removeComposeDraftDir(id string) {
	if dir, err := composeDraftDir(id); err == nil {
		_ = os.RemoveAll(dir)
	}
}

func (s *ComposeService) resolveAccountID(id string) (string, error) {
	if s.cfg == nil {
		return "", fmt.Errorf("config service is unavailable")
	}
	accounts, err := s.cfg.GetAccounts()
	if err != nil {
		return "", fmt.Errorf("load accounts: %w", err)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		if len(accounts) == 0 {
			return "", fmt.Errorf("no email accounts are configured")
		}
		return accounts[0].ID, nil
	}
	for _, account := range accounts {
		if account.ID == id {
			return id, nil
		}
	}
	return "", fmt.Errorf("account %q not found", id)
}

func (s *ComposeService) pickFiles(draftID string, inline bool) ([]ComposeAttachment, error) {
	app := application.Get()
	if app == nil {
		return nil, fmt.Errorf("application is not ready")
	}
	s.mu.Lock()
	exists := s.state != nil && s.state.draft.ID == draftID
	s.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("compose draft %q not found", draftID)
	}

	dialog := app.Dialog.OpenFile().SetTitle("Add attachments")
	if inline {
		dialog.SetTitle("Add inline image").AddFilter("Images", "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp")
	}
	paths, err := dialog.PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	return s.stageFiles(draftID, paths, inline)
}

func (s *ComposeService) stageFiles(draftID string, paths []string, inline bool) ([]ComposeAttachment, error) {
	dir, err := composeDraftDir(draftID)
	if err != nil {
		return nil, err
	}
	result := make([]ComposeAttachment, 0, len(paths))
	for _, source := range paths {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		info, err := os.Stat(source)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", source, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%q is not a regular file", source)
		}
		name := sanitizeAttachmentFilename(filepath.Base(source))
		if name == "" {
			name = "attachment"
		}
		storedName := randomHex(8) + "-" + name
		dest := filepath.Join(dir, storedName)
		if err := copyFile(source, dest); err != nil {
			return nil, err
		}
		contentType, err := detectFileContentType(dest, name)
		if err != nil {
			_ = os.Remove(dest)
			return nil, err
		}
		if inline && !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			_ = os.Remove(dest)
			return nil, fmt.Errorf("%q is not an image", name)
		}
		attachment := ComposeAttachment{
			ID:       storedName,
			Name:     name,
			MimeType: contentType,
			Size:     info.Size(),
			Path:     dest,
			Inline:   inline,
		}
		if inline {
			attachment.ContentID = "wvmail-" + randomHex(16)
			attachment.Preview, _ = dataURL(dest, contentType)
		}
		result = append(result, attachment)
	}
	return result, nil
}

func composeDraftDir(id string) (string, error) {
	if !validComposeID(id) {
		return "", fmt.Errorf("invalid compose draft id")
	}
	base, err := configDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "compose", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create compose draft dir: %w", err)
	}
	return dir, nil
}

func validComposeID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %q: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create staged file: %w", err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("stage %q: %w", source, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("close staged file: %w", closeErr)
	}
	return nil
}

func detectFileContentType(path, filename string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	return normalizeContentType("", filename, head[:n]), nil
}

func dataURL(path, contentType string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func parseMailto(raw string) (OpenComposeInput, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return OpenComposeInput{}, fmt.Errorf("invalid mailto URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "mailto") {
		return OpenComposeInput{}, fmt.Errorf("not a mailto URL")
	}
	rawRecipients, err := url.PathUnescape(u.Opaque)
	if err != nil {
		return OpenComposeInput{}, fmt.Errorf("invalid mailto recipient: %w", err)
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return OpenComposeInput{}, fmt.Errorf("invalid mailto query: %w", err)
	}
	to := append(splitAddressList(rawRecipients), splitAddressList(query.Get("to"))...)
	cc := splitAddressList(query.Get("cc"))
	bcc := splitAddressList(query.Get("bcc"))
	body := query.Get("body")
	input := OpenComposeInput{
		To:      parseAddressInputs(to),
		Cc:      parseAddressInputs(cc),
		Bcc:     parseAddressInputs(bcc),
		Subject: query.Get("subject"),
		Text:    body,
	}
	if strings.TrimSpace(body) != "" {
		input.HTML = textToHTML(body)
	}
	return input, nil
}

func splitAddressList(value string) []string {
	var result []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseAddressInputs(values []string) []EmailAddress {
	result := make([]EmailAddress, 0, len(values))
	for _, value := range values {
		parsed, err := mail.ParseAddress(value)
		if err != nil {
			result = append(result, EmailAddress{Address: value})
			continue
		}
		result = append(result, EmailAddress{Name: parsed.Name, Address: parsed.Address})
	}
	return result
}

func cloneDraft(draft ComposeDraft) ComposeDraft {
	clone := draft
	clone.To = append([]EmailAddress(nil), draft.To...)
	clone.Cc = append([]EmailAddress(nil), draft.Cc...)
	clone.Bcc = append([]EmailAddress(nil), draft.Bcc...)
	clone.Attachments = append([]ComposeAttachment(nil), draft.Attachments...)
	clone.InlineImages = append([]ComposeAttachment(nil), draft.InlineImages...)
	clone.References = append([]string(nil), draft.References...)
	return clone
}
