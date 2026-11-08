package services

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// MailNotificationSender turns newly received INBOX messages into native
// desktop notifications through Wails' cross-platform notification service.
type MailNotificationSender struct {
	config  *ConfigService
	service *notifications.NotificationService

	shouldNotify func() bool
	policyMu     sync.RWMutex
	authMu       sync.Mutex
	sendMu       sync.Mutex
	authReady    bool
	authorized   bool
}

func NewMailNotificationSender(
	config *ConfigService,
	service *notifications.NotificationService,
) *MailNotificationSender {
	return &MailNotificationSender{config: config, service: service}
}

// SetShouldNotify registers whether notifications should be suppressed, for
// example while the main window is focused.
//
//wails:ignore
func (s *MailNotificationSender) SetShouldNotify(shouldNotify func() bool) {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	s.shouldNotify = shouldNotify
}

func (s *MailNotificationSender) NotifyNewMail(account EmailAccount, emails []EmailSummary) {
	if s.service == nil || len(emails) == 0 {
		return
	}
	emails = append([]EmailSummary(nil), emails...)
	if s.isSuppressed() {
		slog.Info("notification: skipped new-mail notification while main window is focused",
			"account_id", account.ID, "count", len(emails))
		return
	}
	slog.Info("notification: new mail detected", "account_id", account.ID, "count", len(emails))
	go s.sendNewMail(account, emails)
}

func (s *MailNotificationSender) sendNewMail(account EmailAccount, emails []EmailSummary) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if s.isSuppressed() {
		return
	}
	if !s.ensureAuthorized() {
		return
	}
	language := "en-US"
	if s.config != nil {
		config, err := s.config.GetConfig()
		if err != nil {
			slog.Warn("notification: failed to load language setting", "error", err)
		} else {
			language = config.Language
		}
	}

	for _, email := range emails {
		if s.isSuppressed() {
			return
		}
		if err := s.service.SendNotification(newMailNotificationOptions(language, account, email)); err != nil {
			slog.Warn("notification: failed to send new-mail notification",
				"account_id", account.ID, "uid", email.UID, "error", err)
		} else {
			slog.Info("notification: new-mail notification sent",
				"account_id", account.ID, "uid", email.UID)
		}
	}
}

func (s *MailNotificationSender) isSuppressed() bool {
	s.policyMu.RLock()
	shouldNotify := s.shouldNotify
	s.policyMu.RUnlock()
	return shouldNotify != nil && shouldNotify()
}

func (s *MailNotificationSender) ensureAuthorized() bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()

	if s.authReady {
		return s.authorized
	}

	authorized, err := s.service.CheckNotificationAuthorization()
	if err != nil {
		slog.Warn("notification: failed to check authorization", "error", err)
		return false
	}
	if !authorized {
		authorized, err = s.service.RequestNotificationAuthorization()
		if err != nil {
			slog.Warn("notification: failed to request authorization", "error", err)
			return false
		}
	}

	s.authReady = true
	s.authorized = authorized
	if !authorized {
		slog.Warn("notification: authorization denied")
	}
	return authorized
}

func newMailNotificationOptions(language string, account EmailAccount, email EmailSummary) notifications.NotificationOptions {
	title := "New email"
	if language == "zh-CN" {
		title = "新邮件"
	}

	sender := strings.TrimSpace(email.FromName)
	if sender == "" {
		sender = strings.TrimSpace(email.FromAddr)
	}
	if sender == "" {
		sender = strings.TrimSpace(email.From)
	}
	if sender == "" {
		sender = strings.TrimSpace(account.Email)
	}
	subject := strings.TrimSpace(email.Subject)
	if subject == "" {
		subject = "(No Subject)"
	}

	return notifications.NotificationOptions{
		ID:       fmt.Sprintf("mail-%s-%d", account.ID, email.UID),
		Title:    title,
		Body:     fmt.Sprintf("%s\n%s", sender, subject),
		ThreadID: account.ID,
	}
}
