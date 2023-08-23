package nullplatform

type Capability struct {
	Visibility                 map[string]string `json:"visibility"`
	ServerlessRuntime          map[string]string `json:"serverless_runtime"`
	ServerlessHandler          map[string]string `json:"serverless_handler"`
	ServerlessTimeout          map[string]int    `json:"serverless_timeout"`
	ServerlessEphemeralStorage map[string]int    `json:"serverless_ephemeral_storage"`
	ServerlessMemory           map[string]int    `json:"serverless_memory"`
}

type RequestSpec struct {
	MemoryInGb   float32 `json:"memory_in_gb"`
	CpuProfile   string  `json:"cpu_profile"`
	LocalStorage int     `json:"local_storage"`
}

type Scope struct {
	Id               int         `json:"id"`
	Status           string      `json:"status"`
	Slug             string      `json:"slug"`
	Domain           string      `json:"domain"`
	ActiveDeployment int         `json:"active_deployment"`
	Nrn              string      `json:"nrn"`
	Name             string      `json:"name"`
	ApplicationId    int         `json:"application_id"`
	Type             string      `json:"type"`
	ExternalCreated  bool        `json:"external_created"`
	RequestedSpec    RequestSpec `json:"requested_spec"`
	Capabilities     Capability  `json:"capabilities"`
}
