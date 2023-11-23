package hackforward

type config struct {
	ConnConfig ConnConfig `cf:"connection"`
}

type ConnConfig struct {
	Hostname string `cf:"hostname"`
	Port     int    `cf:"port"`
}
