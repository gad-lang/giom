package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gad-lang/gad"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	giom "github.com/gad-lang/giom"
	giomnode "github.com/gad-lang/giom/node"
	giomparser "github.com/gad-lang/giom/parser"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultAddr = ":8080"

type App struct {
	DB           *gorm.DB
	Root         string
	PublicDir    string
	TranspileDir string
}

type Page struct {
	gorm.Model
	Title      string `json:"title"`
	Slug       string `json:"slug" gorm:"uniqueIndex"`
	Summary    string `json:"summary"`
	Body       string `json:"body"`
	CoverImage string `json:"coverImage"`
	Images     string `json:"images"`
	Published  bool   `json:"published"`
}

type Tag struct {
	gorm.Model
	Name string `json:"name"`
	Slug string `json:"slug" gorm:"uniqueIndex"`
}

type Post struct {
	gorm.Model
	Title      string `json:"title"`
	Slug       string `json:"slug" gorm:"uniqueIndex"`
	Summary    string `json:"summary"`
	Body       string `json:"body"`
	RightBody  string `json:"rightBody"`
	CoverImage string `json:"coverImage"`
	Images     string `json:"images"`
	Published  bool   `json:"published"`
	Tags       []Tag  `json:"tags" gorm:"many2many:post_tags"`
}

type MenuItem struct {
	gorm.Model
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	PageID   *uint  `json:"pageId"`
	Page     *Page  `json:"page"`
	TagID    *uint  `json:"tagId"`
	Tag      *Tag   `json:"tag"`
	URL      string `json:"url"`
	Position int    `json:"position"`
	Visible  bool   `json:"visible"`
}

// seed.yaml model types

type SeedData struct {
	Pages []SeedPage     `yaml:"pages"`
	Tags  []SeedTag      `yaml:"tags"`
	Posts []SeedPost     `yaml:"posts"`
	Menu  []SeedMenuItem `yaml:"menu"`
}

type SeedPage struct {
	Title      string   `yaml:"title"`
	Slug       string   `yaml:"slug"`
	Summary    string   `yaml:"summary"`
	Body       string   `yaml:"body"`
	CoverImage string   `yaml:"coverImage"`
	Images     []string `yaml:"images"`
	Published  bool     `yaml:"published"`
}

type SeedTag struct {
	Name string `yaml:"name"`
	Slug string `yaml:"slug"`
}

type SeedPost struct {
	Title      string   `yaml:"title"`
	Slug       string   `yaml:"slug"`
	Summary    string   `yaml:"summary"`
	Body       string   `yaml:"body"`
	RightBody  string   `yaml:"rightBody"`
	CoverImage string   `yaml:"coverImage"`
	Images     []string `yaml:"images"`
	Published  bool     `yaml:"published"`
	Tags       []string `yaml:"tags"`
}

type SeedMenuItem struct {
	Label    string `yaml:"label"`
	Kind     string `yaml:"kind"`
	Page     string `yaml:"page"`
	Tag      string `yaml:"tag"`
	URL      string `yaml:"url"`
	Position int    `yaml:"position"`
	Visible  bool   `yaml:"visible"`
}

func main() {
	app, err := NewApp(".")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	app.routes(mux)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	log.Printf("cms example listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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

func (a *App) routes(mux *http.ServeMux) {
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(a.Root, "static")))))
	mux.Handle("/seed-data/", http.StripPrefix("/seed-data/", http.FileServer(http.Dir(filepath.Join(a.Root, "seed-data")))))
	mux.HandleFunc("/admin", a.admin)
	mux.HandleFunc("/admin/", a.admin)
	mux.HandleFunc("/api/dashboard", a.apiDashboard)
	mux.HandleFunc("/api/pages", a.collection(&Page{}))
	mux.HandleFunc("/api/pages/", a.member(&Page{}))
	mux.HandleFunc("/api/posts", a.postsCollection)
	mux.HandleFunc("/api/posts/", a.postMember)
	mux.HandleFunc("/api/tags", a.collection(&Tag{}))
	mux.HandleFunc("/api/tags/", a.member(&Tag{}))
	mux.HandleFunc("/api/menus", a.collection(&MenuItem{}))
	mux.HandleFunc("/api/menus/", a.member(&MenuItem{}))
	mux.HandleFunc("/pages/", a.page)
	mux.HandleFunc("/posts/", a.post)
	mux.HandleFunc("/tags/", a.tagPosts)
	mux.HandleFunc("/", a.index)
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	posts, err := a.latestPosts(5)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "index.giom", a.model("Home", []crumb{{"Home", "/"}}, gad.Dict{
		"Posts": postsToGad(posts),
	}))
}

