# Migration

## Use [draft]
### Install

**Download**
```shell
$ go get -u gitlab.com/mikrowezel/backend/migration
```

**Implement Exec interface**

Something like this
```go
package yourpkg

import (
	"github.com/jmoiron/sqlx"

	"gitlab.com/mikrowezel/backend/migration"
)

type (
	mig struct {
		name string
		up   migration.Fx
		down migration.Fx
		tx   *sqlx.Tx
	}
)

func (m *mig) Config(up migration.Fx, down migration.Fx) {
	m.up = up
	m.down = down
}

func (m *mig) GetName() (name string) {
	return m.name
}

func (m *mig) GetUp() (up migration.Fx) {
	return m.up
}

func (m *mig) GetDown() (down migration.Fx) {
	return m.down
}

func (m *mig) SetTx(tx *sqlx.Tx) {
	m.tx = tx
}

func (m *mig) GetTx() (tx *sqlx.Tx) {
	return m.tx
}
```

**Implement migrations**

They look something like this.
```go
package yourpkg

import "log"

// CreateUsersTable migration
func (m *mig) CreateUsersTable() error {
	tx := m.GetTx()

	st := `CREATE TABLE users
	(
		id UUID PRIMARY KEY,
		slug VARCHAR(36),
		username VARCHAR(32) UNIQUE,
		password_digest CHAR(128),
		email VARCHAR(255) UNIQUE,
	);`

	_, err := tx.Exec(st)
	if err != nil {
		log.Prinf("%s\n", err.Error())
		return err
	}

  return nil
}

// DropUsersTable rollback
func (m *mig) DropUsersTable() error {
	tx := m.GetTx()

	st := `DROP TABLE users;`

	_, err := tx.Exec(st)
	if err != nil {
		log.Prinf("%s\n", err.Error())
		return err
	}

	return nil
}
```

**Add them to Migrator**

```go
package yourpkg

import(
	"gitlab.com/mikrowezel/backend/config"
	"gitlab.com/mikrowezel/backend/migration"
)

// ...
func getMigrator() *migration.Migrator {
  cfg := getConfig()
	m := migration.Init(cfg)

	// Migrations
	// Enable PoVstgis
	mg := &mig{}
	mg.Config(mg.EnablePostgis, mg.DropPostgis)
	m.AddMigration(mg)

	// CreateUsersTable
	mg = &mig{}
	mg.Config(mg.CreateUsersTable, mg.DropUsersTable)
	m.AddMigration(mg)

	return m
}

func getConfig() *config.Config {
	cfg := &config.Config{}
	values := map[string]string{
		"pg.host":               "localhost",
		"pg.port":               "5432",
		"pg.schema":             "public",
		"pg.database":           "granica_test",
		"pg.user":               "granica",
		"pg.password":           "granica",
		"pg.backoff.maxentries": "3",
	}

	cfg.SetNamespace("appnamespace")
	cfg.SetValues(values)
	return cfg
}

// ...
```

**Execute them**
```go
// ...

func init(){
  m := getMigrator()

  // Migrate
  m.Migrate()

  // Rollback last
  m.Rollback()

  // Migrate again
  m.Migrate()

  // Last 5 migrations
  // Or less if number exceeds currently applied
  m.Rollback(5) // Default to one in this case

  // Migrate again
  m.Migrate()

  // Rollback all
  m.RollbackAll() // All migrations

  // Create database
  m.CreateDb()

  // Drop database
  m.DropDb()

  // Reset database
  // Drop, if exists, create and migrate.
  m.Reset()
}

```

## Notes
* API not full polished/implemeted.
* Only PostgreSQL for now but changes to make it work with other databases should be trivial.
* Sample usage:
  * [Code](https://github.com/adrianpk/granica/tree/master/internal/migration)
  * [Tests](https://github.com/adrianpk/granica/blob/master/internal/repo/user_test.go)

## Next
* G̶r̶o̶u̶p̶ ̶m̶i̶g̶r̶a̶t̶i̶o̶n̶ ̶a̶n̶d̶ ̶a̶s̶s̶o̶c̶i̶a̶t̶e̶d̶ ̶r̶o̶l̶l̶b̶a̶c̶k̶ ̶i̶n̶t̶o̶ ̶a̶ ̶s̶i̶n̶g̶l̶e̶ ̶s̶t̶r̶u̶c̶t̶u̶r̶e̶.̶
* M̶a̶p̶ ̶b̶o̶t̶h̶ ̶t̶o̶ ̶a̶ ̶s̶i̶n̶g̶l̶e̶ ̶n̶a̶m̶e̶.̶
* Remove unneded log messages.
* Implement pending methods.
* ...
