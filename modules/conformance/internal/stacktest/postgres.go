package stacktest

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

	"github.com/skkil/store/modules/conformance/reference"
)

// Image is the Postgres image the tests run against, the same as modules/service.
const Image = "postgres:17.11-alpine"

const templateName = "conformance_template"

var (
	cluster     clusterState
	clusterOnce sync.Once
	databaseSeq atomic.Int64
)

type clusterState struct {
	admin string
	err   error
}

// Database is a fresh database with the store migrations and the reference schema.
type Database struct {
	URL  string
	Pool *pgxpool.Pool
}

// NewDatabase returns an isolated database on a Postgres container shared
// by the test binary.
func NewDatabase(t testing.TB) *Database {
	t.Helper()
	clusterOnce.Do(func() { cluster = startCluster() })
	if cluster.err != nil {
		t.Fatalf("postgres: %v", cluster.err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := fmt.Sprintf("conformance_%d_%d", os.Getpid(), databaseSeq.Add(1))
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
	if err := applyMigrations(ctx, template); err != nil {
		return err
	}
	_, err = template.Exec(ctx, reference.Schema)
	return err
}

func applyMigrations(ctx context.Context, conn *pgx.Conn) error {
	files, err := filepath.Glob(filepath.Join(ModuleRoot(), "..", "sql", "migrations", "V*__*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations in modules/sql/migrations")
	}
	slices.SortFunc(files, func(a, b string) int { return migrationVersion(a) - migrationVersion(b) })
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

func migrationVersion(file string) int {
	name := strings.TrimPrefix(filepath.Base(file), "V")
	n, _ := strconv.Atoi(name[:strings.Index(name, "__")])
	return n
}

// ModuleRoot returns the modules/conformance directory.
func ModuleRoot() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "..", "..")
}

func withDatabase(url, name string) string {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.User, config.Password, config.Host, config.Port, name)
}
