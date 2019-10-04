package migration

import (
	"fmt"
	"log"
	"os"
	"reflect"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // package init.
	"gitlab.com/mikrowezel/config"
)

type (
	Migrator struct {
		cfg  *config.Config
		conn *sqlx.DB
		up   []*Migration
		down []*Migration
	}

	Exec interface {
		SetFx(func() (string, error))
		SetTx(*sqlx.Tx)
		GetTx() *sqlx.Tx
		SetErr(error)
		GetErr() error
	}

	Migration struct {
		Order    int
		Executor Exec
	}
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

func (m *Migrator) AddMigration(e Exec) {
	m.AddUp(&Migration{Executor: e})
}

func (m *Migrator) AddRollback(e Exec) {
	m.AddDown(&Migration{Executor: e})
}

//func (m *Migrator) AddMigration(f func() (string, error)) {
//m.AddUp(&Migration{Fx: f})
//}

//func (m *Migrator) AddRollback(f func() (string, error)) {
//m.AddDown(&Migration{Fx: f})
//}

func (m *Migrator) GetTx() *sqlx.Tx {
	return m.conn.MustBegin()
}

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

func (m *Migrator) CreateDb() error {
	return nil
}

func (m *Migrator) DropDb() error {
	return nil
}

func (m *Migrator) Reset() error {
	err := m.DropDb()
	if err != nil {
		log.Printf("Drop database error: %s", err.Error())
		// Do't return maybe it was not created before.
	}

	err = m.CreateDb()
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

func (m *Migrator) AddUp(mg *Migration) {
	m.up = append(m.up, mg)
}

func (m *Migrator) AddDown(rb *Migration) {
	m.down = append(m.down, rb)
}

func (m *Migrator) MigrateAll() error {
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
			log.Println("Not a valid migration function name")
		}
		// Read error
		err, ok := values[1].Interface().(error)
		if !ok && err != nil {
			log.Printf("Migration not executed: %s\n", name)
			log.Printf("Err  %+v' of type %T", err, err)
		}

		log.Printf("Migration executed: %s\n", name)
	}

	return nil
}

func (m *Migrator) RollbackAll() error {
	top := len(m.down) - 1
	for i := top; i >= 0; i-- {
		mg := m.down[i]
		exec := mg.Executor
		exec.SetTx(m.GetTx())
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

func (m *Migrator) dbURL() string {
	host := m.cfg.ValOrDef("pg.host", "localhost")
	port := m.cfg.ValOrDef("pg.port", "5432")
	db := m.cfg.ValOrDef("pg.database", "granica_test_d1x89s0l")
	user := m.cfg.ValOrDef("pg.user", "granica")
	pass := m.cfg.ValOrDef("pg.password", "granica")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, db)
}
