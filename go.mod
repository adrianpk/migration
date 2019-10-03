module gitlab.com/mikrowezel/migration

go 1.12

require (
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	gitlab.com/mikrowezel/config v0.0.0-00010101000000-000000000000
	google.golang.org/appengine v1.6.4 // indirect
)

replace gitlab.com/mikrowezel/config => ../config
