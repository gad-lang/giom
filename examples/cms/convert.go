package main

import (
	"html/template"
	"strings"

	"github.com/gad-lang/gad"
)

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
