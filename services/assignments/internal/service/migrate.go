package service

import (
	"embed"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"io/fs"
)

//go:embed migrations/*.sql
var files embed.FS

func Migrations() []migrate.Set {
	m, err := fs.Sub(files, "migrations")
	if err != nil {
		panic(err)
	}
	return []migrate.Set{outbox.Migrations(), {Name: "assignments", FS: m}}
}
