package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type Storage struct {
	db  *sql.DB
	cfg *config.Config
}

func New(ctx context.Context, cfg *config.Config) (*Storage, error) {
	log.Print("Connecting to db")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	log.Print("Succesfully connected to db")
	storage := &Storage{db, cfg}
	storage.StartPeriodicViewRefresh(ctx, cfg.Public.BoardPreviewRefreshInterval*time.Second)
	return storage, nil
}

func Connect(cfg *config.Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Private.Pg.Host, cfg.Private.Pg.Port, cfg.Private.Pg.User, cfg.Private.Pg.Password, cfg.Private.Pg.Dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (s *Storage) Cleanup() error {
	return s.db.Close()
}

func messagesPartitionName(shortName domain.BoardShortName) string {
	return fmt.Sprintf("messages_%s", shortName)
}

func MessagesPartitionName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(messagesPartitionName(shortName))
}

func threadsPartitionName(shortName domain.BoardShortName) string {
	return fmt.Sprintf("threads_%s", shortName)
}

func ThreadsPartitionName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(threadsPartitionName(shortName))
}

func RepliesPartitionName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(fmt.Sprintf("message_replies_%s", shortName))
}

func PartitionName(shortName domain.BoardShortName, table string) string {
	if table == "messages" {
		return MessagesPartitionName(shortName)
	} else if table == "threads" {
		return ThreadsPartitionName(shortName)
	} else if table == "replies" {
		return RepliesPartitionName(shortName)
	} else {
		return "unknown"
	}
}

func viewTableName(shortName domain.BoardShortName) string {
	return fmt.Sprintf("board_preview_%s", shortName)
}

func ViewTableName(shortName domain.BoardShortName) string {
	return pq.QuoteIdentifier(viewTableName(shortName))
}