func (a *App) page(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/pages/")
	var p Page
	if err := a.DB.Where("slug = ? AND published = ?", slug, true).First(&p).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, "page.giom", a.model(p.Title, []crumb{{"Home", "/"}, {p.Title, "/pages/" + p.Slug}}, gad.Dict{
		"Page": pageToGad(p),
	}))
}

func (a *App) post(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/posts/")
	var p Post
	if err := a.DB.Preload("Tags").Where("slug = ? AND published = ?", slug, true).First(&p).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, "post.giom", a.model(p.Title, []crumb{{"Home", "/"}, {"Posts", "/"}, {p.Title, "/posts/" + p.Slug}}, gad.Dict{
		"Post": postToGad(p),
	}))
}

func (a *App) tagPosts(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/tags/")
	pageNo := queryInt(r, "page", 1)
	if pageNo < 1 {
		pageNo = 1
	}
	var tag Tag
	if err := a.DB.Where("slug = ?", slug).First(&tag).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	posts, total, err := a.postsByTag(tag.ID, pageNo, 5)
	if err != nil {
		a.serverError(w, err)
		return
	}
	totalPages := int((total + 4) / 5)
	a.render(w, "tag.giom", a.model(tag.Name, []crumb{{"Home", "/"}, {tag.Name, "/tags/" + tag.Slug}}, gad.Dict{
		"Tag":   tagToGad(tag),
		"Posts": postsToGad(posts),
		"Pager": gad.Dict{
			"Page":       gad.Int(pageNo),
			"TotalPages": gad.Int(totalPages),
			"HasPrev":    gad.Bool(pageNo > 1),
			"HasNext":    gad.Bool(pageNo < totalPages),
			"PrevURL":    gad.Str(fmt.Sprintf("/tags/%s?page=%d", tag.Slug, pageNo-1)),
			"NextURL":    gad.Str(fmt.Sprintf("/tags/%s?page=%d", tag.Slug, pageNo+1)),
		},
	}))
}

func (a *App) render(w http.ResponseWriter, name string, model gad.Dict) {
	src, err := a.loadTemplate(name)
	if err != nil {
		a.serverError(w, err)
		return
	}
	builtins := giom.AppendBuiltins(gad.NewBuiltins())
	st := gad.NewSymbolTable(builtins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		a.serverError(w, err)
		return
	}
	_, bc, err := giom.Compile(st, []byte(src), gad.CompileOptions{})
	if err != nil {
		a.serverError(w, fmt.Errorf("compile %s: %w", name, err))
		return
	}
	var out bytes.Buffer
	vm := gad.NewVM(builtins.Build(), bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out, Globals: gad.Dict{"Model": model}})
	if err != nil {
		a.serverError(w, fmt.Errorf("render %s: %w", name, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out.Bytes())
}

var importLine = regexp.MustCompile(`(?m)^@import\s+"([^"]+)"\s*$`)

func (a *App) loadTemplate(name string) (string, error) {
	fullSrc, err := a.resolveImports(name, map[string]bool{})
	if err != nil {
		return "", err
	}
	a.transpile(name, fullSrc)
	return fullSrc, nil
}

func (a *App) resolveImports(name string, seen map[string]bool) (string, error) {
	clean := filepath.Clean(name)
	if seen[clean] {
		return "", nil
	}
	seen[clean] = true
	path := filepath.Join(a.PublicDir, clean)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", clean, err)
	}
	src := string(b)
	var imports strings.Builder
	for _, m := range importLine.FindAllStringSubmatch(src, -1) {
		part, err := a.resolveImports(m[1], seen)
		if err != nil {
			return "", err
		}
		imports.WriteString(part)
		if !strings.HasSuffix(part, "\n") {
			imports.WriteByte('\n')
		}
	}
	src = importLine.ReplaceAllString(src, "")
	return imports.String() + src, nil
}

