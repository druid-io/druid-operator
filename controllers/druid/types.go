package druid

const (
	finalizerName     = "deletepvc.finalizers.druid.apache.org"
	ignoredAnnotation = "druid.apache.org/ignored"

	broker        = "broker"
	coordinator   = "coordinator"
	overlord      = "overlord"
	middleManager = "middleManager"
	indexer       = "indexer"
	historical    = "historical"
	router        = "router"
)
