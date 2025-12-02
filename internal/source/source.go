package source

import (
	"fmt"

	"mangarr/internal/browser"
	"mangarr/internal/domain"
)

func Select(monitoredManga domain.MonitoredManga, bm *browser.Manager) (domain.Source, error) {
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
		return NewAsurascans(monitoredManga.Manga, bm), nil
	case "cubari":
		return NewCubari(monitoredManga.Manga, monitoredManga.Group), nil
	case "comick":
		return NewComick(monitoredManga.Manga, monitoredManga.Group, monitoredManga.Language, bm), nil
	case "mangapark":
		return NewMangaPark(monitoredManga.Manga, bm), nil
	case "weebcentral":
		return NewWeebCentral(monitoredManga.Manga, bm), nil
	case "comix":
		return NewComix(monitoredManga.Manga, monitoredManga.Group), nil
	}

	return nil, fmt.Errorf("unknown monitored manga source %s", monitoredManga.Source)
}
