package services

import (
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EmailSyncService manages periodic email syncing for all accounts based on
// each account's SyncInterval setting. It runs a background loop that checks
// every 30 seconds and syncs accounts whose sync interval has elapsed.
type EmailSyncService struct {
	config   *ConfigService
	email    *EmailService
	mu       sync.Mutex
	lastSync map[string]time.Time // accountID → last sync time
	stopCh   chan struct{}
}

func NewEmailSyncService(cfg *ConfigService, email *EmailService) *EmailSyncService {
	return &EmailSyncService{
		config:   cfg,
		email:    email,
		lastSync: make(map[string]time.Time),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic sync loop. It checks every 30 seconds.
func (s *EmailSyncService) Start() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.syncLoop()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop stops the background sync loop.
func (s *EmailSyncService) Stop() {
	close(s.stopCh)
}

// ForceSyncNow immediately syncs the INBOX for a specific account and returns
// the refreshed email summaries. It is called from the frontend's force-refresh
// button.
func (s *EmailSyncService) ForceSyncNow(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	resp, err := s.email.RefreshEmails(account, folder, page, pageSize)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.lastSync[account.ID] = time.Now()
	s.mu.Unlock()

	// Notify frontend
	app := application.Get()
	if app != nil {
		app.Event.Emit("emails-synced", account.ID)
	}
	return resp, nil
}

func (s *EmailSyncService) syncLoop() {
	accounts, err := s.config.GetAccounts()
	if err != nil {
		return
	}

	now := time.Now()

	for _, acct := range accounts {
		if acct.SyncInterval <= 0 {
			continue
		}

		s.mu.Lock()
		last, exists := s.lastSync[acct.ID]
		s.mu.Unlock()

		interval := time.Duration(acct.SyncInterval) * time.Minute
		if exists && now.Sub(last) < interval {
			continue
		}

		go s.syncAccount(acct)
	}
}

func (s *EmailSyncService) syncAccount(acct EmailAccount) {
	folders, err := s.email.ListFolders(acct)
	if err != nil {
		return
	}

	// Find the INBOX folder and sync it
	for _, f := range folders {
		name := strings.ToLower(f.Name)
		if name == "inbox" || strings.HasSuffix(name, "/inbox") {
			_, err := s.email.RefreshEmails(acct, f.Name, 1, 20)
			if err != nil {
				return
			}

			s.mu.Lock()
			s.lastSync[acct.ID] = time.Now()
			s.mu.Unlock()

			app := application.Get()
			if app != nil {
				app.Event.Emit("emails-synced", acct.ID)
			}
			return
		}
	}
}
