// 本文件职责：承载 runtime 与 scope 的构造、继承和 option 应用逻辑。
// 定位：负责把配置组织成作用域树，不参与请求执行与错误写回。
package runtime

type Option interface {
	apply(*scopeConfig)
}

type optionFunc func(*scopeConfig)

func (fn optionFunc) apply(cfg *scopeConfig) {
	fn(cfg)
}

func New(opts ...Option) *Runtime {
	rt := &Runtime{}
	rt.local.extractor = DefaultExtractor
	rt.local.hasExtractor = true
	applyOptions(&rt.local, opts)
	return rt
}

func (rt *Runtime) Scope(opts ...Option) *Runtime {
	if rt == nil {
		panic("chix: runtime must not be nil")
	}

	child := &Runtime{parent: rt}
	applyOptions(&child.local, opts)
	return child
}

func WithErrorMapper(mapper ErrorMapper) Option {
	return optionFunc(func(cfg *scopeConfig) {
		if mapper == nil {
			return
		}
		cfg.errorMappers = append(cfg.errorMappers, mapper)
	})
}

func WithObserver(observer Observer) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.observer = observer
		cfg.hasObserver = true
	})
}

func WithExtractor(extractor Extractor) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.extractor = extractor
		cfg.hasExtractor = true
	})
}

func WithSuccessStatus(status int) Option {
	if status <= 0 {
		panic("chix: success status must be positive")
	}

	return optionFunc(func(cfg *scopeConfig) {
		cfg.successStatus = status
		cfg.hasSuccessStatus = true
	})
}

func applyOptions(cfg *scopeConfig, opts []Option) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(cfg)
	}
}