func (a *App) transpile(name, src string) {
	transpiledName := strings.TrimSuffix(name, ".giom") + ".gad"
	outPath := filepath.Join(a.TranspileDir, transpiledName)
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)

	fs := source.NewFileSet()
	f := fs.AddFileData(name, -1, []byte(src))
	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	if err != nil {
		log.Printf("transpile parse %s: %v", name, err)
		return
	}
	stmts := giomnode.Convert(file.Stmts)
	var buf bytes.Buffer
	gnode.CodeW(&buf, stmts, gnode.CodeWithPrefix("\t"), gnode.CodeFormat())
	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		log.Printf("transpile write %s: %v", transpiledName, err)
	}
}

type crumb struct{ Label, URL string }

func (a *App) model(title string, crumbs []crumb, values gad.Dict) gad.Dict {
	model := gad.Dict{
		"SiteTitle":   gad.Str("GION CMS"),
		"Title":       gad.Str(title),
		"Menu":        a.menuGad(),
		"Breadcrumbs": crumbsToGad(crumbs),
		"Year":        gad.Int(time.Now().Year()),
	}
	for k, v := range values {
		model[k] = v
	}
	return model
}

func (a *App) menuGad() gad.Array {
	var items []MenuItem
	a.DB.Preload("Page").Preload("Tag").Where("visible = ?", true).Order("position ASC, id ASC").Find(&items)
	arr := make(gad.Array, 0, len(items))
	for _, item := range items {
		url := item.URL
		if item.Kind == "page" && item.Page != nil {
			url = "/pages/" + item.Page.Slug
		}
		if item.Kind == "tag" && item.Tag != nil {
			url = "/tags/" + item.Tag.Slug
		}
		arr = append(arr, gad.Dict{"Label": gad.Str(item.Label), "URL": gad.Str(url), "Kind": gad.Str(item.Kind)})
	}
	return arr
}

func (a *App) latestPosts(limit int) ([]Post, error) {
	var posts []Post
	err := a.DB.Preload("Tags").Where("published = ?", true).Order("created_at DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

func (a *App) postsByTag(tagID uint, page, limit int) ([]Post, int64, error) {
	var total int64
	base := a.DB.Model(&Post{}).Joins("JOIN post_tags ON post_tags.post_id = posts.id").Where("posts.published = ? AND post_tags.tag_id = ?", true, tagID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var posts []Post
	err := base.Preload("Tags").Order("posts.created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&posts).Error
	return posts, total, err
}

func pageToGad(p Page) gad.Dict {
	return gad.Dict{"ID": gad.Uint(p.ID), "Title": gad.Str(p.Title), "Slug": gad.Str(p.Slug), "Summary": gad.Str(p.Summary), "Body": gad.RawStr(template.HTML(p.Body)), "CoverImage": gad.Str(p.CoverImage), "Images": imagesToGad(p.Images)}
}

func postToGad(p Post) gad.Dict {
	return gad.Dict{"ID": gad.Uint(p.ID), "Title": gad.Str(p.Title), "Slug": gad.Str(p.Slug), "Summary": gad.Str(p.Summary), "Body": gad.RawStr(template.HTML(p.Body)), "RightBody": gad.RawStr(template.HTML(p.RightBody)), "CoverImage": gad.Str(p.CoverImage), "Images": imagesToGad(p.Images), "Tags": tagsToGad(p.Tags), "URL": gad.Str("/posts/" + p.Slug)}
}

func tagToGad(t Tag) gad.Dict {
	return gad.Dict{"ID": gad.Uint(t.ID), "Name": gad.Str(t.Name), "Slug": gad.Str(t.Slug), "URL": gad.Str("/tags/" + t.Slug)}
}

func postsToGad(posts []Post) gad.Array {
	arr := make(gad.Array, 0, len(posts))
	for _, p := range posts {
		arr = append(arr, postToGad(p))
	}
	return arr
}

func tagsToGad(tags []Tag) gad.Array {
	arr := make(gad.Array, 0, len(tags))
	for _, t := range tags {
		arr = append(arr, tagToGad(t))
	}
	return arr
}

func imagesToGad(s string) gad.Array {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' })
	arr := make(gad.Array, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			arr = append(arr, gad.Str(part))
		}
	}
	return arr
}

func crumbsToGad(crumbs []crumb) gad.Array {
	arr := make(gad.Array, 0, len(crumbs))
	for _, c := range crumbs {
		arr = append(arr, gad.Dict{"Label": gad.Str(c.Label), "URL": gad.Str(c.URL)})
	}
	return arr
}

func queryInt(r *http.Request, key string, fallback int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return n
}

func (a *App) admin(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/admin/src/") {
		name := strings.TrimPrefix(r.URL.Path, "/admin/src/")
		http.ServeFile(w, r, filepath.Join(a.Root, "admin", "src", filepath.Clean(name)))
		return
	}
	http.ServeFile(w, r, filepath.Join(a.Root, "admin", "index.html"))
}

func (a *App) apiDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	var pages, posts, tags, menus int64
	a.DB.Model(&Page{}).Count(&pages)
	a.DB.Model(&Post{}).Count(&posts)
	a.DB.Model(&Tag{}).Count(&tags)
	a.DB.Model(&MenuItem{}).Count(&menus)
	writeJSON(w, gad.Dict{"pages": gad.Int(pages), "posts": gad.Int(posts), "tags": gad.Int(tags), "menus": gad.Int(menus)})
}

func (a *App) collection(model any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			a.list(w, model)
		case http.MethodPost:
			a.create(w, r, model)
		default:
			methodNotAllowed(w)
		}
	}
}

