# Migration

## Notes
* API not full polished/implemeted.
* Only PostgreSQL for now but changes to make it work with other databases should be trivial.
* Sample usage:
  * [Code](https://github.com/adrianpk/granica/tree/master/internal/migration)
  * [Tests](https://github.com/adrianpk/granica/blob/master/internal/repo/user_test.go)

## Next
* Group migration and associated rollback into a single structure.
* Map both to a single name.
* Remove unneded log messages.
* Implement pending methods.
* ...
