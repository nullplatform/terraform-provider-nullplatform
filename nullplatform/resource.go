package nullplatform

type Resource struct {
	AWSS3AssestBucket               string `json:"aws.s3_assets_bucket"`
	AWSScopeWorkflowRole            string `json:"aws.scope_workflow_role"`
	AWSLogGroupName                 string `json:"aws.log_group_name"`
	AWSLambdaFunctionName           string `json:"aws.lambdaFunctionName"`
	AWSLambdaCurrentFunctionVersion string `json:"aws.lambdaCurrentFunctionVersion"`
	AWSLambdaFunctionRole           string `json:"aws.lambdaFunctionRole"`
	AWSLambdaFunctionMainAlias      string `json:"aws.lambdaFunctionMainAlias"`
	AWSLogReaderLog                 string `json:"aws.log_reader_role"`
	AWSLambdaFunctionWarmAlias      string `json:"aws.lambdaFunctionWarmAlias"`
}
