package module

import (
	imapbackend "github.com/emersion/go-imap/backend"
)

// Storage interface is a slightly modified go-imap's Backend interface
// (authentication is removed).
//
// Modules implementing this interface should be registered with prefix
// "storage." in name.
type Storage interface {
	// GetOrCreateIMAPAcct returns User associated with storage account specified by
	// the name.
	//
	// If it doesn't exists - it should be created.
	GetOrCreateIMAPAcct(username string) (imapbackend.User, error)
	GetIMAPAcct(username string) (imapbackend.User, error)

	// Extensions returns list of IMAP extensions supported by backend.
	IMAPExtensions() []string
}

// ManageableStorage is an extended Storage interface that allows to
// list existing accounts, create and delete them.
type ManageableStorage interface {
	Storage

	ListIMAPAccts() ([]string, error)
	CreateIMAPAcct(username string) error
	DeleteIMAPAcct(username string) error
}
