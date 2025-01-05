package pg

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var storage *Storage

func TestMain(m *testing.M) {
	ctx := context.Background()
	var container *postgres.PostgresContainer
	storage, container = mustSetup(ctx)
	defer teardown(ctx, storage, container)

	exitCode := m.Run()
	os.Exit(exitCode)
}

func mustSetup(ctx context.Context) (*Storage, *postgres.PostgresContainer) {
	dbName := "itchan"
	dbUser := "user"
	dbPassword := "password"
	container, err := postgres.Run(ctx,
		"postgres:15.3-alpine",
		postgres.WithInitScripts(filepath.Join("migrations", "init.sql")),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			// First, we wait for the container to log readiness twice.
			// This is because it will restart itself after the first startup.
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("failed to start container: %s", err)
	}
	containerPort, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		log.Fatalf("failed to obtain container port: %s", err)
	}
	port, err := strconv.Atoi(containerPort.Port())
	if err != nil {
		log.Fatalf("failed to obtain int container port: %s", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to obtain container host: %s", err)
	}

	storage, err := New(&config.Config{Public: config.Public{ThreadsPerPage: 3, NLastMsg: 3, BumpLimit: 15}, Private: config.Private{Pg: config.Pg{Host: host, Port: port, User: dbUser, Password: dbPassword, Dbname: dbName}}})
	if err != nil {
		log.Fatalf("failed to connect to postgres container: %s", err)
	}
	return storage, container
}

func teardown(ctx context.Context, storage *Storage, container *postgres.PostgresContainer) {
	if err := storage.Cleanup(); err != nil {
		log.Printf("failed to close storage connection: %s", err)
	}
	if err := container.Terminate(ctx); err != nil {
		log.Printf("failed to terminate container: %s", err)
	}
}
