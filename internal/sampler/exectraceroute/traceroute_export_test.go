package exectraceroute

// export for testing.
func WithTracer(tracer Tracer) Option {
	return func(c *TraceRoute) error {
		c.tracer = tracer
		return nil
	}
}
