package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // package init.
	"github.com/twinj/uuid"
	"gitlab.com/mikrowezel/backend/config"
	"gitlab.com/mikrowezel/backend/db"
	"gitlab.com/mikrowezel/backend/db/postgres"
)

type (
	// Migrator struct.
	Migrator struct {
		cfg    *config.Config
		conn   *sqlx.DB
		schema string
		db     string
		up     []*Migration
		down   []*Migration
	}

	// Exec interface.
	Exec interface {
		SetFx(func() (funcName string, err error))
		SetTx(tx *sqlx.Tx)
		GetTx() *sqlx.Tx
		SetErr(err error)
		GetErr() error
	}

	// Migration struct.
	Migration struct {
		Order    int
		Executor Exec
	}

	migRecord struct {
		ID        uuid.UUID      `db:"id" json:"id"`
		Name      sql.NullString `db:"name" json:"name"`
		IsApplied sql.NullBool   `db:"is_applied" json:"isApplied"`
		CreatedAt pq.NullTime    `db:"created_at" json:"createdAt"`
	}
)

const (
	pgMigrationsTable = "migrations"

	pgCreateDbSt = `
		CREATE DATABASE %s.%s;`

	pgDropDbSt = `
		DROP DATABASE %s.%s;`

	pgCreateMigrationsSt = `CREATE TABLE %s.%s (
		id UUID PRIMARY KEY,
		name VARCHAR(32),
 		is_applied BOOLEAN,
		created_at TIMESTAMP
	);`

	pgDropMigrationsSt = `DROP TABLE %s.%s;`

	pgRegMigrationSt = `INSERT INTO %s.%s (id, name, is_applied, created_at)
		VALUES (:id, :name, :is_applied, :created_at);`
)

// Init to explicitly start the migrator.
func Init(cfg *config.Config) *Migrator {
	mig := &Migrator{cfg: cfg}
	err := mig.Connect()
	if err != nil {
		os.Exit(1)
	}

	return mig
}

// Connect to database.
func (m *Migrator) Connect() error {
	conn, err := sqlx.Open("postgres", m.dbURL())
	if err != nil {
		log.Printf("Connection error: %s\n", err.Error())
		return err
	}

	err = conn.Ping()
	if err != nil {
		log.Printf("Connection error: %s", err.Error())
		return err
	}

	m.conn = conn
	return nil
}

// GetTx returns a new transaction from migrator connection.
func (m *Migrator) GetTx() *sqlx.Tx {
	return m.conn.MustBegin()
}

// PreSetup creates database
// and migrations table if needed.
func (m *Migrator) PreSetup() {
	if !m.dbExists() {
		m.CreateDb()
	}

	if !m.migTableExists() {
		m.createMigrationsTable()
	}
}

// dbExists returns true if migrator
// referenced database has been already created.
// Only for postgress at the moment.
func (m *Migrator) dbExists() bool {
	st := fmt.Sprintf(`select exists(
		SELECT datname FROM pg_catalog.pg_database WHERE lower(datname) = lower('%s')
	);`, m.db)

	r, err := m.conn.Query(st)
	if err != nil {
		log.Printf("Error checking database: %s\n", err.Error())
		return false
	}

	for r.Next() {
		var exists sql.NullBool
		err = r.Scan(&exists)
		if err != nil {
			log.Printf("Cannot read query result: %s\n", err.Error())
			return false
		}
		return exists.Bool
	}
	return false
}

// migExists returns true if migrations table exists.
func (m *Migrator) migTableExists() bool {
	st := fmt.Sprintf(`SELECT EXISTS (
		SELECT 1
   	FROM   pg_catalog.pg_class c
   	JOIN   pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   	WHERE  n.nspname = '%s'
   	AND    c.relname = '%s'
   	AND    c.relkind = 'r'
	);`, m.schema, m.db)

	r, err := m.conn.Query(st)
	if err != nil {
		log.Printf("Error checking database: %s\n", err.Error())
		return false
	}

	for r.Next() {
		var exists sql.NullBool
		err = r.Scan(&exists)
		if err != nil {
			log.Printf("Cannot read query result: %s\n", err.Error())
			return false
		}

		return exists.Bool
	}
	return false
}

// CreateDb migration.
func (m *Migrator) CreateDb() (string, error) {
	tx := m.GetTx()

	st := fmt.Sprintf(pgCreateDbSt, m.schema, m.db)

	_, err := tx.Exec(st)
	if err != nil {
		return m.db, err
	}

	return m.db, nil
}

// DropDb migration.
func (m *Migrator) DropDb() (string, error) {
	tx := m.GetTx()

	st := fmt.Sprintf(pgDropDbSt, m.schema, m.db)

	_, err := tx.Exec(st)
	if err != nil {
		return m.db, err
	}

	return m.db, nil
}

// DropDb migration.
func (m *Migrator) createMigrationsTable() (string, error) {
	tx := m.GetTx()

	st := fmt.Sprintf(pgCreateMigrationsSt, m.schema, pgMigrationsTable)

	_, err := tx.Exec(st)
	if err != nil {
		return pgMigrationsTable, err
	}

	return pgMigrationsTable, tx.Commit()
}

