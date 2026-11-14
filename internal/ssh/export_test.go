package ssh

// Exported for the external tests of this package, which run the embedded SSH
// server on their own listener.
var (
	NewServerConfig = newServerConfig
	Serve           = serve
)