func (a *App) member(model any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idFromPath(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			a.update(w, r, model, id)
		case http.MethodDelete:
			a.delete(w, model, id)
		default:
			methodNotAllowed(w)
		}
	}
}

func (a *App) list(w http.ResponseWriter, model any) {
	switch model.(type) {
	case *Page:
		var rows []Page
		a.DB.Order("id DESC").Find(&rows)
		writeJSON(w, rows)
	case *Tag:
		var rows []Tag
		a.DB.Order("name ASC").Find(&rows)
		writeJSON(w, rows)
	case *MenuItem:
		var rows []MenuItem
		a.DB.Preload("Page").Preload("Tag").Order("position ASC, id ASC").Find(&rows)
		writeJSON(w, rows)
	}
}

func (a *App) create(w http.ResponseWriter, r *http.Request, model any) {
	if err := json.NewDecoder(r.Body).Decode(model); err != nil {
		badRequest(w, err)
		return
	}
	if err := a.DB.Create(model).Error; err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, model)
}

func (a *App) update(w http.ResponseWriter, r *http.Request, model any, id uint) {
	if err := a.DB.First(model, id).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(model); err != nil {
		badRequest(w, err)
		return
	}
	if err := a.DB.Save(model).Error; err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, model)
}

func (a *App) delete(w http.ResponseWriter, model any, id uint) {
	if err := a.DB.Delete(model, id).Error; err != nil {
		a.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) postsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var posts []Post
		a.DB.Preload("Tags").Order("id DESC").Find(&posts)
		writeJSON(w, posts)
	case http.MethodPost:
		var input Post
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			badRequest(w, err)
			return
		}
		if err := a.savePost(&input); err != nil {
			a.serverError(w, err)
			return
		}
		writeJSON(w, input)
	default:
		methodNotAllowed(w)
	}
}

func (a *App) postMember(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input Post
		if err := a.DB.Preload("Tags").First(&input, id).Error; err != nil {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			badRequest(w, err)
			return
		}
		input.ID = id
		if err := a.savePost(&input); err != nil {
			a.serverError(w, err)
			return
		}
		writeJSON(w, input)
	case http.MethodDelete:
		a.delete(w, &Post{}, id)
	default:
		methodNotAllowed(w)
	}
}

