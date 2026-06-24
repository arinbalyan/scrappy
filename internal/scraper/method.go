package scraper

import "github.com/arinbalyan/scrappy/internal/model"

// Scrape method constants.
const (
	MethodHTTPAPI    = "http_api"
	MethodHTMLParse  = "html_parse"
	MethodPlaywright = "playwright"
	MethodRSS        = "rss"
	MethodGraphQL    = "graphql"
	MethodHybrid     = "hybrid"
)

// Methods maps known sites to their primary scraping method.
var Methods = map[model.Site]string{
	model.SiteLinkedIn:           MethodPlaywright,
	model.SiteIndeed:             MethodHybrid,
	model.SiteGoogle:             MethodHybrid,
	model.SiteMonster:            MethodHybrid,
	model.SiteDice:               MethodHTTPAPI,
	model.SiteAdzuna:             MethodHTTPAPI,
	model.SiteCareerjet:          MethodHTTPAPI,
	model.SiteFindwork:           MethodHTTPAPI,
	model.SiteArbeitsagentur:     MethodHTTPAPI,
	model.SiteWeb3Career:         MethodHTTPAPI,
	model.SiteJobTechDev:         MethodHTTPAPI,
	model.SiteInfoJobs:           MethodHTTPAPI,
	model.SiteAuthenticJobs:      MethodHTTPAPI,
	model.SiteTalroo:             MethodHTTPAPI,
	model.SiteJazzHR:             MethodHTTPAPI,
	model.SiteFreshteam:          MethodHTTPAPI,
	model.SiteExa:                MethodHTTPAPI,
	model.SiteUpwork:             MethodHTTPAPI,
	model.SiteDeel:               MethodHTTPAPI,
	model.SiteFountain:           MethodHTTPAPI,
	model.SiteSnagajob:           MethodHTTPAPI,
	model.SiteReed:               MethodHTTPAPI,
	model.SiteTheMuse:            MethodHTTPAPI,
	model.SiteCareerBuilder:      MethodHTTPAPI,
	model.SiteSimplyHired:        MethodHTMLParse,
	model.SiteRemoteOK:           MethodHTMLParse,
	model.SiteRemotive:           MethodHTMLParse,
	model.SiteHackerNews:         MethodHTMLParse,
	model.SiteWorkingNomads:      MethodHTMLParse,
	model.SiteJobspresso:         MethodHTMLParse,
	model.SiteHimalayas:          MethodHTMLParse,
	model.SiteHiringCafe:         MethodHTMLParse,
	model.SiteCryptocurrencyJobs: MethodHTMLParse,
	model.SiteEcoJobs:            MethodRSS,
	model.SiteEchoJobs:           MethodHTMLParse,
	model.SiteGreenJobsBoard:     MethodHTMLParse,
	model.SiteNoFluffJobs:        MethodHTMLParse,
	model.SiteGolangJobs:         MethodHTMLParse,
	model.SitePythonJobs:         MethodHTMLParse,
	model.SiteRailsJobs:          MethodHTMLParse,
	model.SiteAndroidJobs:        MethodHTMLParse,
}

// Method returns the scrape method for a site, defaulting to html_parse.
func Method(site model.Site) string {
	if m, ok := Methods[site]; ok {
		return m
	}
	return MethodHTMLParse
}
