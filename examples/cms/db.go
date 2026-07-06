package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gad-lang/gad"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type App struct {
	DB            *gorm.DB
	Root          string
	PublicDir     string
	TranspileDir  string
	TemplateDelay time.Duration

	mu            sync.Mutex
	templateCache map[string]*templateCacheEntry
}

type templateCacheEntry struct {
	bc        *gad.Bytecode
	builtins  *gad.Builtins
	files     map[string]time.Time
	changedAt time.Time
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
		DB:            db,
		Root:          root,
		PublicDir:     filepath.Join(root, "public"),
		TranspileDir:  filepath.Join(root, "public", ".transpiled"),
		TemplateDelay: 5 * time.Second,
		templateCache: make(map[string]*templateCacheEntry),
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
