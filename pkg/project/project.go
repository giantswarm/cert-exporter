package project

var (
	description = "The cert-exporter walks a directory path it has gotten as input and emits all NotAfter timestamps as metrics."
	gitSHA      = "n/a"
	name        = "cert-exporter"
	source      = "https://github.com/giantswarm/cert-exporter"
	version     = "2.9.15"
)

func Description() string {
	return description
}

func GitSHA() string {
	return gitSHA
}

func Name() string {
	return name
}

func Source() string {
	return source
}

func Version() string {
	return version
}
