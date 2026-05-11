package server

import (
	"net/url"
	"time"

	"github.com/kros/hubabox/internal/filemeta"
	"github.com/kros/hubabox/internal/files"
)

func (s *Server) buildFileRows(downloadPrefix string) ([]fileRow, error) {
	entries, err := files.ListEntries(s.filesDir)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows := make([]fileRow, 0, len(entries))
	for _, e := range entries {
		kind := filemeta.Kind(e.Name)
		rows = append(rows, fileRow{
			Name:      e.Name,
			URL:       downloadPrefix + url.PathEscape(e.Name),
			Kind:      kind,
			KindLabel: filemeta.KindLabel(kind),
			SizeHuman: filemeta.HumanSize(e.Size),
			Age:       filemeta.RelativeAge(e.ModTime, now),
		})
	}
	return rows, nil
}
