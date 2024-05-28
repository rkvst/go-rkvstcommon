package k8sworker

type K8sOptions struct {

	// logger used for GoMaxProcs
	logger func(string, ...any)
}

type K8sOption func(*K8sOptions)

// WithLogger sets the optional logger for goMaxProcs
func WithLogger(logger func(string, ...any)) K8sOption {
	return func(ko *K8sOptions) { ko.logger = logger }
}

// ParseOptions parses the given options into a K8sOptions struct
func ParseOptions(options ...K8sOption) K8sOptions {
	k8sOptions := K8sOptions{}

	for _, option := range options {
		option(&k8sOptions)
	}

	return k8sOptions
}
