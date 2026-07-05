package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type App struct {
	DB           *gorm.DB
	Root         string
	PublicDir    string
	TranspileDir string
}

func NewApp(root string) (*App, error) {
	dbPath := filepath.Join(root, "cms.db")
	_, err := os.Stat(dbPath)
	firstRun := os.IsNotExist(err)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	app := &App{
		DB:           db,
		Root:         root,
		PublicDir:    filepath.Join(root, "public"),
		TranspileDir: filepath.Join(root, "public", ".transpiled"),
	}
	app.cleanTranspiled()
	if err := app.DB.AutoMigrate(&Page{}, &Tag{}, &Post{}, &MenuItem{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if firstRun {
		if err := app.seed(); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func (a *App) cleanTranspiled() {
	os.RemoveAll(a.TranspileDir)
}
