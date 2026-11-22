package wintunnel

func reportProgress(report func(string), stage string) {
	if report != nil {
		report(stage)
	}
}
