package jobs

import (
	"context"
	"database/sql"
	"log"

	"campusrec/internal/repository"
)

const ArchiverLockID int64 = 107

// Archiver archives old orders and tickets monthly.
func Archiver(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		backupRepo := repository.NewBackupRepository(db)

		ordersArchived, err := backupRepo.ArchiveOrders(24, 500)
		if err != nil {
			log.Printf("Archiver: error archiving orders: %v", err)
			return err
		}

		ticketsArchived, err := backupRepo.ArchiveTickets(24, 500)
		if err != nil {
			log.Printf("Archiver: error archiving tickets: %v", err)
			return err
		}

		if ordersArchived > 0 || ticketsArchived > 0 {
			log.Printf("Archiver: archived %d orders, %d tickets", ordersArchived, ticketsArchived)
		}

		return nil
	}
}
