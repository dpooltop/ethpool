package main

type Config struct {
	Threads int			`json:"threads"`
	BlockRefreshInterval string	`json:"blockRefreshInterval"`
	Upstream []Upstream		`json:"upstream"`
	UpstreamCheckInterval string	`json:"upstreamCheckInterval"`
}

type Upstream struct {
	Name string			`json:"name"`
	Url string			`json:"url"`
	Timeout string			`json:"timeout"`
}
