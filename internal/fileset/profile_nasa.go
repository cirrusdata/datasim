package fileset

// nasaProfile returns the built-in nasa fileset profile.
func nasaProfile() Profile {
	return Profile{
		Name:        "nasa",
		Description: "Mission-focused dataset with telemetry, experiments, imagery, software, and archive payloads.",
		Directories: []string{
			"Mission/Apollo/Telemetry", "Mission/Artemis/Imagery", "Science/Experiments", "Launch/Logs",
			"Ground/Software", "Archives/Payloads", "Research/Simulations", "Ops/Procedures",
		},
		Nouns:    []string{"telemetry", "payload", "guidance", "orbital", "mission", "apollo", "artemis", "experiment"},
		Prefixes: []string{"sol", "launch", "uplink", "downlink", "checklist", "review"},
		Extensions: map[string][]string{
			"archive": {".zip", ".tar.gz"},
			"code":    {".go", ".py", ".yaml"},
			"doc":     {".pdf", ".txt"},
			"img":     {".jpg", ".png"},
			"log":     {".log", ".dat"},
			"sheet":   {".csv"},
			"vid":     {".mp4"},
		},
	}
}
