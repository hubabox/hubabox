package server

import (
	"net/url"
	"time"

	"github.com/kros/hubabox/internal/filemeta"
	"github.com/kros/hubabox/internal/files"
)

func (s *Server) buildFileRowsFromEntries(entries []files.FileEntry, downloadPrefix, previewPrefix, insightPrefix string, newSince *time.Time) []fileRow {
	now := time.Now()
	rows := make([]fileRow, 0, len(entries))
	for _, e := range entries {
		isNew := false
		if newSince != nil && e.ModTime.After(*newSince) {
			isNew = true
		}
		kind := filemeta.Kind(e.Name)
		previewURL := ""
		if filemeta.Previewable(e.Name) {
			previewURL = previewPrefix + url.PathEscape(e.Name)
		}
		rows = append(rows, fileRow{
			Name:                      e.Name,
			URL:                       downloadPrefix + url.PathEscape(e.Name),
			PreviewURL:                previewURL,
			InsightURL:                insightPrefix + url.PathEscape(e.Name),
			Kind:                      kind,
			KindLabel:                 filemeta.KindLabel(kind),
			SizeHuman:                 filemeta.HumanSize(e.Size),
			Age:                       filemeta.RelativeAge(e.ModTime, now),
			IsNew:                     isNew,
			BrowserPreviewUnavailable: kind == "video" && previewURL == "",
		})
	}
	return rows
}
