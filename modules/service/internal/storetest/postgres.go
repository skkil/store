package storetest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Image is the Postgres image every test runs against.
const Image = "postgres:17.11-alpine"

const templateName = "store_template"

// DemoSchema is the product schema every test database starts with: notes
// (whose secret column is excluded from sync) and their tags, which are
// removed by ON DELETE CASCADE.
const DemoSchema = `
CREATE TABLE note (
  id uuid PRIMARY KEY,
  scope_id uuid NOT NULL,
  title text NOT NULL,
  done boolean NOT NULL DEFAULT false,
  secret text
);
CREATE TABLE note_tag (
  id uuid PRIMARY KEY,
  note_id uuid NOT NULL REFERENCES note ON DELETE CASCADE,
  tag text NOT NULL
);
SELECT store_register('note', 'id', ARRAY['secret']);
SELECT store_register('note_tag');
`

var (
	cluster     clusterState
	clusterOnce sync.Once
	databaseSeq atomic.Int64
)

type clusterState struct {
	admin string
	err   error
}

// Database is a fresh database with the store migrations and DemoSchema applied.
type Database struct {
	URL  string
	Pool *pgxpool.Pool
}

// NewDatabase returns an isolated database on a shared Postgres container.
// The container starts on first use and is removed when the test binary exits.
func NewDatabase(t testing.TB) *Database {
	t.Helper()
	clusterOnce.Do(func() { cluster = startCluster() })
	if cluster.err != nil {
		t.Fatalf("postgres: %v", cluster.err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := fmt.Sprintf("store_test_%d_%d", os.Getpid(), databaseSeq.Add(1))
	admin, err := pgx.Connect(ctx, cluster.admin)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateName)); err != nil {
		t.Fatal(err)
	}
	url := withDatabase(cluster.admin, name)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &Database{URL: url, Pool: pool}
}

func startCluster() clusterState {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, Image,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("store"),
		postgres.WithPassword("store"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return clusterState{err: err}
	}
	admin, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return clusterState{err: err}
	}
	if err := createTemplate(ctx, admin); err != nil {
		return clusterState{err: err}
	}
	return clusterState{admin: admin}
}

func createTemplate(ctx context.Context, admin string) error {
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+templateName); err != nil {
		return err
	}
	template, err := pgx.Connect(ctx, withDatabase(admin, templateName))
	if err != nil {
		return err
	}
	defer template.Close(ctx)
	if err := ApplyMigrations(ctx, template); err != nil {
		return err
	}
	_, err = template.Exec(ctx, DemoSchema)
	return err
}

// ApplyMigrations runs every modules/sql/migrations/V<n>__*.sql file in order.
func ApplyMigrations(ctx context.Context, conn *pgx.Conn) error {
	files, err := MigrationFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

// MigrationFiles lists the SQL migrations ordered by version.
func MigrationFiles() ([]string, error) {
	_, self, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(self), "..", "..", "..", "sql", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "V*__*.sql"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no migrations in %s", dir)
	}
	slices.SortFunc(files, func(a, b string) int { return migrationVersion(a) - migrationVersion(b) })
	return files, nil
}

func migrationVersion(file string) int {
	name := strings.TrimPrefix(filepath.Base(file), "V")
	n, _ := strconv.Atoi(name[:strings.Index(name, "__")])
	return n
}

func withDatabase(url, name string) string {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.User, config.Password, config.Host, config.Port, name)
}