func (a *App) savePost(post *Post) error {
	tags := post.Tags
	post.Tags = nil
	if post.ID == 0 {
		if err := a.DB.Create(post).Error; err != nil {
			return err
		}
	} else if err := a.DB.Save(post).Error; err != nil {
		return err
	}
	return a.DB.Model(post).Association("Tags").Replace(tags)
}

func idFromPath(path string) (uint, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return 0, errors.New("missing id")
	}
	n, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	return uint(n), err
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json response failed: %v", err)
	}
}

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (a *App) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (a *App) seed() error {
	data, err := os.ReadFile(filepath.Join(a.Root, "seed.yaml"))
	if err != nil {
		return fmt.Errorf("read seed.yaml: %w", err)
	}
	var sd SeedData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("parse seed.yaml: %w", err)
	}
	return a.importSeedData(&sd)
}

func (a *App) importSeedData(sd *SeedData) error {
	// tags first (no dependencies)
	var tags []Tag
	tagByName := map[string]*Tag{}
	tagBySlug := map[string]*Tag{}
	for _, st := range sd.Tags {
		t := Tag{Name: st.Name, Slug: st.Slug}
		tags = append(tags, t)
	}
	if err := a.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(&tags).Error; err != nil {
		return fmt.Errorf("seed tags: %w", err)
	}
	// Reload tags from DB to get any existing ones.
	var allTags []Tag
	a.DB.Find(&allTags)
	for i := range allTags {
		tagByName[allTags[i].Name] = &allTags[i]
		tagBySlug[allTags[i].Slug] = &allTags[i]
	}

	// pages next (no dependencies)
	var pages []Page
	pageBySlug := map[string]*Page{}
	for _, sp := range sd.Pages {
		p := Page{
			Title:      sp.Title,
			Slug:       sp.Slug,
			Summary:    sp.Summary,
			Body:       sp.Body,
			CoverImage: sp.CoverImage,
			Images:     strings.Join(sp.Images, "\n"),
			Published:  sp.Published,
		}
		pages = append(pages, p)
	}
	if err := a.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(&pages).Error; err != nil {
		return fmt.Errorf("seed pages: %w", err)
	}
	var allPages []Page
	a.DB.Find(&allPages)
	for i := range allPages {
		pageBySlug[allPages[i].Slug] = &allPages[i]
	}

	// posts (depend on tags)
	for _, sp := range sd.Posts {
		exists := int64(0)
		a.DB.Model(&Post{}).Where("slug = ?", sp.Slug).Count(&exists)
		if exists > 0 {
			continue
		}
		p := Post{
			Title:      sp.Title,
			Slug:       sp.Slug,
			Summary:    sp.Summary,
			Body:       sp.Body,
			RightBody:  sp.RightBody,
			CoverImage: sp.CoverImage,
			Images:     strings.Join(sp.Images, "\n"),
			Published:  sp.Published,
		}
		if err := a.DB.Create(&p).Error; err != nil {
			return fmt.Errorf("seed post %q: %w", sp.Slug, err)
		}
		var postTags []*Tag
		for _, tname := range sp.Tags {
			if t, ok := tagByName[tname]; ok {
				postTags = append(postTags, t)
			}
		}
		if err := a.DB.Model(&p).Association("Tags").Replace(postTags); err != nil {
			return fmt.Errorf("seed post %q tags: %w", sp.Slug, err)
		}
	}

	// menu items (depend on pages and tags)
	var menuItems []MenuItem
	for _, sm := range sd.Menu {
		mi := MenuItem{
			Label:    sm.Label,
			Kind:     sm.Kind,
			Position: sm.Position,
			Visible:  sm.Visible,
		}
		switch sm.Kind {
		case "url":
			mi.URL = sm.URL
		case "page":
			if p, ok := pageBySlug[sm.Page]; ok {
				mi.PageID = &p.ID
				mi.Page = p
			}
		case "tag":
			if t, ok := tagBySlug[sm.Tag]; ok {
				mi.TagID = &t.ID
				mi.Tag = t
			}
		}
		menuItems = append(menuItems, mi)
	}
	if len(menuItems) > 0 {
		a.DB.Where("1 = 1").Delete(&MenuItem{})
		return a.DB.Create(&menuItems).Error
	}
	return nil
}
