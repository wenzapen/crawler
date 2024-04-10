package sqldb

import "database/sql"

type DBer interface {
	CreateTable(t TableData) error
	Insert(t TableData) error
}

type Sqldb struct {
	options
	db *sql.DB
}
