package audio

import (
	"MusicConvert/api"
	"MusicConvert/config"
	"log"
	"strconv"
)

type OnlineMetadata struct {
	Artist     string
	Album      string
	Title      string
	Year       string
	TrackNum   int
	TrackTotal int
	CoverURL   string
	Source     string
}

type MetadataProvider interface {
	Name() string
	Fetch(title, artist, albumHint string) (OnlineMetadata, bool)
}

type ProviderChain struct {
	providers []MetadataProvider
}

func NewProviderChain() *ProviderChain {
	chain := &ProviderChain{}
	if config.IsSpotifyEnabled() {
		chain.Add(&SpotifyProvider{})
	}
	chain.Add(&MusicBrainzProvider{})
	chain.Add(&NeteaseProvider{})
	return chain
}

func (c *ProviderChain) Add(p MetadataProvider) {
	c.providers = append(c.providers, p)
}

func (c *ProviderChain) Fetch(title, artist, albumHint string) (OnlineMetadata, bool) {
	for _, p := range c.providers {
		meta, ok := p.Fetch(title, artist, albumHint)
		if ok {
			log.Printf("[Metadata Winner] Source: %s. Data: %s - %s / %s", p.Name(), meta.Artist, meta.Title, meta.Album)
			return meta, true
		}
	}
	return OnlineMetadata{}, false
}

// ---------------------------------------------------------
// Spotify Provider
// ---------------------------------------------------------
type SpotifyProvider struct{}

func (p *SpotifyProvider) Name() string { return "Spotify" }

func (p *SpotifyProvider) Fetch(title, artist, albumHint string) (OnlineMetadata, bool) {
	sArtist, sAlbum, sTitle, sCover, sYear, sTrackNum := api.FetchSpotifyMeta(title, artist, albumHint)
	if sArtist != "" {
		trackNum := sTrackNum
		return OnlineMetadata{
			Artist:   sArtist,
			Album:    sAlbum,
			Title:    sTitle,
			Year:     sYear,
			TrackNum: trackNum,
			CoverURL: sCover,
			Source:   p.Name(),
		}, true
	}
	return OnlineMetadata{}, false
}

// ---------------------------------------------------------
// MusicBrainz Provider
// ---------------------------------------------------------
type MusicBrainzProvider struct{}

func (p *MusicBrainzProvider) Name() string { return "MusicBrainz" }

func (p *MusicBrainzProvider) Fetch(title, artist, albumHint string) (OnlineMetadata, bool) {
	if api.ContainsChinese(title) || api.ContainsChinese(artist) || api.ContainsChinese(albumHint) {
		log.Printf("[MusicBrainz Skip] Chinese metadata detected; skipping MusicBrainz.")
		return OnlineMetadata{}, false
	}

	mArtist, mAlbum, mTitle, mYear, mTrackNumStr, mTrackTotal := api.FetchMBMetadata(title, artist, albumHint)
	if mArtist != "" {
		// Like before, MB needs Netease to translate/fetch cover
		_, _, _, coverUrl, _ := api.GetTranslatedArtistAlbum(mArtist, mAlbum)

		trackNum, _ := strconv.Atoi(mTrackNumStr)
		return OnlineMetadata{
			Artist:     mArtist,
			Album:      mAlbum,
			Title:      mTitle,
			Year:       mYear,
			TrackNum:   trackNum,
			TrackTotal: mTrackTotal,
			CoverURL:   coverUrl,
			Source:     p.Name(),
		}, true
	}
	return OnlineMetadata{}, false
}

// ---------------------------------------------------------
// Netease Provider
// ---------------------------------------------------------
type NeteaseProvider struct{}

func (p *NeteaseProvider) Name() string { return "Netease" }

func (p *NeteaseProvider) Fetch(title, artist, albumHint string) (OnlineMetadata, bool) {
	nArtist, nAlbum, nTitle, nTrackNumStr, nTrackCount := api.IdentifyFullContext(title, artist, albumHint)
	if nArtist != "" || nAlbum != "" || nTitle != "" {
		_, _, _, coverUrl, neteaseYear := api.GetTranslatedArtistAlbum(nArtist, nAlbum)
		trackNum := nTrackNumStr
		return OnlineMetadata{
			Artist:     nArtist,
			Album:      nAlbum,
			Title:      nTitle,
			Year:       neteaseYear,
			TrackNum:   trackNum,
			TrackTotal: nTrackCount,
			CoverURL:   coverUrl,
			Source:     p.Name(),
		}, true
	}
	return OnlineMetadata{}, false
}
