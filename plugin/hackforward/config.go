package hackforward

type config struct {
	Upstreams []string `cf:"upstreams"`
}

type ConnConfig struct {
	Hostname string
	Port     int
}
