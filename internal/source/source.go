package source

import (
	"fmt"

	"mangarr/internal/domain"
)

func Select(monitoredManga domain.MonitoredManga) (domain.Source, error) {
	switch monitoredManga.Source {
	case "tcbscans":
		return NewTCBScans(monitoredManga.Manga), nil
	case "mangadex":
		return NewMangadex(monitoredManga.Manga, monitoredManga.Group, monitoredManga.Language), nil
	case "mangaplus":
		return NewMangaPlus(monitoredManga.Manga), nil
	case "flamecomics":
		return NewFlamecomics(monitoredManga.Manga), nil
	case "asurascans":
		return NewAsurascans(monitoredManga.Manga), nil
	case "cubari":
		return NewCubari(monitoredManga.Manga, monitoredManga.Group), nil
	}

	return nil, fmt.Errorf("unknown monitored manga source %s", monitoredManga.Source)
}
