package source

import (
	"fmt"

	"mangarr/internal/domain"
)

type constructor func(domain.MonitoredManga) domain.Source

var registry = map[string]constructor{
	"asurascans": func(m domain.MonitoredManga) domain.Source { return NewAsurascans(m.Manga) },
	"atsumaru":   func(m domain.MonitoredManga) domain.Source { return NewAtsumaru(m.Manga, m.Group) },
	"comix":      func(m domain.MonitoredManga) domain.Source { return NewComix(m.Manga, m.Group) },
	"cubari":     func(m domain.MonitoredManga) domain.Source { return NewCubari(m.Manga, m.Group) },
	"flamecomics": func(m domain.MonitoredManga) domain.Source {
		return NewFlamecomics(m.Manga)
	},
	"mangadex": func(m domain.MonitoredManga) domain.Source {
		return NewMangadex(m.Manga, m.Group, m.Language)
	},
	"mangaplus":   func(m domain.MonitoredManga) domain.Source { return NewMangaPlus(m.Manga) },
	"tcbscans":    func(m domain.MonitoredManga) domain.Source { return NewTCBScans(m.Manga) },
	"weebcentral": func(m domain.MonitoredManga) domain.Source { return NewWeebCentral(m.Manga) },
}

func Select(monitoredManga domain.MonitoredManga) (domain.Source, error) {
	newSource, ok := registry[monitoredManga.Source]
	if !ok {
		return nil, fmt.Errorf("unknown monitored manga source %s", monitoredManga.Source)
	}

	return newSource(monitoredManga), nil
}
