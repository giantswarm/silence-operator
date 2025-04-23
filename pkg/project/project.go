package project

var (
	description = "The silence-operator does something."
	gitSHA      = "n/a"
	name        = "silence-operator"
	source      = "https://github.com/giantswarm/silence-operator"
	version     = "0.14.1"
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
