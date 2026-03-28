// 本文件职责：承载 runtime 与 scope 的构造、继承和 option 应用逻辑。
// 定位：负责把配置组织成作用域树，不参与请求执行与错误写回。
package runtime

type Option interface {
	apply(*scopeConfig)
}

type optionFunc func(*scopeConfig)

// apply 将 optionFunc 适配为 Option。
func (fn optionFunc) apply(cfg *scopeConfig) {
	fn(cfg)
}

// New 返回应用给定选项后的运行时实例，并默认启用请求上下文提取器。
func New(opts ...Option) *Runtime {
	rt := &Runtime{}
	rt.local.extractor = DefaultExtractor
	rt.local.hasExtractor = true
	applyOptions(&rt.local, opts)
	return rt
}

// Scope 基于当前 Runtime 创建子作用域，并在子作用域上应用给定选项；rt 为空时会 panic。
func (rt *Runtime) Scope(opts ...Option) *Runtime {
	if rt == nil {
		panic("chix: runtime must not be nil")
	}

	child := &Runtime{parent: rt}
	applyOptions(&child.local, opts)
	return child
}

// WithErrorMapper 追加错误映射器；mapper 为空时忽略。
func WithErrorMapper(mapper ErrorMapper) Option {
	return optionFunc(func(cfg *scopeConfig) {
		if mapper == nil {
			return
		}
		cfg.errorMappers = append(cfg.errorMappers, mapper)
	})
}

// WithObserver 设置观察器。
func WithObserver(observer Observer) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.observer = observer
		cfg.hasObserver = true
	})
}

// WithExtractor 设置请求上下文提取器。
func WithExtractor(extractor Extractor) Option {
	return optionFunc(func(cfg *scopeConfig) {
		cfg.extractor = extractor
		cfg.hasExtractor = true
	})
}

// WithSuccessStatus 设置成功响应的 HTTP 状态码；status 小于等于 0 时会 panic。
func WithSuccessStatus(status int) Option {
	if status <= 0 {
		panic("chix: success status must be positive")
	}

	return optionFunc(func(cfg *scopeConfig) {
		cfg.successStatus = status
		cfg.hasSuccessStatus = true
	})
}

// applyOptions 依次将非 nil 选项应用到给定作用域配置。
func applyOptions(cfg *scopeConfig, opts []Option) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(cfg)
	}
}
