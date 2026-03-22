package chix

type config struct {
	mappers []ErrorMapper
}

// Option customizes how Wrap and WriteError map returned errors.
type Option func(*config)

func buildConfig(opts ...Option) config {
	var cfg config
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// WithErrorMapper appends a single mapper to the error mapping chain.
func WithErrorMapper(mapper ErrorMapper) Option {
	return WithErrorMappers(mapper)
}

// WithErrorMappers appends multiple mappers to the error mapping chain.
func WithErrorMappers(mappers ...ErrorMapper) Option {
	filtered := make([]ErrorMapper, 0, len(mappers))
	for _, mapper := range mappers {
		if mapper != nil {
			filtered = append(filtered, mapper)
		}
	}

	return func(cfg *config) {
		cfg.mappers = append(cfg.mappers, filtered...)
	}
}

// ChainMappers returns a mapper that evaluates mappers in order and returns the
// first non-nil mapped error.
func ChainMappers(mappers ...ErrorMapper) ErrorMapper {
	filtered := make([]ErrorMapper, 0, len(mappers))
	for _, mapper := range mappers {
		if mapper != nil {
			filtered = append(filtered, mapper)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return func(err error) *Error {
		for _, mapper := range filtered {
			if mapped := mapper(err); mapped != nil {
				return mapped
			}
		}
		return nil
	}
}