func (m *Migrator) AddMigration(e Exec) {
	m.AddUp(&Migration{Executor: e})
}

func (m *Migrator) AddRollback(e Exec) {
	m.AddDown(&Migration{Executor: e})
}

func (m *Migrator) Reset(name string) error {
	_, err := m.DropDb()
	if err != nil {
		log.Printf("Drop database error: %s", err.Error())
		// Do't return maybe it was not created before.
	}

	_, err = m.CreateDb()
	if err != nil {
		log.Printf("Drop database error: %s", err.Error())
		return err
	}

	err = m.MigrateAll()
	if err != nil {
		log.Printf("Drop database error: %s", err.Error())
		return err
	}

	return nil
}

func (m *Migrator) CreateMigrations() bool {
	return false
}

// DropMigrations table.
func (m *Migrator) DropMigrations() bool {
	return false
}

func (m *Migrator) AddUp(mg *Migration) {
	m.up = append(m.up, mg)
}

func (m *Migrator) AddDown(rb *Migration) {
	m.down = append(m.down, rb)
}

func (m *Migrator) MigrateAll() error {
	m.PreSetup()

	for i, mg := range m.up {
		exec := mg.Executor
		tx := m.GetTx()
		exec.SetTx(tx)

		// Expected function name to execute
		fn := fmt.Sprintf("Up%08d", i+1)
		values := reflect.ValueOf(exec).MethodByName(fn).Call([]reflect.Value{})

		// Type assert result
		name, ok := values[0].Interface().(string)

		// Read name
		if !ok {
			tx.Rollback()
			return errors.New("Not a valid migration function name")
		}

		// Read error
		err, ok := values[1].Interface().(error)
		if !ok && err != nil {
			log.Printf("Migration not executed: %s\n", name) // TODO: Remove log
			log.Printf("Err  %+v' of type %T\n", err, err)   // TODO: Remove log.
			msg := fmt.Sprintf("cannot run migration '%s': %s", name, err.Error())
			tx.Rollback()
			return errors.New(msg)
		}

		// Register migration
		err = m.recMigration(tx, name)

		err = tx.Commit()
		if err != nil {
			msg := fmt.Sprintf("Cannot update migrations table: %s\n", err.Error())
			log.Printf("Commit error: %s", msg)
			tx.Rollback()
			return errors.New(msg)
		}

		log.Printf("Migration executed: %s\n", name)
	}

	return nil
}

func (m *Migrator) RollbackAll() error {
	top := len(m.down) - 1
	for i := top; i >= 0; i-- {
		tx := m.GetTx()
		mg := m.down[i]
		exec := mg.Executor
		exec.SetTx(tx)

		fn := fmt.Sprintf("Down%08d", i+1)
		values := reflect.ValueOf(exec).MethodByName(fn).Call([]reflect.Value{})

		// Type assert result
		name, ok := values[0].Interface().(string)

		// Read name
		if !ok {
			log.Println("Not a valid rollback function name")
		}

		// Read error
		err, ok := values[1].Interface().(error)
		if !ok && err != nil {
			log.Printf("Rollback not executed: %s\n", name)
			log.Printf("Err '%+v' of type %T", err, err)
		}

		err = tx.Commit()
		if err != nil {
			msg := fmt.Sprintf("Cannot update migrations table: %s\n", err.Error())
			log.Printf("Commit error: %s", msg)
			tx.Rollback()
			return errors.New(msg)
		}

		log.Printf("Rollback executed: %s\n", name)
	}

	return nil
}

func (m *Migrator) MigrateThis(mg Migration) error {
	return nil
}

func (m *Migrator) RollbackThis(r Migration) error {
	return nil
}

func (m *Migrator) recMigration(tx *sqlx.Tx, name string) error {
	st := fmt.Sprintf(pgRegMigrationSt, m.schema, pgMigrationsTable)
	_, err := tx.NamedExec(st, migRecord{
		ID:        uuid.NewV4(),
		Name:      db.ToNullString(name),
		IsApplied: db.ToNullBool(true),
		CreatedAt: postgres.ToNullTime(time.Now()),
	})

	if err != nil {
		msg := fmt.Sprintf("Cannot update migrations table: %s\n", err.Error())
		return errors.New(msg)
	}

	return nil
}

func (m *Migrator) dbURL() string {
	host := m.cfg.ValOrDef("pg.host", "localhost")
	port := m.cfg.ValOrDef("pg.port", "5432")
	m.schema = m.cfg.ValOrDef("pg.schema", "public")
	m.db = m.cfg.ValOrDef("pg.database", "granica_test_d1x89s0l")
	user := m.cfg.ValOrDef("pg.user", "granica")
	pass := m.cfg.ValOrDef("pg.password", "granica")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable search_path=%s", host, port, user, pass, m.db, m.schema)
}
