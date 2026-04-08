package reqx

const defaultMaxBodyBytes int64 = 1 << 20

type bindBodyConfig struct {
	maxBodyBytes       int64
	allowUnknownFields bool
}

type bindValuesConfig struct {
	allowUnknownFields bool
}

type bindConfig struct {
	body   bindBodyConfig
	query  bindValuesConfig
	header bindValuesConfig
}

func defaultBindConfig() bindConfig {
	return bindConfig{
		body: bindBodyConfig{
			maxBodyBytes:       defaultMaxBodyBytes,
			allowUnknownFields: true,
		},
		query: bindValuesConfig{
			allowUnknownFields: true,
		},
		header: bindValuesConfig{
			allowUnknownFields: true,
		},
	}
}
