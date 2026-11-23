package domain

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Newterios/Examory/services/platform/ids"
)

var (
	ErrInvalid   = errors.New("invalid course data")
	ErrNotFound  = errors.New("course or material not found")
	ErrConflict  = errors.New("course revision has changed")
	ErrForbidden = errors.New("course access denied")
)

type Material struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}
type Module struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Materials []Material `json:"materials"`
}
type Course struct {
	ID       string   `json:"id"`
	Revision int      `json:"revision"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Modules  []Module `json:"modules"`
}

func Title(s string) bool  { return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 200 }
func status(s string) bool { return s == "draft" || s == "published" || s == "archived" }

// Client-assigned IDs may refer only to existing items from this course.
// Fresh IDs are generated server-side, so one course cannot import another's progress.
func (c *Course) Prepare(old Course) error {
	if !Title(c.Title) || !status(c.Status) || len(c.Modules) > 100 {
		return ErrInvalid
	}
	known := map[string]string{}
	for _, m := range old.Modules {
		known[m.ID] = "module"
		for _, v := range m.Materials {
			known[v.ID] = "material"
		}
	}
	seen := map[string]bool{}
	prepareID := func(id *string, kind string) bool {
		if *id == "" {
			*id = ids.NewV7()
		} else if known[*id] != kind {
			return false
		}
		if seen[*id] {
			return false
		}
		seen[*id] = true
		return true
	}
	if c.Modules == nil {
		c.Modules = []Module{}
	}
	for i := range c.Modules {
		m := &c.Modules[i]
		if !prepareID(&m.ID, "module") || !Title(m.Title) || !status(m.Status) || len(m.Materials) > 150 {
			return ErrInvalid
		}
		if m.Materials == nil {
			m.Materials = []Material{}
		}
		for j := range m.Materials {
			v := &m.Materials[j]
			if !prepareID(&v.ID, "material") || !Title(v.Title) || !status(v.Status) || strings.TrimSpace(v.Content) == "" || len(v.Content) > 65536 {
				return ErrInvalid
			}
			switch v.Kind {
			case "text":
			case "link":
				u, e := url.ParseRequestURI(v.Content)
				if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
					return ErrInvalid
				}
			default:
				return ErrInvalid
			}
		}
	}
	return nil
}
func (c Course) Published() Course {
	mods := []Module{}
	for _, m := range c.Modules {
		if m.Status != "published" {
			continue
		}
		vs := []Material{}
		for _, v := range m.Materials {
			if v.Status == "published" {
				vs = append(vs, v)
			}
		}
		m.Materials = vs
		mods = append(mods, m)
	}
	c.Modules = mods
	return c
}
func (c Course) FindPublished(material string) (string, bool) {
	if c.Status != "published" {
		return "", false
	}
	for _, m := range c.Modules {
		if m.Status != "published" {
			continue
		}
		for _, v := range m.Materials {
			if v.ID == material && v.Status == "published" {
				return m.ID, true
			}
		}
	}
	return "", false
}
