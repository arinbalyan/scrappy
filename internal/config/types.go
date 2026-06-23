package config

type Site struct {
	ID   string `toml:"id"`
	Type string `toml:"type"` // html | api | graphql | rss

	Search     string `toml:"search"`
	Location   string `toml:"location"`
	Results    int    `toml:"results"`
	IsRemote   *bool  `toml:"is_remote,omitempty"`
	JobType    string `toml:"job_type,omitempty"`
	HoursOld   *int   `toml:"hours_old,omitempty"`
	Country    string `toml:"country,omitempty"`

	// Indeed
	IndeedAPIKey string `toml:"api_key,omitempty"`

	// LinkedIn
	FetchDescription *bool `toml:"fetch_description,omitempty"`

	// HTML mechanism
	URL             string         `toml:"url,omitempty"`
	ItemSelector    string         `toml:"item_selector,omitempty"`
	TitleSelector   string         `toml:"title_selector,omitempty"`
	CompanySelector string         `toml:"company_selector,omitempty"`
	LocSelector     string         `toml:"loc_selector,omitempty"`
	DescSelector    string         `toml:"desc_selector,omitempty"`
	DateSelector    string         `toml:"date_selector,omitempty"`

	// API mechanism
	Endpoint    string            `toml:"endpoint,omitempty"`
	SearchParam string            `toml:"search_param,omitempty"`
	Headers     map[string]string `toml:"headers,omitempty"`

	// GraphQL mechanism
	Query string `toml:"query,omitempty"`
}

type Defaults struct {
	Concurrency int    `toml:"concurrency"`
	DelayMs     int    `toml:"delay_ms"`
	MaxResults  int    `toml:"max_results"`
	Output      string `toml:"output"` // csv, postgres, both
}

type CSVOut struct{ Path string `toml:"path"` }
type PgOut struct{ URL string `toml:"url"` }
type Output struct {
	CSV      CSVOut `toml:"csv"`
	Postgres PgOut  `toml:"postgres"`
}

type RunConfig struct {
	Defaults Defaults `toml:"defaults"`
	Output   Output   `toml:"output"`
	Sites    []Site   `toml:"sites"`
}
