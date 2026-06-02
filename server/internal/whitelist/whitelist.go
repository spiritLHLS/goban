package whitelist

import (
	"strings"

	"github.com/spiritlhl/goban/internal/models"
)

type Matcher struct {
	uids   map[int64]struct{}
	unames map[string]struct{}
}

func NewMatcher(rows []models.WhitelistUser) Matcher {
	m := Matcher{
		uids:   map[int64]struct{}{},
		unames: map[string]struct{}{},
	}
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		if row.UID > 0 {
			m.uids[row.UID] = struct{}{}
		}
		if strings.TrimSpace(row.Uname) != "" {
			m.unames[strings.ToLower(strings.TrimSpace(row.Uname))] = struct{}{}
		}
	}
	return m
}

func (m Matcher) Contains(uid int64, uname string) bool {
	if uid > 0 {
		if _, ok := m.uids[uid]; ok {
			return true
		}
	}
	uname = strings.ToLower(strings.TrimSpace(uname))
	if uname == "" {
		return false
	}
	_, ok := m.unames[uname]
	return ok
}
